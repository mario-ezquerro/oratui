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
}

