package telemetry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mario/ora-tui/internal/collector"
	"github.com/mario/ora-tui/internal/model"
)

func TestRenderPrometheusAndOTelMetrics(t *testing.T) {
	st := *collector.NewMockState(model.ModeRAC)
	now := time.Unix(1729000000, 0)

	promText := RenderPrometheusMetrics(st, now, "1.5.0")

	requiredMetrics := []string{
		`oratui_info{version="1.5.0",host="ora-rac01.corp.local",cluster="crs-prod-cluster"`,
		`oratui_cluster_node_up{`,
		`oratui_cluster_daemon_up{`,
		`oratui_instance_up{`,
		`oratui_instance_fra_used_percent{`,
		`oratui_pdb_up{`,
		`oratui_service_up{`,
		`oratui_sessions_total{`,
		`oratui_sessions_blocking_locks_total{`,
		`oratui_tablespace_used_percent{`,
		`oratui_asm_diskgroup_used_percent{`,
		`oratui_listener_up{`,
		`oratui_rman_backup_status{`,
		`oratui_dataguard_mrp_up{`,
		`oratui_alert_ora_errors_total{`,
		`oratui_alert_ora_error_count{`,
		`oratui_os_metric_used_percent{`,
		`oratui_os_metric_healthy{`,
		`oratui_filesystem_used_percent{`,
		`oratui_filesystem_inodes_used_percent{`,
		`oratui_storage_path_healthy{`,
		`oratui_sql_plan_instability_elapsed_ms{`,
		`oratui_rman_channel_progress_percent{`,
		`oratui_scheduler_job_failed_runs_24h{`,
		`oratui_rac_gc_cr_block_receive_ms{`,
		`oratui_datafile_io_avg_read_ms{`,
		`oratui_redo_switches_last_hour{`,
		`oratui_buffer_cache_hit_percent{`,
		`oratui_wait_chains_active_total{`,
		`oratui_parallel_query_downgraded{`,
		`oratui_advisor_findings_total{`,
	}

	for _, req := range requiredMetrics {
		if !strings.Contains(promText, req) {
			t.Fatalf("expected Prometheus metrics output to contain %q, got:\n%s", req, promText)
		}
	}

	exp := NewExporter(st, ":9161", "1.5.0")

	// Verify /otel/v1/metrics endpoint
	req := httptest.NewRequest(http.MethodGet, "/otel/v1/metrics", nil)
	rec := httptest.NewRecorder()
	exp.handleOTelJSONMetrics(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /otel/v1/metrics, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"resourceMetrics"`) || !strings.Contains(rec.Body.String(), `"oratui.storage.path.healthy"`) {
		t.Fatalf("expected valid OTLP JSON resourceMetrics payload, got: %s", rec.Body.String())
	}
}

func TestSRESLOAndRemoteActions(t *testing.T) {
	st := *collector.NewMockState(model.ModeRAC)
	exp := NewExporter(st, ":9161", "1.7.0")

	// 1. Test /api/v1/sre
	sreReq := httptest.NewRequest(http.MethodGet, "/api/v1/sre", nil)
	sreRec := httptest.NewRecorder()
	exp.handleSREGetOrPost(sreRec, sreReq)
	if sreRec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /api/v1/sre, got %d", sreRec.Code)
	}
	body := sreRec.Body.String()
	for _, field := range []string{`"slos"`, `"active_alerts"`, `"history"`, `"availability_target_pct"`, `"slo_availability"`} {
		if !strings.Contains(body, field) {
			t.Fatalf("expected /api/v1/sre response to contain %q, got:\n%s", field, body)
		}
	}

	// 2. Test /api/v1/action (mock execution)
	actReq := httptest.NewRequest(http.MethodPost, "/api/v1/action", strings.NewReader(`{"action":"kill_session","target":"512,9931,@2"}`))
	actRec := httptest.NewRecorder()
	exp.handleExecuteAction(actRec, actReq)
	if actRec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /api/v1/action, got %d", actRec.Code)
	}
	if !strings.Contains(actRec.Body.String(), `"System altered."`) {
		t.Fatalf("expected /api/v1/action response to contain 'System altered.', got:\n%s", actRec.Body.String())
	}

	// 3. Test anti-injection validation on /api/v1/action
	injReq := httptest.NewRequest(http.MethodPost, "/api/v1/action", strings.NewReader(`{"action":"add_datafile","target":"USERS; DROP DATABASE"}`))
	injRec := httptest.NewRecorder()
	exp.handleExecuteAction(injRec, injReq)
	if injRec.Code != http.StatusBadRequest {
		t.Fatalf("expected HTTP 400 for SQL injection attempt on /api/v1/action, got %d", injRec.Code)
	}

	// 4. Test Basic Auth / Token enforcement when SetSecurityConfig is enabled
	exp.SetSecurityConfig("super-secret-token", "", "")
	unauthReq := httptest.NewRequest(http.MethodPost, "/api/v1/action", strings.NewReader(`{"action":"switch_logfile"}`))
	unauthRec := httptest.NewRecorder()
	exp.handleExecuteAction(unauthRec, unauthReq)
	if unauthRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected HTTP 401 for unauthenticated request when token is required, got %d", unauthRec.Code)
	}

	// Valid Basic Auth (admin:admin default)
	basicReq := httptest.NewRequest(http.MethodPost, "/api/v1/action", strings.NewReader(`{"action":"switch_logfile"}`))
	basicReq.SetBasicAuth("admin", "admin")
	basicRec := httptest.NewRecorder()
	exp.handleExecuteAction(basicRec, basicReq)
	if basicRec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 with valid Basic Auth (admin:admin), got %d", basicRec.Code)
	}

	// 5. Test new ora-brabo actions (pin_sql_plan, run_scheduler_job, tune_memory_advisor)
	for _, tc := range []struct {
		payload string
		expect  string
	}{
		{`{"action":"pin_sql_plan","target":"8f4k2m91x7p0q:1049281744"}`, "SPM Plans Loaded"},
		{`{"action":"run_scheduler_job","target":"CRM_DATA.JOB_SYNC_CREDIT_SCORES"}`, "Coordinator CJQ0"},
		{`{"action":"tune_memory_advisor","target":"SGA:40960"}`, "SGA_SIZE_FACTOR"},
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/action", strings.NewReader(tc.payload))
		req.SetBasicAuth("admin", "admin")
		rec := httptest.NewRecorder()
		exp.handleExecuteAction(rec, req)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), tc.expect) {
			t.Fatalf("expected HTTP 200 containing %q for %s, got %d: %s", tc.expect, tc.payload, rec.Code, rec.Body.String())
		}
	}

	// 6. Test Executive HTML/PDF Report (/api/v1/report)
	repReq := httptest.NewRequest(http.MethodGet, "/api/v1/report", nil)
	repRec := httptest.NewRecorder()
	exp.handleExecutiveReport(repRec, repReq)
	if repRec.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200 from /api/v1/report, got %d", repRec.Code)
	}
	repHTML := repRec.Body.String()
	for _, snippet := range []string{"INFORME EJECUTIVO DE SALUD ORACLE DBA", "8f4k2m91x7p0q", "JOB_SYNC_CREDIT_SCORES", "PRODCDB1"} {
		if !strings.Contains(repHTML, snippet) {
			t.Fatalf("expected /api/v1/report HTML to contain %q", snippet)
		}
	}
}

