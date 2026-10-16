package telemetry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mario/ora-tui/internal/model"
)

// DefaultOTelPort is 9161 (standard Prometheus/OpenTelemetry port for Oracle DB monitoring,
// completely outside Oracle's port ranges 1521-1529, 5500, 8080, 4903).
const DefaultOTelPort = 9161

// Exporter serves thread-safe OpenTelemetry (OTLP JSON), Prometheus/OpenMetrics (/metrics),
// SRE/SLO & Telegram/Email Alerting (/api/v1/sre), and Remote Remediation Actions (/api/v1/action).
type Exporter struct {
	mu                  sync.RWMutex
	state               model.EnvironmentState
	lastUpdated         time.Time
	listenAddr          string
	version             string
	history             []MetricHistoryPoint
	sloConfig           SLOConfig
	totalSamples        int
	goodAvailSamples    int
	goodLockFreeSamples int
	goodStorageSamples  int
	goodSANSamples      int
	activeAlerts        []DelicateAlert
	notifications       []NotificationEvent
	alertLastSent       map[string]time.Time
}

func NewExporter(initial model.EnvironmentState, listenAddr, version string) *Exporter {
	if listenAddr == "" {
		listenAddr = fmt.Sprintf(":%d", DefaultOTelPort)
	} else if !strings.Contains(listenAddr, ":") {
		listenAddr = ":" + listenAddr
	}
	now := time.Now()
	exp := &Exporter{
		state:         initial,
		lastUpdated:   now,
		listenAddr:    listenAddr,
		version:       version,
		sloConfig:     defaultSLOConfig(),
		alertLastSent: make(map[string]time.Time),
	}
	exp.recordStateAndEvaluateSRE(initial, now)
	return exp
}

func (e *Exporter) ListenAddr() string {
	return e.listenAddr
}

func (e *Exporter) UpdateState(s model.EnvironmentState) {
	now := time.Now()
	e.mu.Lock()
	e.state = s
	e.lastUpdated = now
	e.recordStateAndEvaluateSRE(s, now)
	e.mu.Unlock()
}

func (e *Exporter) Snapshot() (model.EnvironmentState, time.Time) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state, e.lastUpdated
}

// Start launches the HTTP server in a background goroutine.
func (e *Exporter) Start() (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", e.handlePrometheusMetrics)
	mux.HandleFunc("/otel/v1/metrics", e.handleOTelJSONMetrics)
	mux.HandleFunc("/api/v1/state", e.handleStateJSON)
	mux.HandleFunc("/api/v1/sre", e.handleSREGetOrPost)
	mux.HandleFunc("/api/v1/sre/config", e.handleSREConfigUpdate)
	mux.HandleFunc("/api/v1/sre/notify", e.handleSRENotifyNow)
	mux.HandleFunc("/api/v1/action", e.handleExecuteAction)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if enableCORS(w, r) {
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK\n"))
	})
	mux.HandleFunc("/", e.handleIndexPage)

	srv := &http.Server{
		Addr:              e.listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		_ = srv.ListenAndServe()
	}()
	return srv, nil
}

func (e *Exporter) handleIndexPage(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	st, updated := e.Snapshot()
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>ORA-TUI OpenTelemetry, SRE &amp; Control API</title>
  <style>
    body { background: #0000af; color: #ffffff; font-family: monospace; padding: 24px; }
    .box { border: 3px double #5fffff; padding: 20px; max-width: 900px; background: #000080; }
    h1 { color: #ffff5f; margin-top: 0; }
    a { color: #5fffff; font-weight: bold; }
    .badge { background: #00afaf; color: #000; padding: 2px 8px; font-weight: bold; }
    li { margin: 10px 0; }
  </style>
</head>
<body>
  <div class="box">
    <h1>══ ORA-TUI %s :: OpenTelemetry, SRE/SLO &amp; Remote Actions API ══</h1>
    <p><span class="badge">%s</span> <b>Host:</b> %s │ <b>Cluster:</b> %s │ <b>Oracle Ver:</b> %s │ <b>Last Scrape:</b> %s</p>
    <hr style="border-color:#5fffff">
    <ul>
      <li><a href="/metrics">/metrics</a> — Formato <b>Prometheus / OpenMetrics</b> (52 métricas)</li>
      <li><a href="/otel/v1/metrics">/otel/v1/metrics</a> — Formato <b>OpenTelemetry OTLP JSON (resourceMetrics)</b></li>
      <li><a href="/api/v1/state">/api/v1/state</a> — Snapshot completo del estado + historial temporal + SRE/SLO en <b>JSON</b> (para Flutter Web)</li>
      <li><a href="/api/v1/sre">/api/v1/sre</a> — Estado de <b>Objetivos SLO, Error Budget, Alertas Críticas y Canales Telegram/Email</b></li>
      <li><b>POST /api/v1/action</b> — Ejecución remota de acciones DBA y Linux/SAN con salida completa en tiempo real</li>
      <li><a href="/healthz">/healthz</a> — Healthcheck HTTP 200 OK</li>
    </ul>
  </div>
</body>
</html>`, e.version, st.Topology, st.HostName, st.ClusterName, st.ReleaseVersion, updated.Format("2006-01-02 15:04:05"))
}

func (e *Exporter) handleStateJSON(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	e.mu.RLock()
	st := e.state
	updated := e.lastUpdated
	cfg := e.sloConfig
	slos := computeSLOs(e.totalSamples, e.goodAvailSamples, e.goodLockFreeSamples, e.goodStorageSamples, e.goodSANSamples, cfg)
	alerts := append([]DelicateAlert(nil), e.activeAlerts...)
	hist := append([]MetricHistoryPoint(nil), e.history...)
	notifs := append([]NotificationEvent(nil), e.notifications...)
	e.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	payload := map[string]interface{}{
		"version":       e.version,
		"last_updated":  updated.Format(time.RFC3339),
		"state":         st,
		"slos":          slos,
		"active_alerts": alerts,
		"history":       hist,
		"sre_config":    cfg,
		"notifications": notifs,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func (e *Exporter) handlePrometheusMetrics(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	st, updated := e.Snapshot()
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	_, _ = w.Write([]byte(RenderPrometheusMetrics(st, updated, e.version)))
}

// RenderPrometheusMetrics renders 100% of EnvironmentState metrics in Prometheus / OpenMetrics text format.
func RenderPrometheusMetrics(st model.EnvironmentState, updated time.Time, version string) string {
	var b strings.Builder
	host := st.HostName
	if host == "" {
		host = "localhost"
	}
	cluster := st.ClusterName
	if cluster == "" {
		cluster = "single-instance"
	}
	topo := string(st.Topology)

	writeHelpType := func(name, help, mtype string) {
		b.WriteString(fmt.Sprintf("# HELP %s %s\n# TYPE %s %s\n", name, help, name, mtype))
	}

	// 1. Metadata & Exporter Health
	writeHelpType("oratui_info", "Static metadata about the monitored Oracle host and cluster", "gauge")
	b.WriteString(fmt.Sprintf("oratui_info{version=%q,host=%q,cluster=%q,topology=%q,oracle_version=%q,oracle_home=%q,grid_home=%q} 1\n",
		version, host, cluster, topo, st.ReleaseVersion, st.OracleHome, st.GridHome))

	writeHelpType("oratui_last_scrape_timestamp_seconds", "Unix timestamp of the last collector refresh", "gauge")
	b.WriteString(fmt.Sprintf("oratui_last_scrape_timestamp_seconds{host=%q,cluster=%q} %d\n",
		host, cluster, updated.Unix()))

	// 2. Cluster Nodes & Daemons (CRS, CSSD, EVMD, ASM)
	writeHelpType("oratui_cluster_node_up", "Oracle Clusterware node status (1=ONLINE, 0=OFFLINE)", "gauge")
	writeHelpType("oratui_cluster_daemon_up", "Oracle Clusterware daemon status (1=ONLINE, 0=OFFLINE)", "gauge")
	for _, n := range st.Nodes {
		nodeUp := boolToFloat(isUpStatus(n.State))
		b.WriteString(fmt.Sprintf("oratui_cluster_node_up{host=%q,cluster=%q,node=%q,role=%q,vip=%q,interconnect=%q} %.0f\n",
			host, cluster, n.Name, n.Role, n.VIP, n.Interconnect, nodeUp))

		daemons := map[string]string{
			"crs":  n.CRS,
			"cssd": n.CSSD,
			"evmd": n.EVMD,
			"asm":  n.ASM,
		}
		for _, dName := range []string{"crs", "cssd", "evmd", "asm"} {
			dVal := daemons[dName]
			b.WriteString(fmt.Sprintf("oratui_cluster_daemon_up{host=%q,cluster=%q,node=%q,daemon=%q,raw_state=%q} %.0f\n",
				host, cluster, n.Name, dName, dVal, boolToFloat(isUpStatus(dVal))))
		}
	}

	// 3. Oracle Database Instances & FRA Usage
	writeHelpType("oratui_instance_up", "Oracle Database instance status (1=OPEN, 0.5=MOUNTED, 0=DOWN)", "gauge")
	writeHelpType("oratui_instance_fra_used_percent", "Fast Recovery Area (FRA) usage percentage", "gauge")
	for _, inst := range st.Instances {
		val := 0.0
		upSt := strings.ToUpper(inst.Status)
		upMode := strings.ToUpper(inst.OpenMode)
		if strings.Contains(upSt, "OPEN") || strings.Contains(upMode, "READ") {
			val = 1.0
		} else if strings.Contains(upSt, "MOUNT") || strings.Contains(upMode, "MOUNT") {
			val = 0.5
		}
		b.WriteString(fmt.Sprintf("oratui_instance_up{host=%q,cluster=%q,instance=%q,db_name=%q,node=%q,status=%q,open_mode=%q,role=%q} %g\n",
			host, cluster, inst.InstName, inst.DBName, inst.NodeName, inst.Status, inst.OpenMode, inst.Role, val))
		b.WriteString(fmt.Sprintf("oratui_instance_fra_used_percent{host=%q,cluster=%q,instance=%q,db_name=%q,node=%q} %d\n",
			host, cluster, inst.InstName, inst.DBName, inst.NodeName, inst.FRAUsedPct))
	}

	// 4. Pluggable Databases (PDBs)
	writeHelpType("oratui_pdb_up", "Oracle Pluggable Database status (1=READ WRITE/READ ONLY, 0=MOUNTED)", "gauge")
	writeHelpType("oratui_pdb_size_gb", "Oracle Pluggable Database total size in GB", "gauge")
	for _, pdb := range st.PDBs {
		pdbUp := boolToFloat(strings.Contains(strings.ToUpper(pdb.OpenMode), "READ"))
		b.WriteString(fmt.Sprintf("oratui_pdb_up{host=%q,cluster=%q,pdb=%q,con_id=%d,open_mode=%q,restricted=%q} %.0f\n",
			host, cluster, pdb.Name, pdb.ConID, pdb.OpenMode, pdb.Restricted, pdbUp))
		b.WriteString(fmt.Sprintf("oratui_pdb_size_gb{host=%q,cluster=%q,pdb=%q,con_id=%d} %.2f\n",
			host, cluster, pdb.Name, pdb.ConID, pdb.SizeGB))
	}

	// 5. Cluster / Database Services
	writeHelpType("oratui_service_up", "Oracle Database service status (1=ONLINE/RUNNING, 0=OFFLINE)", "gauge")
	for _, svc := range st.Services {
		b.WriteString(fmt.Sprintf("oratui_service_up{host=%q,cluster=%q,service=%q,pdb=%q,preferred=%q,available=%q,active_inst=%q} %.0f\n",
			host, cluster, svc.Name, svc.PDB, svc.Preferred, svc.Available, svc.ActiveInst, boolToFloat(isUpStatus(svc.State))))
	}

	// 6. Sessions & Blocking Locks (gv$session)
	activeSess := 0
	inactiveSess := 0
	blockingLocks := 0
	writeHelpType("oratui_session_pga_mb", "PGA memory in MB used by active/notable Oracle session", "gauge")
	for _, s := range st.Sessions {
		if strings.ToUpper(s.Status) == "ACTIVE" {
			activeSess++
		} else {
			inactiveSess++
		}
		if strings.Contains(strings.ToUpper(s.BlockingSession), "BLOCK") || strings.Contains(strings.ToUpper(s.WaitEvent), "LOCK") {
			blockingLocks++
		}
		b.WriteString(fmt.Sprintf("oratui_session_pga_mb{host=%q,cluster=%q,inst_id=%q,sid=%q,serial=%q,username=%q,status=%q,sql_id=%q,event=%q,blocking=%q} %.2f\n",
			host, cluster, strconv.Itoa(s.InstID), strconv.Itoa(s.SID), strconv.Itoa(s.Serial),
			s.Username, s.Status, s.SQLID, s.WaitEvent, s.BlockingSession, s.PGAMB))
	}
	writeHelpType("oratui_sessions_total", "Total number of monitored user sessions", "gauge")
	b.WriteString(fmt.Sprintf("oratui_sessions_total{host=%q,cluster=%q} %d\n", host, cluster, len(st.Sessions)))
	writeHelpType("oratui_sessions_active_total", "Number of ACTIVE user sessions", "gauge")
	b.WriteString(fmt.Sprintf("oratui_sessions_active_total{host=%q,cluster=%q} %d\n", host, cluster, activeSess))
	writeHelpType("oratui_sessions_inactive_total", "Number of INACTIVE user sessions", "gauge")
	b.WriteString(fmt.Sprintf("oratui_sessions_inactive_total{host=%q,cluster=%q} %d\n", host, cluster, inactiveSess))
	writeHelpType("oratui_sessions_blocking_locks_total", "Number of sessions involved in TX/TM blocking locks", "gauge")
	b.WriteString(fmt.Sprintf("oratui_sessions_blocking_locks_total{host=%q,cluster=%q} %d\n", host, cluster, blockingLocks))

	// 7. Tablespaces & Datafiles
	writeHelpType("oratui_tablespace_alloc_mb", "Currently allocated size on disk for tablespace in MB", "gauge")
	writeHelpType("oratui_tablespace_max_mb", "Maximum expandable capacity (AUTOEXTEND MAXBYTES) for tablespace in MB", "gauge")
	writeHelpType("oratui_tablespace_used_mb", "Used space in tablespace in MB", "gauge")
	writeHelpType("oratui_tablespace_free_mb", "Real free space (including AUTOEXTEND headroom) in tablespace in MB", "gauge")
	writeHelpType("oratui_tablespace_used_percent", "Real used percentage of tablespace considering AUTOEXTEND MAXBYTES", "gauge")
	writeHelpType("oratui_tablespace_datafiles_count", "Number of datafiles in tablespace", "gauge")
	for _, ts := range st.Tablespaces {
		labels := fmt.Sprintf("host=%q,cluster=%q,tablespace=%q,type=%q,status=%q,autoextend=%q",
			host, cluster, ts.Name, ts.Type, ts.Status, ts.AutoExtend)
		b.WriteString(fmt.Sprintf("oratui_tablespace_alloc_mb{%s} %d\n", labels, ts.AllocMB))
		b.WriteString(fmt.Sprintf("oratui_tablespace_max_mb{%s} %d\n", labels, ts.TotalMB))
		b.WriteString(fmt.Sprintf("oratui_tablespace_used_mb{%s} %d\n", labels, ts.UsedMB))
		b.WriteString(fmt.Sprintf("oratui_tablespace_free_mb{%s} %d\n", labels, ts.FreeMB))
		b.WriteString(fmt.Sprintf("oratui_tablespace_used_percent{%s} %.2f\n", labels, ts.UsedPct))
		b.WriteString(fmt.Sprintf("oratui_tablespace_datafiles_count{%s} %d\n", labels, ts.Datafiles))
	}

	writeHelpType("oratui_datafile_size_mb", "Datafile current size in MB", "gauge")
	writeHelpType("oratui_datafile_max_mb", "Datafile maximum autoextend size in MB", "gauge")
	for _, df := range st.Datafiles {
		labels := fmt.Sprintf("host=%q,cluster=%q,file_id=%q,tablespace=%q,file_name=%q,autoextend=%q,status=%q",
			host, cluster, strconv.Itoa(df.FileID), df.Tablespace, df.FileName, df.AutoExtend, df.Status)
		b.WriteString(fmt.Sprintf("oratui_datafile_size_mb{%s} %d\n", labels, df.SizeMB))
		b.WriteString(fmt.Sprintf("oratui_datafile_max_mb{%s} %d\n", labels, df.MaxSizeMB))
	}

	// 8. Table Segments & Invalid Objects
	writeHelpType("oratui_segment_size_mb", "Top table or index segment size in MB", "gauge")
	writeHelpType("oratui_segment_rows", "Estimated number of rows in table segment", "gauge")
	writeHelpType("oratui_segment_stale_stats", "Whether segment optimizer statistics are STALE (1=STALE, 0=FRESH)", "gauge")
	for _, seg := range st.TableSegments {
		labels := fmt.Sprintf("host=%q,cluster=%q,owner=%q,segment=%q,type=%q,tablespace=%q,stats_status=%q,fragmented=%q",
			host, cluster, seg.Owner, seg.SegmentName, seg.SegmentType, seg.Tablespace, seg.StatsStatus, seg.Fragmented)
		b.WriteString(fmt.Sprintf("oratui_segment_size_mb{%s} %d\n", labels, seg.SizeMB))
		b.WriteString(fmt.Sprintf("oratui_segment_rows{%s} %d\n", labels, seg.NumRows))
		stale := boolToFloat(strings.Contains(strings.ToUpper(seg.StatsStatus), "STALE"))
		b.WriteString(fmt.Sprintf("oratui_segment_stale_stats{host=%q,cluster=%q,owner=%q,segment=%q,tablespace=%q} %.0f\n",
			host, cluster, seg.Owner, seg.SegmentName, seg.Tablespace, stale))
	}

	writeHelpType("oratui_invalid_objects_total", "Total number of INVALID database objects in dba_objects", "gauge")
	b.WriteString(fmt.Sprintf("oratui_invalid_objects_total{host=%q,cluster=%q} %d\n", host, cluster, len(st.InvalidObjects)))
	writeHelpType("oratui_invalid_object_info", "Presence of an INVALID database object (1=INVALID)", "gauge")
	for _, inv := range st.InvalidObjects {
		b.WriteString(fmt.Sprintf("oratui_invalid_object_info{host=%q,cluster=%q,owner=%q,object_name=%q,object_type=%q,last_ddl=%q} 1\n",
			host, cluster, inv.Owner, inv.ObjectName, inv.ObjectType, inv.LastDDL))
	}

	// 9. ASM Diskgroups & Oracle Net Listeners
	writeHelpType("oratui_asm_diskgroup_total_mb", "ASM diskgroup total capacity in MB", "gauge")
	writeHelpType("oratui_asm_diskgroup_free_mb", "ASM diskgroup raw free space in MB", "gauge")
	writeHelpType("oratui_asm_diskgroup_usable_mb", "ASM diskgroup usable file space in MB considering redundancy", "gauge")
	writeHelpType("oratui_asm_diskgroup_used_percent", "ASM diskgroup used space percentage", "gauge")
	writeHelpType("oratui_asm_diskgroup_offline_disks", "Number of offline disks in ASM diskgroup", "gauge")
	for _, dg := range st.Diskgroups {
		labels := fmt.Sprintf("host=%q,cluster=%q,diskgroup=%q,state=%q,redundancy=%q",
			host, cluster, dg.Name, dg.State, dg.Type)
		b.WriteString(fmt.Sprintf("oratui_asm_diskgroup_total_mb{%s} %d\n", labels, dg.TotalMB))
		b.WriteString(fmt.Sprintf("oratui_asm_diskgroup_free_mb{%s} %d\n", labels, dg.FreeMB))
		b.WriteString(fmt.Sprintf("oratui_asm_diskgroup_usable_mb{%s} %d\n", labels, dg.UsableMB))
		b.WriteString(fmt.Sprintf("oratui_asm_diskgroup_used_percent{%s} %.2f\n", labels, dg.UsedPct))
		b.WriteString(fmt.Sprintf("oratui_asm_diskgroup_offline_disks{%s} %d\n", labels, dg.OfflineDisks))
	}

	writeHelpType("oratui_listener_up", "Oracle Net Listener status (1=ONLINE, 0=OFFLINE)", "gauge")
	writeHelpType("oratui_listener_services_ready", "Number of dynamically registered READY services in listener", "gauge")
	writeHelpType("oratui_listener_service_status", "Service handler state in listener (1=READY, 0.5=UNKNOWN, 0=BLOCKED)", "gauge")
	for _, l := range st.Listeners {
		b.WriteString(fmt.Sprintf("oratui_listener_up{host=%q,cluster=%q,listener=%q,type=%q,node=%q,endpoint=%q,ready_status=%q} %.0f\n",
			host, cluster, l.Name, l.Type, l.Node, l.Endpoint, l.ReadyStatus, boolToFloat(isUpStatus(l.State))))
		b.WriteString(fmt.Sprintf("oratui_listener_services_ready{host=%q,cluster=%q,listener=%q,node=%q} %d\n",
			host, cluster, l.Name, l.Node, l.Services))
		for _, sh := range l.ServiceList {
			sVal := 0.0
			switch strings.ToUpper(sh.Status) {
			case "READY":
				sVal = 1.0
			case "UNKNOWN":
				sVal = 0.5
			}
			b.WriteString(fmt.Sprintf("oratui_listener_service_status{host=%q,cluster=%q,listener=%q,service=%q,instance=%q,status=%q} %g\n",
				host, cluster, sh.ListenerName, sh.ServiceName, sh.InstanceName, sh.Status, sVal))
		}
	}

	// 10. RMAN Backups, Restore Points & Data Guard Standby
	writeHelpType("oratui_rman_backup_status", "RMAN backup job status (1=COMPLETED, 0.5=RUNNING/WARNING, 0=FAILED)", "gauge")
	writeHelpType("oratui_rman_backup_size_gb", "RMAN backup job size in GB", "gauge")
	for _, rb := range st.RMANBackups {
		stVal := 0.0
		up := strings.ToUpper(rb.Status)
		if strings.Contains(up, "COMPLETED") && !strings.Contains(up, "ERROR") {
			stVal = 1.0
		} else if strings.Contains(up, "RUNNING") || strings.Contains(up, "WARN") {
			stVal = 0.5
		}
		labels := fmt.Sprintf("host=%q,cluster=%q,session_key=%q,input_type=%q,device_type=%q,start_time=%q,status=%q",
			host, cluster, strconv.Itoa(rb.SessionKey), rb.InputType, rb.DeviceType, rb.StartTime, rb.Status)
		b.WriteString(fmt.Sprintf("oratui_rman_backup_status{%s} %g\n", labels, stVal))
		b.WriteString(fmt.Sprintf("oratui_rman_backup_size_gb{%s} %.2f\n", labels, rb.SizeGB))
	}

	writeHelpType("oratui_restore_points_total", "Number of active Flashback Restore Points", "gauge")
	b.WriteString(fmt.Sprintf("oratui_restore_points_total{host=%q,cluster=%q} %d\n", host, cluster, len(st.RestorePoints)))

	writeHelpType("oratui_dataguard_mrp_up", "Data Guard Managed Recovery Process status (1=APPLYING_LOG, 0=STOPPED)", "gauge")
	writeHelpType("oratui_dataguard_transport_lag_seconds", "Data Guard redo transport lag in seconds", "gauge")
	writeHelpType("oratui_dataguard_apply_lag_seconds", "Data Guard redo apply lag in seconds", "gauge")
	for _, dg := range st.StandbyDBs {
		mrpUp := boolToFloat(strings.Contains(strings.ToUpper(dg.MRPStatus), "APPLY") || isUpStatus(dg.MRPStatus))
		labels := fmt.Sprintf("host=%q,cluster=%q,standby_db=%q,role=%q,standby_host=%q,open_mode=%q,switchover=%q",
			host, cluster, dg.DBUniqueName, dg.Role, dg.Host, dg.OpenMode, dg.Switchover)
		b.WriteString(fmt.Sprintf("oratui_dataguard_mrp_up{%s} %.0f\n", labels, mrpUp))
		b.WriteString(fmt.Sprintf("oratui_dataguard_transport_lag_seconds{%s} %d\n", labels, parseLagSeconds(dg.TransportLag)))
		b.WriteString(fmt.Sprintf("oratui_dataguard_apply_lag_seconds{%s} %d\n", labels, parseLagSeconds(dg.ApplyLag)))
	}

	// 11. Alert Log & Strict ORA- Errors
	writeHelpType("oratui_alert_log_size_mb", "Size of the monitored Oracle alert_<SID>.log file in MB", "gauge")
	b.WriteString(fmt.Sprintf("oratui_alert_log_size_mb{host=%q,cluster=%q,trace_dir=%q} %.2f\n",
		host, cluster, st.DiagTraceDir, st.AlertFileSizeMB))

	writeHelpType("oratui_alert_ora_errors_total", "Total number of recent ORA- errors detected in alert.log", "gauge")
	b.WriteString(fmt.Sprintf("oratui_alert_ora_errors_total{host=%q,cluster=%q} %d\n", host, cluster, len(st.AlertEntries)))

	writeHelpType("oratui_alert_ora_error_count", "Occurrences of each specific ORA- error code in alert.log", "gauge")
	type oraKey struct {
		code     string
		instance string
		severity string
	}
	oraCounts := make(map[oraKey]int)
	for _, a := range st.AlertEntries {
		k := oraKey{code: a.ErrorCode, instance: a.Instance, severity: a.Severity}
		oraCounts[k]++
	}
	var keys []oraKey
	for k := range oraCounts {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i].code < keys[j].code })
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("oratui_alert_ora_error_count{host=%q,cluster=%q,error_code=%q,instance=%q,severity=%q} %d\n",
			host, cluster, k.code, k.instance, k.severity, oraCounts[k]))
	}

	// 12. Linux Server Health Checks (CPU/Load, RAM/Swap, HugePages/THP, AIO, File Descriptors)
	writeHelpType("oratui_os_metric_used_percent", "Linux server subsystem utilization percentage", "gauge")
	writeHelpType("oratui_os_metric_healthy", "Linux server subsystem health check (1=ONLINE/OK, 0=WARNING/CRITICAL)", "gauge")
	for _, m := range st.OSMetrics {
		healthy := boolToFloat(isUpStatus(m.Status))
		b.WriteString(fmt.Sprintf("oratui_os_metric_used_percent{host=%q,cluster=%q,category=%q,component=%q,status=%q,current_value=%q} %.2f\n",
			host, cluster, m.Category, m.Component, m.Status, m.CurrentValue, m.UsedPct))
		b.WriteString(fmt.Sprintf("oratui_os_metric_healthy{host=%q,cluster=%q,category=%q,component=%q,status=%q} %.0f\n",
			host, cluster, m.Category, m.Component, m.Status, healthy))
	}

	// 13. Mounted Filesystems & Inode Exhaustion (df -PTm / df -Pi)
	writeHelpType("oratui_filesystem_total_mb", "Mounted filesystem total capacity in MB", "gauge")
	writeHelpType("oratui_filesystem_used_mb", "Mounted filesystem used space in MB", "gauge")
	writeHelpType("oratui_filesystem_avail_mb", "Mounted filesystem available space in MB", "gauge")
	writeHelpType("oratui_filesystem_used_percent", "Mounted filesystem space used percentage", "gauge")
	writeHelpType("oratui_filesystem_inodes_used_percent", "Mounted filesystem inode usage percentage (prevents ORA-09925)", "gauge")
	for _, fs := range st.Filesystems {
		labels := fmt.Sprintf("host=%q,cluster=%q,mountpoint=%q,device=%q,fstype=%q,status=%q",
			host, cluster, fs.MountPoint, fs.Device, fs.FSType, fs.Status)
		b.WriteString(fmt.Sprintf("oratui_filesystem_total_mb{%s} %d\n", labels, fs.TotalMB))
		b.WriteString(fmt.Sprintf("oratui_filesystem_used_mb{%s} %d\n", labels, fs.UsedMB))
		b.WriteString(fmt.Sprintf("oratui_filesystem_avail_mb{%s} %d\n", labels, fs.AvailMB))
		b.WriteString(fmt.Sprintf("oratui_filesystem_used_percent{%s} %.2f\n", labels, fs.UsedPct))
		b.WriteString(fmt.Sprintf("oratui_filesystem_inodes_used_percent{%s} %.2f\n", labels, fs.InodeUsedPct))
	}

	// 14. SAN Multipath LUNs, Fibre Channel HBAs & Block Disks
	writeHelpType("oratui_storage_path_healthy", "SAN Multipath LUN, FC HBA port, or Block Disk path health (1=ALL PATHS ONLINE, 0=DEGRADED/FAULTY/OFFLINE)", "gauge")
	for _, sp := range st.StoragePaths {
		healthy := boolToFloat(isUpStatus(sp.Status) && !strings.Contains(strings.ToUpper(sp.PathsSummary), "DEGRADED") && !strings.Contains(strings.ToUpper(sp.PathsSummary), "DOWN"))
		b.WriteString(fmt.Sprintf("oratui_storage_path_healthy{host=%q,cluster=%q,name=%q,type=%q,size=%q,wwid_model=%q,paths_summary=%q,status=%q} %.0f\n",
			host, cluster, sp.Name, sp.Type, sp.Size, sp.WWIDOrModel, sp.PathsSummary, sp.Status, healthy))
	}

	return b.String()
}

// handleOTelJSONMetrics serves the OpenTelemetry OTLP/HTTP JSON ResourceMetrics representation
// so standard OpenTelemetry collectors or OTLP HTTP consumers can ingest metrics directly.
func (e *Exporter) handleOTelJSONMetrics(w http.ResponseWriter, r *http.Request) {
	st, updated := e.Snapshot()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	unixNano := updated.UnixNano()

	type otelAttrVal struct {
		StringValue string `json:"stringValue,omitempty"`
	}
	type otelAttr struct {
		Key   string      `json:"key"`
		Value otelAttrVal `json:"value"`
	}
	type otelDataPoint struct {
		Attributes   []otelAttr `json:"attributes,omitempty"`
		TimeUnixNano int64      `json:"timeUnixNano"`
		AsDouble     float64    `json:"asDouble"`
	}
	type otelGauge struct {
		DataPoints []otelDataPoint `json:"dataPoints"`
	}
	type otelMetric struct {
		Name        string    `json:"name"`
		Description string    `json:"description"`
		Unit        string    `json:"unit,omitempty"`
		Gauge       otelGauge `json:"gauge"`
	}

	attr := func(k, v string) otelAttr {
		return otelAttr{Key: k, Value: otelAttrVal{StringValue: v}}
	}

	var metrics []otelMetric

	// Instances
	var instPoints []otelDataPoint
	for _, inst := range st.Instances {
		v := 0.0
		if strings.Contains(strings.ToUpper(inst.Status), "OPEN") {
			v = 1.0
		}
		instPoints = append(instPoints, otelDataPoint{
			TimeUnixNano: unixNano,
			AsDouble:     v,
			Attributes:   []otelAttr{attr("instance", inst.InstName), attr("db_name", inst.DBName), attr("node", inst.NodeName), attr("open_mode", inst.OpenMode)},
		})
	}
	metrics = append(metrics, otelMetric{Name: "oratui.instance.up", Description: "Oracle instance status", Unit: "1", Gauge: otelGauge{DataPoints: instPoints}})

	// Tablespaces
	var tbsPoints []otelDataPoint
	for _, ts := range st.Tablespaces {
		tbsPoints = append(tbsPoints, otelDataPoint{
			TimeUnixNano: unixNano,
			AsDouble:     ts.UsedPct,
			Attributes:   []otelAttr{attr("tablespace", ts.Name), attr("type", ts.Type), attr("autoextend", ts.AutoExtend)},
		})
	}
	metrics = append(metrics, otelMetric{Name: "oratui.tablespace.used_percent", Description: "Tablespace used percentage", Unit: "%", Gauge: otelGauge{DataPoints: tbsPoints}})

	// OS Health
	var osPoints []otelDataPoint
	for _, m := range st.OSMetrics {
		osPoints = append(osPoints, otelDataPoint{
			TimeUnixNano: unixNano,
			AsDouble:     m.UsedPct,
			Attributes:   []otelAttr{attr("category", m.Category), attr("component", m.Component), attr("status", m.Status), attr("value", m.CurrentValue)},
		})
	}
	metrics = append(metrics, otelMetric{Name: "oratui.os.metric.used_percent", Description: "Linux server health metric percentage", Unit: "%", Gauge: otelGauge{DataPoints: osPoints}})

	// Filesystems & Inodes
	var fsSpacePoints, fsInodePoints []otelDataPoint
	for _, fs := range st.Filesystems {
		at := []otelAttr{attr("mountpoint", fs.MountPoint), attr("device", fs.Device), attr("fstype", fs.FSType)}
		fsSpacePoints = append(fsSpacePoints, otelDataPoint{TimeUnixNano: unixNano, AsDouble: fs.UsedPct, Attributes: at})
		fsInodePoints = append(fsInodePoints, otelDataPoint{TimeUnixNano: unixNano, AsDouble: fs.InodeUsedPct, Attributes: at})
	}
	metrics = append(metrics,
		otelMetric{Name: "oratui.filesystem.used_percent", Description: "Filesystem space used percentage", Unit: "%", Gauge: otelGauge{DataPoints: fsSpacePoints}},
		otelMetric{Name: "oratui.filesystem.inodes_used_percent", Description: "Filesystem inode used percentage", Unit: "%", Gauge: otelGauge{DataPoints: fsInodePoints}},
	)

	// Storage & Multipath
	var pathPoints []otelDataPoint
	for _, sp := range st.StoragePaths {
		healthy := boolToFloat(isUpStatus(sp.Status) && !strings.Contains(strings.ToUpper(sp.PathsSummary), "DEGRADED"))
		pathPoints = append(pathPoints, otelDataPoint{
			TimeUnixNano: unixNano,
			AsDouble:     healthy,
			Attributes:   []otelAttr{attr("name", sp.Name), attr("type", sp.Type), attr("paths_summary", sp.PathsSummary), attr("wwid_model", sp.WWIDOrModel)},
		})
	}
	metrics = append(metrics, otelMetric{Name: "oratui.storage.path.healthy", Description: "SAN Multipath & block device path health", Unit: "1", Gauge: otelGauge{DataPoints: pathPoints}})

	payload := map[string]interface{}{
		"resourceMetrics": []map[string]interface{}{
			{
				"resource": map[string]interface{}{
					"attributes": []otelAttr{
						attr("service.name", "ora-tui"),
						attr("service.version", e.version),
						attr("host.name", st.HostName),
						attr("oracle.cluster", st.ClusterName),
						attr("oracle.topology", string(st.Topology)),
						attr("oracle.version", st.ReleaseVersion),
					},
				},
				"scopeMetrics": []map[string]interface{}{
					{
						"scope": map[string]string{
							"name":    "github.com/mario/ora-tui/internal/telemetry",
							"version": e.version,
						},
						"metrics": metrics,
					},
				},
			},
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(payload)
}

func isUpStatus(s string) bool {
	u := strings.ToUpper(strings.TrimSpace(s))
	if u == "" {
		return false
	}
	if strings.Contains(u, "OFFLINE") || strings.Contains(u, "DOWN") || strings.Contains(u, "FAIL") ||
		strings.Contains(u, "CRIT") || strings.Contains(u, "WARN") || strings.Contains(u, "DEGRAD") ||
		strings.Contains(u, "FAULT") || strings.Contains(u, "STALE") || strings.Contains(u, "INVALID") {
		return false
	}
	return true
}

func boolToFloat(b bool) float64 {
	if b {
		return 1.0
	}
	return 0.0
}

var lagRe = regexp.MustCompile(`\+?(\d{2}):(\d{2}):(\d{2})`)

func parseLagSeconds(lag string) int {
	m := lagRe.FindStringSubmatch(lag)
	if len(m) == 4 {
		h, _ := strconv.Atoi(m[1])
		min, _ := strconv.Atoi(m[2])
		sec, _ := strconv.Atoi(m[3])
		return h*3600 + min*60 + sec
	}
	return 0
}
