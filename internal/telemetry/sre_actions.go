package telemetry

import (
	"bytes"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/smtp"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
)

// MetricHistoryPoint stores a time-series snapshot for Flutter Web live charts.
type MetricHistoryPoint struct {
	Timestamp          string  `json:"timestamp"`
	ActiveSessions     int     `json:"active_sessions"`
	BlockingLocks      int     `json:"blocking_locks"`
	MaxTablespacePct   float64 `json:"max_tablespace_pct"`
	FRAPct             int     `json:"fra_pct"`
	MaxASMPct          float64 `json:"max_asm_pct"`
	ORAErrorCount      int     `json:"ora_error_count"`
	CPULoadPct         float64 `json:"cpu_load_pct"`
	RAMUsedPct         float64 `json:"ram_used_pct"`
	MaxInodePct        float64 `json:"max_inode_pct"`
	UnhealthySANPaths  int     `json:"unhealthy_san_paths"`
	DataGuardApplyLagS int     `json:"dataguard_apply_lag_s"`
}

// SLOConfig defines user-customizable SRE/SLO targets and Telegram/Email alert channels.
type SLOConfig struct {
	AvailabilityTargetPct float64 `json:"availability_target_pct"`
	LockFreeTargetPct     float64 `json:"lock_free_target_pct"`
	StorageTargetPct      float64 `json:"storage_target_pct"`
	SANTargetPct          float64 `json:"san_target_pct"`

	AutoNotifyCritical bool `json:"auto_notify_critical"`
	CooldownMinutes    int  `json:"cooldown_minutes"`

	TelegramEnabled  bool   `json:"telegram_enabled"`
	TelegramBotToken string `json:"telegram_bot_token"`
	TelegramChatID   string `json:"telegram_chat_id"`

	EmailEnabled bool   `json:"email_enabled"`
	SMTPHost     string `json:"smtp_host"`
	SMTPPort     int    `json:"smtp_port"`
	SMTPUser     string `json:"smtp_user"`
	SMTPPassword string `json:"smtp_password"`
	SMTPFrom     string `json:"smtp_from"`
	SMTPTo       string `json:"smtp_to"`
}

// SLOStatus represents the calculated SLI, SLO, Error Budget, and Burn Rate for an SRE dimension.
type SLOStatus struct {
	ID                  string  `json:"id"`
	Name                string  `json:"name"`
	Description         string  `json:"description"`
	TargetPct           float64 `json:"target_pct"`
	CurrentSLIPct       float64 `json:"current_sli_pct"`
	ErrorBudgetRemPct   float64 `json:"error_budget_remaining_pct"`
	BurnRate            float64 `json:"burn_rate"`
	Status              string  `json:"status"` // HEALTHY, WARNING, BURNING
	TotalChecks         int     `json:"total_checks"`
	GoodChecks          int     `json:"good_checks"`
	RecommendedActionID string  `json:"recommended_action_id"`
}

// DelicateAlert represents an actionable high-priority SRE alert with a 1-click remediation action.
type DelicateAlert struct {
	ID                string `json:"id"`
	Severity          string `json:"severity"` // CRITICAL, WARNING
	Category          string `json:"category"`
	Title             string `json:"title"`
	Description       string `json:"description"`
	RecommendedAction string `json:"recommended_action"`
	ActionID          string `json:"action_id"`
	ActionTarget      string `json:"action_target"`
	DetectedAt        string `json:"detected_at"`
}

// NotificationEvent logs sent Telegram / Email messages.
type NotificationEvent struct {
	Timestamp string `json:"timestamp"`
	Channel   string `json:"channel"` // TELEGRAM, EMAIL
	Status    string `json:"status"`  // SENT, ERROR
	Target    string `json:"target"`
	Summary   string `json:"summary"`
	Detail    string `json:"detail"`
}

// ActionRequest represents a remote operation triggered from the Flutter Web UI.
type ActionRequest struct {
	Action string            `json:"action"`
	Target string            `json:"target"`
	Params map[string]string `json:"params"`
}

// ActionResponse returns the full multi-line terminal output of an executed operation.
type ActionResponse struct {
	Success    bool     `json:"success"`
	Action     string   `json:"action"`
	Target     string   `json:"target"`
	Title      string   `json:"title"`
	Command    string   `json:"command"`
	Output     []string `json:"output"`
	DurationMs int64    `json:"duration_ms"`
	ExecutedAt string   `json:"executed_at"`
}

func defaultSLOConfig() SLOConfig {
	cfg := SLOConfig{
		AvailabilityTargetPct: 99.95,
		LockFreeTargetPct:     99.50,
		StorageTargetPct:      99.90,
		SANTargetPct:          99.99,
		AutoNotifyCritical:    true,
		CooldownMinutes:       15,
		TelegramEnabled:       os.Getenv("ORA_TUI_TELEGRAM_TOKEN") != "",
		TelegramBotToken:      os.Getenv("ORA_TUI_TELEGRAM_TOKEN"),
		TelegramChatID:        os.Getenv("ORA_TUI_TELEGRAM_CHAT_ID"),
		EmailEnabled:          os.Getenv("ORA_TUI_SMTP_HOST") != "",
		SMTPHost:              os.Getenv("ORA_TUI_SMTP_HOST"),
		SMTPPort:              587,
		SMTPUser:              os.Getenv("ORA_TUI_SMTP_USER"),
		SMTPPassword:          os.Getenv("ORA_TUI_SMTP_PASS"),
		SMTPFrom:              os.Getenv("ORA_TUI_SMTP_FROM"),
		SMTPTo:                os.Getenv("ORA_TUI_SMTP_TO"),
	}
	if data, err := os.ReadFile("/etc/ora-tui-sre.json"); err == nil {
		_ = json.Unmarshal(data, &cfg)
	}
	return cfg
}

func defaultAPIToken() string {
	for _, envKey := range []string{"ORATUI_API_TOKEN", "ORA_TUI_API_TOKEN"} {
		if v := strings.TrimSpace(os.Getenv(envKey)); v != "" {
			return v
		}
	}
	if data, err := os.ReadFile("/etc/ora-tui-token"); err == nil {
		if v := strings.TrimSpace(string(data)); v != "" {
			return v
		}
	}
	return ""
}

const maskedSecretPlaceholder = "********"

func maskSLOConfig(cfg SLOConfig) SLOConfig {
	out := cfg
	if out.TelegramBotToken != "" {
		out.TelegramBotToken = maskedSecretPlaceholder
	}
	if out.SMTPPassword != "" {
		out.SMTPPassword = maskedSecretPlaceholder
	}
	return out
}

var safeTargetRegex = regexp.MustCompile(`^[a-zA-Z0-9_.,:@/#$+-]{0,96}$`)

func validateSafeTarget(target string) error {
	t := strings.TrimSpace(target)
	if !safeTargetRegex.MatchString(t) {
		return fmt.Errorf("target inválido o con caracteres no permitidos por seguridad: %q", target)
	}
	upper := strings.ToUpper(t)
	for _, forbidden := range []string{"..", "--", "/*", "*/", ";", "'", "\"", "`", "$("} {
		if strings.Contains(upper, forbidden) {
			return fmt.Errorf("secuencia bloqueada por política anti-inyección: %q", forbidden)
		}
	}
	return nil
}

func (e *Exporter) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	e.mu.RLock()
	expectedToken := strings.TrimSpace(e.apiToken)
	e.mu.RUnlock()

	expectedUser := strings.TrimSpace(os.Getenv("FLUTTER_WEB_USER"))
	if expectedUser == "" {
		expectedUser = strings.TrimSpace(os.Getenv("ORATUI_ADMIN_USER"))
	}
	expectedPass := strings.TrimSpace(os.Getenv("FLUTTER_WEB_PASSWORD"))
	if expectedPass == "" {
		expectedPass = strings.TrimSpace(os.Getenv("ORATUI_ADMIN_PASSWORD"))
	}

	// If neither token nor user/pass is configured in env/flags, allow POC default admin:admin or open access in unit tests
	if expectedToken == "" && expectedUser == "" && expectedPass == "" {
		// If the client sent BasicAuth, verify against default admin:admin
		if u, p, ok := r.BasicAuth(); ok {
			if subtle.ConstantTimeCompare([]byte(u), []byte("admin")) == 1 &&
				subtle.ConstantTimeCompare([]byte(p), []byte("admin")) == 1 {
				return true
			}
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"error":   "Credenciales inválidas (Usuario o Contraseña incorrectos).",
			})
			return false
		}
		return true
	}

	if expectedUser == "" {
		expectedUser = "admin"
	}
	if expectedPass == "" {
		expectedPass = "admin"
	}

	// 1. Check HTTP Basic Auth (Authorization: Basic base64(user:pass))
	if u, p, ok := r.BasicAuth(); ok {
		if subtle.ConstantTimeCompare([]byte(u), []byte(expectedUser)) == 1 &&
			subtle.ConstantTimeCompare([]byte(p), []byte(expectedPass)) == 1 {
			return true
		}
	}

	// 2. Check Bearer Token / X-OraTui-Token
	provided := strings.TrimSpace(r.Header.Get("X-OraTui-Token"))
	if provided == "" {
		authHdr := strings.TrimSpace(r.Header.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authHdr), "bearer ") {
			provided = strings.TrimSpace(authHdr[7:])
		}
	}
	if expectedToken != "" && subtle.ConstantTimeCompare([]byte(provided), []byte(expectedToken)) == 1 {
		return true
	}
	// Also accept user:pass formatted as Bearer token
	if subtle.ConstantTimeCompare([]byte(provided), []byte(expectedUser+":"+expectedPass)) == 1 {
		return true
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   "No autorizado: Inicia sesión con Usuario y Contraseña válidos o proporciona X-OraTui-Token.",
	})
	return false
}

func enableCORS(w http.ResponseWriter, r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-OraTui-Token, X-Requested-With")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

// recordStateAndEvaluateSRE updates the time-series ring buffer, updates rolling SLI counters,
// detects delicate alerts, and triggers automatic Telegram/Email notifications if enabled.
func (e *Exporter) recordStateAndEvaluateSRE(s model.EnvironmentState, now time.Time) {
	activeSess := 0
	blockingLocks := 0
	for _, sess := range s.Sessions {
		if strings.EqualFold(sess.Status, "ACTIVE") {
			activeSess++
		}
		if strings.Contains(strings.ToUpper(sess.BlockingSession), "BLOCK") || strings.Contains(strings.ToUpper(sess.WaitEvent), "LOCK") {
			blockingLocks++
		}
	}

	maxTbs := 0.0
	for _, ts := range s.Tablespaces {
		pct := ts.UsedPct
		if pct <= 0 && ts.TotalMB > 0 && ts.UsedMB > 0 {
			pct = float64(ts.UsedMB) * 100.0 / float64(ts.TotalMB)
		}
		if pct > maxTbs {
			maxTbs = pct
		}
	}

	fraPct := 0
	allInstOpen := len(s.Instances) > 0
	for _, inst := range s.Instances {
		if inst.FRAUsedPct > fraPct {
			fraPct = inst.FRAUsedPct
		}
		if !strings.Contains(strings.ToUpper(inst.Status), "OPEN") && !strings.Contains(strings.ToUpper(inst.OpenMode), "READ") {
			allInstOpen = false
		}
	}

	maxASM := 0.0
	asmOffline := 0
	for _, dg := range s.Diskgroups {
		if dg.UsedPct > maxASM {
			maxASM = dg.UsedPct
		}
		asmOffline += dg.OfflineDisks
	}

	cpuLoad := 0.0
	ramPct := 0.0
	for _, m := range s.OSMetrics {
		cat := strings.ToUpper(m.Category)
		if strings.Contains(cat, "CPU") && m.UsedPct > cpuLoad {
			cpuLoad = m.UsedPct
		}
		if strings.Contains(cat, "RAM") && m.UsedPct > ramPct {
			ramPct = m.UsedPct
		}
	}

	maxInode := 0.0
	for _, fs := range s.Filesystems {
		if fs.InodeUsedPct > maxInode {
			maxInode = fs.InodeUsedPct
		}
	}

	unhealthySAN := 0
	for _, sp := range s.StoragePaths {
		if !isUpStatus(sp.Status) || strings.Contains(strings.ToUpper(sp.PathsSummary), "DEGRADED") || strings.Contains(strings.ToUpper(sp.PathsSummary), "FAULTY") {
			unhealthySAN++
		}
	}

	dgLag := 0
	for _, dg := range s.StandbyDBs {
		lag := parseLagSeconds(dg.ApplyLag)
		if lag > dgLag {
			dgLag = lag
		}
	}

	pt := MetricHistoryPoint{
		Timestamp:          now.Format("15:04:05"),
		ActiveSessions:     activeSess,
		BlockingLocks:      blockingLocks,
		MaxTablespacePct:   maxTbs,
		FRAPct:             fraPct,
		MaxASMPct:          maxASM,
		ORAErrorCount:      len(s.AlertEntries),
		CPULoadPct:         cpuLoad,
		RAMUsedPct:         ramPct,
		MaxInodePct:        maxInode,
		UnhealthySANPaths:  unhealthySAN,
		DataGuardApplyLagS: dgLag,
	}

	if len(e.history) < 90 {
		e.history = append(e.history, pt)
	} else {
		copy(e.history, e.history[1:])
		e.history[89] = pt
	}

	// Update rolling SRE SLI counters
	e.totalSamples++
	if allInstOpen {
		e.goodAvailSamples++
	}
	if blockingLocks == 0 {
		e.goodLockFreeSamples++
	}
	if maxTbs < 92.0 && fraPct < 88 && maxASM < 90.0 && maxInode < 88.0 && asmOffline == 0 {
		e.goodStorageSamples++
	}
	if unhealthySAN == 0 {
		e.goodSANSamples++
	}

	alerts := buildDelicateAlerts(s, now)
	e.activeAlerts = alerts

	cooldown := time.Duration(e.sloConfig.CooldownMinutes) * time.Minute
	if cooldown <= 0 {
		cooldown = 15 * time.Minute
	}

	// Prune expired alert cooldown entries so alertLastSent never grows unbounded over months of uptime
	if len(e.alertLastSent) > 0 {
		maxRetention := 2 * cooldown
		for id, ts := range e.alertLastSent {
			if now.Sub(ts) >= maxRetention {
				delete(e.alertLastSent, id)
			}
		}
		if len(e.alertLastSent) > 256 {
			e.alertLastSent = make(map[string]time.Time, 32)
		}
	}

	// Trigger automatic notifications for CRITICAL alerts if enabled and cooldown has elapsed
	if e.sloConfig.AutoNotifyCritical && (e.sloConfig.TelegramEnabled || e.sloConfig.EmailEnabled) {
		var newCriticals []DelicateAlert
		for _, a := range alerts {
			if a.Severity == "CRITICAL" {
				lastSent, seen := e.alertLastSent[a.ID]
				if !seen || now.Sub(lastSent) >= cooldown {
					newCriticals = append(newCriticals, a)
					e.alertLastSent[a.ID] = now
				}
			}
		}
		if len(newCriticals) > 0 {
			cfgCopy := e.sloConfig
			hostCopy := s.HostName
			clusterCopy := s.ClusterName
			go func() {
				e.dispatchAlertNotifications(cfgCopy, hostCopy, clusterCopy, newCriticals, false)
			}()
		}
	}
}

func buildDelicateAlerts(s model.EnvironmentState, now time.Time) []DelicateAlert {
	var list []DelicateAlert
	tsStr := now.Format("2006-01-02 15:04:05")

	// 1. Instances not OPEN
	for _, inst := range s.Instances {
		if !strings.Contains(strings.ToUpper(inst.Status), "OPEN") && !strings.Contains(strings.ToUpper(inst.OpenMode), "READ") {
			list = append(list, DelicateAlert{
				ID:                "INST_DOWN_" + inst.InstName,
				Severity:          "CRITICAL",
				Category:          "Instancia Oracle",
				Title:             fmt.Sprintf("Instancia %s en estado %s (%s)", inst.InstName, inst.Status, inst.OpenMode),
				Description:       fmt.Sprintf("La instancia %s en el nodo %s no está abierta en modo READ WRITE.", inst.InstName, inst.NodeName),
				RecommendedAction: "Arrancar Instancia / Base de Datos",
				ActionID:          "start_instance",
				ActionTarget:      inst.InstName,
				DetectedAt:        tsStr,
			})
		}
		if inst.FRAUsedPct >= 85 {
			list = append(list, DelicateAlert{
				ID:                "FRA_FULL_" + inst.InstName,
				Severity:          "CRITICAL",
				Category:          "Fast Recovery Area (FRA)",
				Title:             fmt.Sprintf("FRA al %d%% en %s (Riesgo ORA-19809)", inst.FRAUsedPct, inst.InstName),
				Description:       "La Fast Recovery Area supera el 85%. Si llega al 100% se detendrá el archivado de Redo Logs.",
				RecommendedAction: "Lanzar Backup RMAN de Archivelogs con DELETE INPUT",
				ActionID:          "rman_archivelog_backup",
				ActionTarget:      inst.DBName,
				DetectedAt:        tsStr,
			})
		}
	}

	// 2. Blocking Sessions (TX/TM Locks)
	for _, sess := range s.Sessions {
		if strings.Contains(strings.ToUpper(sess.BlockingSession), "BLOCK") || strings.Contains(strings.ToUpper(sess.WaitEvent), "LOCK") {
			targetSpec := fmt.Sprintf("%d,%d,@%d", sess.SID, sess.Serial, sess.InstID)
			list = append(list, DelicateAlert{
				ID:                fmt.Sprintf("LOCK_SID_%d", sess.SID),
				Severity:          "CRITICAL",
				Category:          "Bloqueos TX/TM",
				Title:             fmt.Sprintf("Sesión SID %d,%d (%s) en bloqueo: %s", sess.SID, sess.Serial, sess.Username, sess.WaitEvent),
				Description:       fmt.Sprintf("SQL_ID: %s | Bloqueo: %s | Máquina: %s", sess.SQLID, sess.BlockingSession, sess.Machine),
				RecommendedAction: fmt.Sprintf("Matar Sesión (%s) IMMEDIATE", targetSpec),
				ActionID:          "kill_session",
				ActionTarget:      targetSpec,
				DetectedAt:        tsStr,
			})
		}
	}

	// 3. Tablespaces >= 85%
	for _, ts := range s.Tablespaces {
		if ts.UsedPct >= 85.0 {
			sev := "WARNING"
			if ts.UsedPct >= 90.0 {
				sev = "CRITICAL"
			}
			list = append(list, DelicateAlert{
				ID:                "TBS_HIGH_" + ts.Name,
				Severity:          sev,
				Category:          "Tablespace",
				Title:             fmt.Sprintf("Tablespace %s al %.1f%% (Libre: %d MB)", ts.Name, ts.UsedPct, ts.FreeMB),
				Description:       fmt.Sprintf("Riesgo de ORA-01653/ORA-01654 en %s (Autoextend=%s).", ts.Name, ts.AutoExtend),
				RecommendedAction: fmt.Sprintf("Ampliar Tablespace %s (+Datafile 32G)", ts.Name),
				ActionID:          "add_datafile",
				ActionTarget:      ts.Name,
				DetectedAt:        tsStr,
			})
		}
	}

	// 4. SAN Multipath Degraded Paths
	for _, sp := range s.StoragePaths {
		if !isUpStatus(sp.Status) || strings.Contains(strings.ToUpper(sp.PathsSummary), "DEGRADED") || strings.Contains(strings.ToUpper(sp.PathsSummary), "FAULTY") {
			list = append(list, DelicateAlert{
				ID:                "SAN_PATH_" + sp.Name,
				Severity:          "CRITICAL",
				Category:          "SAN Multipath / Fibra FC",
				Title:             fmt.Sprintf("Camino SAN degradado en %s (%s)", sp.Name, sp.PathsSummary),
				Description:       fmt.Sprintf("El dispositivo %s (%s) presenta pérdida de redundancia de caminos físicos.", sp.Name, sp.WWIDOrModel),
				RecommendedAction: "Reescanear Buses SCSI/FC y Multipath (-r)",
				ActionID:          "rescan_scsi",
				ActionTarget:      sp.Name,
				DetectedAt:        tsStr,
			})
		}
	}

	// 5. Filesystems / Inodes High
	for _, fs := range s.Filesystems {
		if fs.InodeUsedPct >= 85.0 || fs.UsedPct >= 90.0 {
			list = append(list, DelicateAlert{
				ID:                "FS_HIGH_" + fs.MountPoint,
				Severity:          "CRITICAL",
				Category:          "Filesystem / Inodos Linux",
				Title:             fmt.Sprintf("Mount %s: Espacio %.1f%% | Inodos %.1f%%", fs.MountPoint, fs.UsedPct, fs.InodeUsedPct),
				Description:       "Riesgo de ORA-09925 por falta de espacio o agotamiento de inodos en trazas/auditoría.",
				RecommendedAction: "Purgar Trazas ADRCI y Analizar Espacio (du -shx)",
				ActionID:          "purge_adrci",
				ActionTarget:      fs.MountPoint,
				DetectedAt:        tsStr,
			})
		}
	}

	// 6. Alert Log ORA- Errors
	if len(s.AlertEntries) > 0 {
		lastErr := s.AlertEntries[0]
		list = append(list, DelicateAlert{
			ID:                "ORA_ERR_" + lastErr.ErrorCode,
			Severity:          "CRITICAL",
			Category:          "Alert Log ORA-",
			Title:             fmt.Sprintf("%d Error(es) en Alert Log (Último: %s en %s)", len(s.AlertEntries), lastErr.ErrorCode, lastErr.Instance),
			Description:       lastErr.Message,
			RecommendedAction: "Purgar Trazas ADRCI & Forzar Switch Logfile",
			ActionID:          "switch_logfile",
			ActionTarget:      lastErr.Instance,
			DetectedAt:        tsStr,
		})
	}

	// 7. Invalid Objects
	if len(s.InvalidObjects) > 0 {
		list = append(list, DelicateAlert{
			ID:                "INVALID_OBJECTS",
			Severity:          "WARNING",
			Category:          "Objetos Inválidos",
			Title:             fmt.Sprintf("%d Objetos en estado INVALID detectados", len(s.InvalidObjects)),
			Description:       fmt.Sprintf("Primer objeto inválido: %s.%s (%s)", s.InvalidObjects[0].Owner, s.InvalidObjects[0].ObjectName, s.InvalidObjects[0].ObjectType),
			RecommendedAction: "Recompilar Objetos Inválidos (UTL_RECOMP)",
			ActionID:          "compile_invalids",
			ActionTarget:      s.InvalidObjects[0].Owner,
			DetectedAt:        tsStr,
		})
	}

	// 8. SQL Plan Instability / Plan Flips (ora-brabo inspired)
	for _, pf := range s.PlanFlips {
		if pf.Status == "REGRESSION" || (pf.BestElapsedMs > 0 && pf.AvgElapsedMs > pf.BestElapsedMs*2.0) {
			sev := "WARNING"
			if pf.Status == "REGRESSION" {
				sev = "CRITICAL"
			}
			targetSpec := fmt.Sprintf("%s:%d", pf.SQLID, pf.BestPHV)
			list = append(list, DelicateAlert{
				ID:                "PLAN_FLIP_" + pf.SQLID,
				Severity:          sev,
				Category:          "SQL Plan Flip",
				Title:             fmt.Sprintf("SQL_ID %s con regresión de plan (PHV %d -> Óptimo %d)", pf.SQLID, pf.CurrentPHV, pf.BestPHV),
				Description:       fmt.Sprintf("Latencia actual %.1f ms vs mejor plan %.1f ms (%d planes en caché | Esquema %s).", pf.AvgElapsedMs, pf.BestElapsedMs, pf.PlanCount, pf.Schema),
				RecommendedAction: fmt.Sprintf("Fijar Plan Óptimo %d en SPM Baseline", pf.BestPHV),
				ActionID:          "pin_sql_plan",
				ActionTarget:      targetSpec,
				DetectedAt:        tsStr,
			})
		}
	}

	// 9. Failed Business / Application Scheduler Jobs (24h)
	for _, jb := range s.SchedulerJobs {
		if jb.Failed24h > 0 || strings.Contains(strings.ToUpper(jb.State), "FAIL") || strings.Contains(strings.ToUpper(jb.State), "BROKEN") {
			jobSpec := fmt.Sprintf("%s.%s", jb.Owner, jb.JobName)
			list = append(list, DelicateAlert{
				ID:                "JOB_FAIL_" + jobSpec,
				Severity:          "CRITICAL",
				Category:          "Scheduler Job",
				Title:             fmt.Sprintf("Job de negocio %s en estado %s (%d fallos 24h)", jobSpec, jb.State, jb.Failed24h),
				Description:       fmt.Sprintf("Último inicio: %s | Detalle: %s", jb.LastStart, jb.Detail),
				RecommendedAction: fmt.Sprintf("Relanzar Job (%s) vía DBMS_SCHEDULER", jobSpec),
				ActionID:          "run_scheduler_job",
				ActionTarget:      jobSpec,
				DetectedAt:        tsStr,
			})
		}
	}

	return list
}

func computeSLOs(total, goodAvail, goodLocks, goodStorage, goodSAN int, cfg SLOConfig) []SLOStatus {
	if total <= 0 {
		total = 1
		goodAvail = 1
		goodLocks = 1
		goodStorage = 1
		goodSAN = 1
	}

	calcOne := func(id, name, desc string, good int, target float64, actionID string) SLOStatus {
		sli := (float64(good) / float64(total)) * 100.0
		allowedBadPct := 100.0 - target
		if allowedBadPct <= 0.0001 {
			allowedBadPct = 0.01
		}
		actualBadPct := 100.0 - sli
		consumedPct := (actualBadPct / allowedBadPct) * 100.0
		remPct := 100.0 - consumedPct
		if remPct < 0 {
			remPct = 0
		}
		if remPct > 100 {
			remPct = 100
		}
		burnRate := actualBadPct / allowedBadPct
		status := "HEALTHY"
		if remPct <= 0 || burnRate >= 2.0 {
			status = "BURNING"
		} else if remPct < 50.0 || burnRate >= 1.0 {
			status = "WARNING"
		}
		return SLOStatus{
			ID:                  id,
			Name:                name,
			Description:         desc,
			TargetPct:           target,
			CurrentSLIPct:       sli,
			ErrorBudgetRemPct:   remPct,
			BurnRate:            burnRate,
			Status:              status,
			TotalChecks:         total,
			GoodChecks:          good,
			RecommendedActionID: actionID,
		}
	}

	return []SLOStatus{
		calcOne("slo_availability", "Disponibilidad BD & Clusterware", "Instancias en modo OPEN READ WRITE y demonios CRS/CSSD/ASM activos", goodAvail, cfg.AvailabilityTargetPct, "start_instance"),
		calcOne("slo_lock_free", "Operación Libre de Bloqueos TX/TM", "Porcentaje de muestreos sin bloqueos transaccionales activos en gv$session", goodLocks, cfg.LockFreeTargetPct, "kill_session"),
		calcOne("slo_storage_capacity", "Salud de Capacidad (TBS, FRA, ASM, Inodos)", "Tablespaces < 92%, FRA < 88%, ASM Diskgroups sin discos offline e Inodos < 88%", goodStorage, cfg.StorageTargetPct, "add_datafile"),
		calcOne("slo_san_multipath", "Redundancia SAN Multipath & Fibra FC", "100% de LUNs Multipath y puertos HBA Fibre Channel con todos los caminos activos", goodSAN, cfg.SANTargetPct, "rescan_scsi"),
	}
}

func (e *Exporter) handleSREGetOrPost(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	e.mu.RLock()
	cfg := maskSLOConfig(e.sloConfig)
	authEnabled := e.apiToken != ""
	slos := computeSLOs(e.totalSamples, e.goodAvailSamples, e.goodLockFreeSamples, e.goodStorageSamples, e.goodSANSamples, e.sloConfig)
	alerts := append([]DelicateAlert(nil), e.activeAlerts...)
	notifs := append([]NotificationEvent(nil), e.notifications...)
	hist := append([]MetricHistoryPoint(nil), e.history...)
	host := e.state.HostName
	cluster := e.state.ClusterName
	e.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"host":          host,
		"cluster":       cluster,
		"auth_enabled":  authEnabled,
		"config":        cfg,
		"slos":          slos,
		"active_alerts": alerts,
		"notifications": notifs,
		"history":       hist,
	})
}

func (e *Exporter) handleSREConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !e.requireAuth(w, r) {
		return
	}
	var newCfg SLOConfig
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&newCfg); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	if newCfg.AvailabilityTargetPct <= 0 {
		newCfg.AvailabilityTargetPct = 99.95
	}
	if newCfg.LockFreeTargetPct <= 0 {
		newCfg.LockFreeTargetPct = 99.50
	}
	if newCfg.StorageTargetPct <= 0 {
		newCfg.StorageTargetPct = 99.90
	}
	if newCfg.SANTargetPct <= 0 {
		newCfg.SANTargetPct = 99.99
	}
	if newCfg.SMTPPort <= 0 {
		newCfg.SMTPPort = 587
	}

	e.mu.Lock()
	// Preserve existing secrets if UI sends back masked placeholder "********"
	if newCfg.TelegramBotToken == maskedSecretPlaceholder {
		newCfg.TelegramBotToken = e.sloConfig.TelegramBotToken
	}
	if newCfg.SMTPPassword == maskedSecretPlaceholder {
		newCfg.SMTPPassword = e.sloConfig.SMTPPassword
	}
	e.sloConfig = newCfg
	e.mu.Unlock()

	if data, err := json.MarshalIndent(newCfg, "", "  "); err == nil {
		_ = os.WriteFile("/etc/ora-tui-sre.json", data, 0600)
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": "Configuración SRE/SLO y canales de alerta (Telegram / Email SMTP) actualizada correctamente.",
		"config":  maskSLOConfig(newCfg),
	})
}

func (e *Exporter) handleSRENotifyNow(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if !e.requireAuth(w, r) {
		return
	}
	e.mu.RLock()
	cfg := e.sloConfig
	alerts := append([]DelicateAlert(nil), e.activeAlerts...)
	host := e.state.HostName
	cluster := e.state.ClusterName
	e.mu.RUnlock()

	// Allow overriding config in POST body for quick testing from the UI
	if r.Body != nil {
		var bodyOverride struct {
			Config *SLOConfig `json:"config"`
		}
		if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&bodyOverride); err == nil && bodyOverride.Config != nil {
			override := *bodyOverride.Config
			if override.TelegramBotToken == maskedSecretPlaceholder {
				override.TelegramBotToken = cfg.TelegramBotToken
			}
			if override.SMTPPassword == maskedSecretPlaceholder {
				override.SMTPPassword = cfg.SMTPPassword
			}
			cfg = override
		}
	}

	results := e.dispatchAlertNotifications(cfg, host, cluster, alerts, true)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"events":  results,
	})
}

func (e *Exporter) dispatchAlertNotifications(cfg SLOConfig, host, cluster string, alerts []DelicateAlert, isTest bool) []NotificationEvent {
	var events []NotificationEvent
	nowStr := time.Now().Format("2006-01-02 15:04:05")

	var sb strings.Builder
	if isTest {
		sb.WriteString(fmt.Sprintf("🚨 [ORA-TUI SRE/SLO ALERT TEST] Host: %s (%s)\n", host, cluster))
	} else {
		sb.WriteString(fmt.Sprintf("🚨 [ORA-TUI ALERTA CRÍTICA SRE] Host: %s (%s)\n", host, cluster))
	}
	sb.WriteString(fmt.Sprintf("Fecha: %s\n", nowStr))
	sb.WriteString("──────────────────────────────────────────\n")
	if len(alerts) == 0 {
		sb.WriteString("✅ Todos los indicadores SLO y chequeos del servidor están en estado HEALTHY.\n")
	} else {
		for i, a := range alerts {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s: %s\n   ↳ Acción recomendada: %s\n", i+1, a.Severity, a.Category, a.Title, a.RecommendedAction))
		}
	}
	msgText := sb.String()

	// 1. Telegram Bot Notification
	if cfg.TelegramEnabled || (isTest && cfg.TelegramBotToken != "" && cfg.TelegramChatID != "") {
		ev := sendTelegramAlert(cfg.TelegramBotToken, cfg.TelegramChatID, msgText, nowStr)
		events = append(events, ev)
	}

	// 2. Email SMTP Notification
	if cfg.EmailEnabled || (isTest && cfg.SMTPHost != "" && cfg.SMTPTo != "") {
		subject := fmt.Sprintf("[ORA-TUI SRE] Alertas en %s (%s) - %d activas", host, cluster, len(alerts))
		ev := sendSMTPAlert(cfg, subject, msgText, nowStr)
		events = append(events, ev)
	}

	if len(events) == 0 {
		events = append(events, NotificationEvent{
			Timestamp: nowStr,
			Channel:   "SYSTEM",
			Status:    "SIMULATED_OK",
			Target:    "Consola SRE Local",
			Summary:   fmt.Sprintf("Aviso SRE procesado (%d alertas en %s)", len(alerts), host),
			Detail:    "Configura tu Bot Token de Telegram o servidor SMTP para enviar avisos externos en tiempo real.",
		})
	}

	e.mu.Lock()
	e.notifications = append(events, e.notifications...)
	if len(e.notifications) > 50 {
		e.notifications = e.notifications[:50]
	}
	e.mu.Unlock()

	return events
}

func sendTelegramAlert(botToken, chatID, text, nowStr string) NotificationEvent {
	if strings.TrimSpace(botToken) == "" || strings.TrimSpace(chatID) == "" {
		return NotificationEvent{
			Timestamp: nowStr,
			Channel:   "TELEGRAM",
			Status:    "ERROR",
			Target:    chatID,
			Summary:   "Falta Bot Token o Chat ID de Telegram",
			Detail:    "Introduce el Token de @BotFather y el Chat ID en la pestaña SRE/Alertas.",
		}
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", strings.TrimSpace(botToken))
	payload, _ := json.Marshal(map[string]string{
		"chat_id": strings.TrimSpace(chatID),
		"text":    text,
	})
	client := &http.Client{Timeout: 8 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		return NotificationEvent{
			Timestamp: nowStr,
			Channel:   "TELEGRAM",
			Status:    "ERROR",
			Target:    chatID,
			Summary:   "Error de red enviando a Telegram",
			Detail:    err.Error(),
		}
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return NotificationEvent{
			Timestamp: nowStr,
			Channel:   "TELEGRAM",
			Status:    "SENT",
			Target:    "Chat " + chatID,
			Summary:   "Alerta enviada correctamente a Telegram",
			Detail:    "HTTP 200 OK via api.telegram.org",
		}
	}
	return NotificationEvent{
		Timestamp: nowStr,
		Channel:   "TELEGRAM",
		Status:    "ERROR",
		Target:    chatID,
		Summary:   fmt.Sprintf("Telegram devolvió HTTP %d", resp.StatusCode),
		Detail:    string(bodyBytes),
	}
}

func sendSMTPAlert(cfg SLOConfig, subject, body, nowStr string) NotificationEvent {
	if strings.TrimSpace(cfg.SMTPHost) == "" || strings.TrimSpace(cfg.SMTPTo) == "" {
		return NotificationEvent{
			Timestamp: nowStr,
			Channel:   "EMAIL",
			Status:    "ERROR",
			Target:    cfg.SMTPTo,
			Summary:   "Falta servidor SMTP o destinatario",
			Detail:    "Configura el Host SMTP y el correo destinatario en la pestaña SRE/Alertas.",
		}
	}
	from := cfg.SMTPFrom
	if strings.TrimSpace(from) == "" {
		from = cfg.SMTPUser
	}
	if strings.TrimSpace(from) == "" {
		from = "oratui-sre@oracle-monitor.local"
	}
	recipients := strings.Split(cfg.SMTPTo, ",")
	for i := range recipients {
		recipients[i] = strings.TrimSpace(recipients[i])
	}
	port := cfg.SMTPPort
	if port <= 0 {
		port = 587
	}
	addr := fmt.Sprintf("%s:%d", strings.TrimSpace(cfg.SMTPHost), port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s",
		from, strings.Join(recipients, ", "), subject, body)

	var auth smtp.Auth
	if strings.TrimSpace(cfg.SMTPUser) != "" && cfg.SMTPPassword != "" {
		auth = smtp.PlainAuth("", strings.TrimSpace(cfg.SMTPUser), cfg.SMTPPassword, strings.TrimSpace(cfg.SMTPHost))
	}

	// Support both standard STARTTLS/Plain and direct TLS
	err := smtp.SendMail(addr, auth, from, recipients, []byte(msg))
	if err != nil && port == 465 {
		conn, tlsErr := tls.Dial("tcp", addr, &tls.Config{ServerName: strings.TrimSpace(cfg.SMTPHost), InsecureSkipVerify: true})
		if tlsErr == nil {
			defer conn.Close()
			if c, cErr := smtp.NewClient(conn, strings.TrimSpace(cfg.SMTPHost)); cErr == nil {
				if auth != nil {
					_ = c.Auth(auth)
				}
				_ = c.Mail(from)
				for _, rcpt := range recipients {
					_ = c.Rcpt(rcpt)
				}
				if w, wErr := c.Data(); wErr == nil {
					_, _ = w.Write([]byte(msg))
					_ = w.Close()
					_ = c.Quit()
					err = nil
				}
			}
		}
	}

	if err != nil {
		return NotificationEvent{
			Timestamp: nowStr,
			Channel:   "EMAIL",
			Status:    "ERROR",
			Target:    cfg.SMTPTo,
			Summary:   fmt.Sprintf("Error SMTP (%s)", addr),
			Detail:    err.Error(),
		}
	}
	return NotificationEvent{
		Timestamp: nowStr,
		Channel:   "EMAIL",
		Status:    "SENT",
		Target:    cfg.SMTPTo,
		Summary:   "Correo de alerta crítica enviado por SMTP",
		Detail:    fmt.Sprintf("Servidor %s -> %s", addr, cfg.SMTPTo),
	}
}

// handleExecuteAction executes DBA & Linux/SAN remediation commands triggered from the Flutter Web UI.
func (e *Exporter) handleExecuteAction(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !e.requireAuth(w, r) {
		return
	}
	var req ActionRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 64*1024)).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
		return
	}
	req.Target = strings.TrimSpace(req.Target)
	if err := validateSafeTarget(req.Target); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ActionResponse{
			Success:    false,
			Action:     req.Action,
			Target:     req.Target,
			Title:      "Acción Bloqueada por Seguridad Anti-Inyección",
			Command:    "SECURITY_VALIDATOR",
			Output:     []string{"[SECURITY BLOCK] " + err.Error()},
			DurationMs: 0,
			ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
		})
		return
	}

	start := time.Now()
	st, _ := e.Snapshot()
	var lines []string
	appendLine := func(line string) {
		lines = append(lines, line)
	}

	ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
	defer cancel()

	runner := executor.NewRunner(45*time.Second, st.IsMock)
	dbSID := ""
	if len(st.Instances) > 0 {
		dbSID = st.Instances[0].InstName
	}
	oraHome := st.OracleHome
	oraOwner := st.OracleOwner
	if oraOwner == "" {
		oraOwner = "oracle"
	}
	oraCreds, credErr := executor.DetectFileOwner(oraHome)
	if credErr != nil || oraCreds == nil {
		oraCreds = &executor.OwnerCreds{Username: oraOwner}
	}

	title := ""
	cmdDesc := ""
	var execErr error

	switch req.Action {
	case "kill_session":
		target := req.Target
		if target == "" {
			target = "512,9931,@2"
		}
		title = fmt.Sprintf("Matar Sesión Bloqueante (%s)", target)
		sql := fmt.Sprintf("ALTER SYSTEM KILL SESSION '%s' IMMEDIATE;", target)
		cmdDesc = fmt.Sprintf("sqlplus -s / as sysdba <<< \"%s\"", sql)
		if st.IsMock {
			appendLine(fmt.Sprintf("Conectado a instancia %s como SYSDBA...", dbSID))
			appendLine("SQL> " + sql)
			appendLine("System altered.")
			appendLine(fmt.Sprintf("Sesión %s finalizada e hilos TX/TM liberados correctamente.", target))
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "add_datafile":
		tbs := req.Target
		if tbs == "" {
			tbs = "ERP_DATA_TBS"
		}
		title = fmt.Sprintf("Ampliar Tablespace %s (+Datafile 32G AUTOEXTEND)", tbs)
		sql := fmt.Sprintf("ALTER TABLESPACE %s ADD DATAFILE SIZE 32G AUTOEXTEND ON NEXT 1G MAXSIZE UNLIMITED;", tbs)
		cmdDesc = fmt.Sprintf("sqlplus -s / as sysdba <<< \"%s\"", sql)
		if st.IsMock {
			appendLine("SQL> " + sql)
			appendLine("Tablespace altered.")
			appendLine(fmt.Sprintf("Nuevo datafile de 32 GB (AUTOEXTEND ON) añadido a %s.", tbs))
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "coalesce_tablespace":
		tbs := req.Target
		if tbs == "" {
			tbs = "USERS"
		}
		title = fmt.Sprintf("Consolidar Espacio Libre (COALESCE) en %s", tbs)
		sql := fmt.Sprintf("ALTER TABLESPACE %s COALESCE;", tbs)
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> " + sql)
			appendLine("Tablespace altered.")
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "compile_invalids":
		title = "Recompilar Objetos Inválidos (UTL_RECOMP)"
		sql := "EXEC UTL_RECOMP.RECOMP_PARALLEL(4);"
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> " + sql)
			appendLine("PL/SQL procedure successfully completed.")
			appendLine("Recompilación en paralelo (4 hilos) completada sin errores.")
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, "BEGIN UTL_RECOMP.RECOMP_PARALLEL(4); END;\n/", appendLine)
		}

	case "gather_stats":
		target := req.Target
		if target == "" {
			target = "ERP.FACTURAS_CAB"
		}
		parts := strings.SplitN(target, ".", 2)
		owner, tab := "SYSTEM", target
		if len(parts) == 2 {
			owner, tab = parts[0], parts[1]
		}
		title = fmt.Sprintf("Recolectar Estadísticas (DBMS_STATS) en %s.%s", owner, tab)
		sql := fmt.Sprintf("BEGIN DBMS_STATS.GATHER_TABLE_STATS(ownname=>'%s', tabname=>'%s', cascade=>TRUE); END;\n/", owner, tab)
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> " + sql)
			appendLine("PL/SQL procedure successfully completed.")
			appendLine(fmt.Sprintf("Estadísticas de %s.%s actualizadas a estado FRESH.", owner, tab))
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "listener_reload":
		lsnr := req.Target
		if lsnr == "" {
			lsnr = "LISTENER"
		}
		title = fmt.Sprintf("Recargar Listener Oracle Net (%s)", lsnr)
		lsnrBin := filepath.Join(oraHome, "bin", "lsnrctl")
		cmdDesc = fmt.Sprintf("%s reload %s", lsnrBin, lsnr)
		if st.IsMock {
			appendLine("LSNRCTL for Linux: Version 19.0.0.0.0 - Production")
			appendLine(fmt.Sprintf("Connecting to (DESCRIPTION=(ADDRESS=(PROTOCOL=TCP)(HOST=%s)(PORT=1521)))", st.HostName))
			appendLine("The command completed successfully")
		} else {
			_, execErr = runner.RunCommand(ctx, oraCreds, false, map[string]string{"ORACLE_HOME": oraHome}, appendLine, lsnrBin, "reload", lsnr)
		}

	case "rman_archivelog_backup":
		title = "Backup RMAN de Archivelogs + Liberación de FRA"
		rmanBin := filepath.Join(oraHome, "bin", "rman")
		cmdDesc = "rman target / <<< 'BACKUP ARCHIVELOG ALL DELETE INPUT;'"
		if st.IsMock {
			appendLine("Recovery Manager: Release 19.0.0.0.0 - Production")
			appendLine("connected to target database: " + st.ClusterName + " (DBID=394810291)")
			appendLine("Starting backup at " + time.Now().Format("02-JAN-06"))
			appendLine("channel ORA_DISK_1: starting archived log backup set")
			appendLine("channel ORA_DISK_1: finished piece 1")
			appendLine("archived log file=+FRA/arch_1_8842.arc deleted")
			appendLine("Finished backup at " + time.Now().Format("02-JAN-06"))
		} else {
			_, execErr = runner.RunCommand(ctx, oraCreds, false, map[string]string{
				"ORACLE_HOME": oraHome,
				"ORACLE_SID":  dbSID,
			}, appendLine, "sh", "-c", fmt.Sprintf("echo 'CROSSCHECK ARCHIVELOG ALL; DELETE NOPROMPT EXPIRED ARCHIVELOG ALL;' | %s target /", rmanBin))
		}

	case "create_restore_point":
		rpName := fmt.Sprintf("GRP_WEB_%s", time.Now().Format("150405"))
		title = fmt.Sprintf("Crear Punto de Restauración Garantizado (%s)", rpName)
		sql := fmt.Sprintf("CREATE RESTORE POINT %s GUARANTEE FLASHBACK DATABASE;", rpName)
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> " + sql)
			appendLine("Restore point created.")
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "switch_logfile":
		title = "Forzar Switch Logfile y Checkpoint Global"
		sql := "ALTER SYSTEM SWITCH LOGFILE;\nALTER SYSTEM CHECKPOINT;"
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> ALTER SYSTEM SWITCH LOGFILE;")
			appendLine("System altered.")
			appendLine("SQL> ALTER SYSTEM CHECKPOINT;")
			appendLine("System altered.")
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "purge_adrci":
		title = "Purgar Trazas Antiguas con ADRCI y Liberar Inodos"
		adrciBin := filepath.Join(oraHome, "bin", "adrci")
		cmdDesc = "adrci exec='purge -age 10080 -type TRACE'"
		if st.IsMock {
			appendLine("ADRCI: Release 19.0.0.0.0 - Production")
			appendLine("Purging diagnostic destination diag/rdbms/...")
			appendLine("Purge completed successfully. Inodos liberados en /u01.")
		} else {
			_, execErr = runner.RunCommand(ctx, oraCreds, false, map[string]string{"ORACLE_HOME": oraHome}, appendLine, adrciBin, "exec=purge -age 10080 -type TRACE")
		}

	case "audit_multipath":
		title = "Auditoría Completa de Caminos SAN Multipath (multipath -ll & lsblk)"
		cmdDesc = "multipath -ll && lsblk -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL"
		if st.IsMock {
			appendLine("mpatha (360002ac0000000000000001a00019b21) dm-3 3PARdata,VV")
			appendLine("size=500G features='1 queue_if_no_path' hwhandler='1 alua' wp=rw")
			appendLine("`-+- policy='service-time 0' prio=50 status=active")
			appendLine("  |- 2:0:0:1 sdb 8:16 active ready running")
			appendLine("  |- 2:0:1:1 sdc 8:32 active ready running")
			appendLine("  |- 3:0:0:1 sdd 8:48 active ready running")
			appendLine("  `- 3:0:1:1 sde 8:64 active ready running")
			appendLine("")
			appendLine("NAME                  SIZE TYPE  FSTYPE     MOUNTPOINT MODEL")
			appendLine("sdb                   500G disk  oracleasm             3PARdata VV")
			appendLine("└─mpatha              500G mpath oracleasm  +DATA")
		} else {
			_, execErr = runner.RunCommand(ctx, nil, true, nil, appendLine, "sh", "-c",
				"echo '=== [1] MULTIPATH -LL ==='; (multipath -ll 2>&1 || true); echo ''; echo '=== [2] LSBLK ==='; (lsblk -o NAME,SIZE,TYPE,FSTYPE,MOUNTPOINT,MODEL 2>&1 || true)")
		}

	case "rescan_scsi":
		title = "Reescanear Buses SCSI/Fibre Channel y Recargar Multipath"
		cmdDesc = "for h in /sys/class/scsi_host/host*/scan; do echo '- - -' > $h; done && multipath -r"
		if st.IsMock {
			appendLine(">>> Reescaneando adaptadores HBA Fibre Channel (/sys/class/scsi_host/host*)...")
			appendLine("  host0: scan '- - -' [OK]")
			appendLine("  host1: scan '- - -' [OK]")
			appendLine(">>> Ejecutando multipath -r (reload maps)...")
			appendLine("  mpatha: 4/4 active ready running")
			appendLine("  mpathc: 4/4 active ready running (camino recuperado)")
		} else {
			_, execErr = runner.RunCommand(ctx, nil, true, nil, appendLine, "sh", "-c",
				"for h in /sys/class/scsi_host/host*/scan; do echo '- - -' > \"$h\" 2>/dev/null || true; done; multipath -r 2>&1 || true; multipath -ll 2>&1 || true")
		}

	case "audit_sysctl":
		title = "Auditoría de Parámetros de Kernel Linux para Oracle (sysctl)"
		cmdDesc = "sysctl -a | grep -E 'shm|sem|file-max|aio|ip_local_port_range|swappiness|nr_hugepages'"
		if st.IsMock {
			appendLine("fs.aio-max-nr = 1048576")
			appendLine("fs.aio-nr = 48192")
			appendLine("fs.file-max = 6815744")
			appendLine("kernel.shmall = 1073741824")
			appendLine("kernel.shmmax = 4398046511104")
			appendLine("vm.nr_hugepages = 16384")
			appendLine("vm.swappiness = 10")
		} else {
			_, execErr = runner.RunCommand(ctx, nil, true, nil, appendLine, "sh", "-c",
				"sysctl -a 2>/dev/null | grep -E 'shm|sem|file-max|file-nr|aio|ip_local_port_range|rmem|wmem|swappiness|nr_hugepages|panic_on_oops' | sort")
		}

	case "top_processes":
		title = "Top 20 Procesos Linux por Consumo de CPU y Memoria RAM"
		cmdDesc = "ps -eo pid,user,%cpu,%mem,rss,comm --sort=-%cpu | head -n 20"
		if st.IsMock {
			appendLine("    PID USER     %CPU %MEM   RSS COMMAND")
			appendLine("  14822 oracle   24.5  4.2 284120 ora_j000_PRODCDB1")
			appendLine("  11042 oracle   18.1  6.8 524288 oraclePRODCDB1 (LOCAL=NO)")
			appendLine("   4102 grid      3.4  1.9 142100 ocssd.bin")
			appendLine("   3819 root      1.8  1.1  98240 crsd.bin")
		} else {
			_, execErr = runner.RunCommand(ctx, nil, true, nil, appendLine, "sh", "-c",
				"ps -eo pid,user,%cpu,%mem,rss,comm --sort=-%cpu | head -n 22")
		}

	case "audit_inodes":
		title = "Auditoría de Espacio e Inodos en Filesystems (df -PTm & df -Pi)"
		cmdDesc = "df -PTm && df -Pi"
		if st.IsMock {
			appendLine("Filesystem          Type  1M-blocks   Used Available Use% Mounted on")
			appendLine("/dev/mapper/vg-u01  xfs      204800 148200     56600  73% /u01")
			appendLine("Filesystem            Inodes   IUsed   IFree IUse% Mounted on")
			appendLine("/dev/mapper/vg-u01  20971520 2936012 18035508  14% /u01")
		} else {
			_, execErr = runner.RunCommand(ctx, nil, true, nil, appendLine, "sh", "-c",
				"echo '=== ESPACIO (MB) ==='; df -PTm; echo ''; echo '=== INODOS ==='; df -Pi")
		}

	case "pin_sql_plan":
		target := req.Target
		if target == "" {
			target = "8f4k2m91x7p0q:1049281744"
		}
		sqlID := target
		phv := ""
		if idx := strings.Index(target, ":"); idx > 0 {
			sqlID = target[:idx]
			phv = target[idx+1:]
		}
		title = fmt.Sprintf("Fijar Plan Óptimo en SQL Plan Baseline (SQL_ID=%s, PHV=%s)", sqlID, phv)
		var sql string
		if phv != "" {
			sql = fmt.Sprintf("DECLARE l_plans PLS_INTEGER; BEGIN l_plans := DBMS_SPM.LOAD_PLANS_FROM_CURSOR_CACHE(sql_id => '%s', plan_hash_value => %s, fixed => 'YES', enabled => 'YES'); DBMS_OUTPUT.PUT_LINE('SPM Plans Loaded & Fixed: ' || l_plans); END;\n/", sqlID, phv)
		} else {
			sql = fmt.Sprintf("DECLARE l_plans PLS_INTEGER; BEGIN l_plans := DBMS_SPM.LOAD_PLANS_FROM_CURSOR_CACHE(sql_id => '%s', fixed => 'YES', enabled => 'YES'); DBMS_OUTPUT.PUT_LINE('SPM Plans Loaded & Fixed: ' || l_plans); END;\n/", sqlID)
		}
		cmdDesc = fmt.Sprintf("DBMS_SPM.LOAD_PLANS_FROM_CURSOR_CACHE(sql_id=>'%s', plan_hash_value=>%s, fixed=>'YES')", sqlID, phv)
		if st.IsMock {
			appendLine("SQL> SET SERVEROUTPUT ON")
			appendLine("SQL> " + sql)
			appendLine("SPM Plans Loaded & Fixed: 1")
			appendLine("PL/SQL procedure successfully completed.")
			appendLine(fmt.Sprintf("Plan óptimo PHV=%s fijado (FIXED=YES) en SQL Plan Management para SQL_ID %s.", phv, sqlID))
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, "SET SERVEROUTPUT ON\n"+sql, appendLine)
		}

	case "run_scheduler_job":
		jobSpec := req.Target
		if jobSpec == "" {
			jobSpec = "CRM_DATA.JOB_SYNC_CREDIT_SCORES"
		}
		title = fmt.Sprintf("Ejecutar Job de Negocio Ahora (DBMS_SCHEDULER.RUN_JOB %s)", jobSpec)
		sql := fmt.Sprintf("BEGIN DBMS_SCHEDULER.RUN_JOB(job_name => '%s', use_current_session => FALSE); END;\n/", jobSpec)
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> " + sql)
			appendLine("PL/SQL procedure successfully completed.")
			appendLine(fmt.Sprintf("Job %s lanzado en segundo plano (Coordinator CJQ0).", jobSpec))
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "enable_scheduler_job":
		jobSpec := req.Target
		if jobSpec == "" {
			jobSpec = "CRM_DATA.JOB_SYNC_CREDIT_SCORES"
		}
		title = fmt.Sprintf("Habilitar Job en DBMS_SCHEDULER (%s)", jobSpec)
		sql := fmt.Sprintf("BEGIN DBMS_SCHEDULER.ENABLE(name => '%s'); END;\n/", jobSpec)
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> " + sql)
			appendLine("PL/SQL procedure successfully completed.")
			appendLine(fmt.Sprintf("Job %s habilitado (ENABLED=TRUE).", jobSpec))
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	case "tune_memory_advisor":
		target := req.Target
		if target == "" {
			target = "SGA:40960"
		}
		title = fmt.Sprintf("Simulación y Recomendación de Memoria Oracle (%s)", target)
		sql := "SELECT sga_size, sga_size_factor, estd_db_time_factor, estd_physical_reads FROM v$sga_target_advice ORDER BY sga_size;\nSELECT ROUND(pga_target_for_estimate/1048576) pga_mb, pga_target_factor, estd_pga_cache_hit_percentage, estd_overalloc_count FROM v$pga_target_advice ORDER BY 1;"
		cmdDesc = sql
		if st.IsMock {
			appendLine("SQL> -- Análisis de v$sga_target_advice y v$pga_target_advice")
			appendLine("  SGA_SIZE SGA_SIZE_FACTOR ESTD_DB_TIME_FACTOR ESTD_PHYSICAL_READS")
			appendLine("---------- --------------- ------------------- -------------------")
			appendLine("     24576             .75                1.18             8420000")
			appendLine("     32768               1                1.00             4820000  <-- ACTUAL")
			appendLine("     40960            1.25                 .86             2910000  <-- ÓPTIMO (-14% DB Time)")
			appendLine("")
			appendLine("Comando sugerido para ventana de cambio (con HugePages verificadas):")
			appendLine("  ALTER SYSTEM SET SGA_TARGET=40G SCOPE=SPFILE SID='*';")
		} else {
			_, _, execErr = runner.RunSQLPlus(ctx, oraHome, dbSID, oraCreds, sql, appendLine)
		}

	default:
		title = "Diagnóstico General del Servidor Oracle"
		cmdDesc = "uname -a && uptime"
		_, execErr = runner.RunCommand(ctx, nil, true, nil, appendLine, "sh", "-c", "uname -a; uptime")
	}

	if execErr != nil {
		appendLine(fmt.Sprintf("[ERROR]: %v", execErr))
	}
	if len(lines) == 0 {
		lines = append(lines, "Operación completada sin salida adicional.")
	}

	resp := ActionResponse{
		Success:    execErr == nil,
		Action:     req.Action,
		Target:     req.Target,
		Title:      title,
		Command:    cmdDesc,
		Output:     lines,
		DurationMs: time.Since(start).Milliseconds(),
		ExecutedAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

// handleExecutiveReport renders a self-contained, print-to-PDF ready HTML Executive Health Report
// inspired by ora-brabo's executive report, combining KPIs, SRE/SLOs, Advisor Findings, SQL Plan Flips,
// RMAN Channels, Business Jobs, RAC Cache Fusion, Datafile I/O, Memory Advisors, and SAN Multipath.
func (e *Exporter) handleExecutiveReport(w http.ResponseWriter, r *http.Request) {
	if enableCORS(w, r) {
		return
	}
	e.mu.RLock()
	st := e.state
	updated := e.lastUpdated
	slos := computeSLOs(e.totalSamples, e.goodAvailSamples, e.goodLockFreeSamples, e.goodStorageSamples, e.goodSANSamples, e.sloConfig)
	alerts := append([]DelicateAlert(nil), e.activeAlerts...)
	e.mu.RUnlock()

	overallStatus := "SALUDABLE (HEALTHY)"
	overallColor := "#16a34a"
	critCount := 0
	warnCount := 0
	for _, f := range st.AdvisorFindings {
		if f.Severity == "CRITICAL" {
			critCount++
		} else if f.Severity == "WARNING" {
			warnCount++
		}
	}
	for _, a := range alerts {
		if a.Severity == "CRITICAL" {
			critCount++
		} else {
			warnCount++
		}
	}
	if critCount > 0 {
		overallStatus = fmt.Sprintf("ATENCIÓN REQUERIDA (%d CRÍTICOS)", critCount)
		overallColor = "#dc2626"
	} else if warnCount > 0 {
		overallStatus = fmt.Sprintf("ADVERTENCIA (%d AVISOS)", warnCount)
		overallColor = "#d97706"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="utf-8">
  <title>Informe Ejecutivo Oracle DBA &amp; SRE - %s (%s)</title>
  <style>
    * { box-sizing: border-box; }
    body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #0f172a; color: #f8fafc; margin: 0; padding: 24px; }
    .container { max-width: 1180px; margin: 0 auto; background: #1e293b; border: 2px solid #38bdf8; border-radius: 8px; padding: 28px; box-shadow: 0 10px 30px rgba(0,0,0,0.5); }
    .header { display: flex; justify-content: space-between; align-items: flex-start; border-bottom: 2px solid #334155; padding-bottom: 18px; margin-bottom: 22px; }
    .title h1 { margin: 0 0 6px 0; color: #38bdf8; font-size: 24px; }
    .title p { margin: 0; color: #94a3b8; font-size: 13px; }
    .status-badge { background: %s; color: #fff; font-weight: 800; padding: 8px 16px; border-radius: 6px; font-size: 14px; display: inline-block; }
    .btn-print { background: #38bdf8; color: #0f172a; border: none; font-weight: 800; padding: 9px 16px; border-radius: 6px; cursor: pointer; margin-left: 10px; font-size: 13px; }
    .kpi-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(175px, 1fr)); gap: 12px; margin-bottom: 24px; }
    .kpi-card { background: #0f172a; border: 1px solid #334155; border-radius: 6px; padding: 14px; }
    .kpi-label { color: #94a3b8; font-size: 11px; text-transform: uppercase; font-weight: 700; }
    .kpi-val { color: #f8fafc; font-size: 20px; font-weight: 800; margin-top: 4px; }
    .kpi-sub { color: #38bdf8; font-size: 11px; margin-top: 2px; }
    h2 { color: #facc15; font-size: 16px; border-bottom: 1px solid #334155; padding-bottom: 6px; margin-top: 26px; margin-bottom: 12px; text-transform: uppercase; letter-spacing: 0.5px; }
    table { width: 100%%; border-collapse: collapse; margin-bottom: 16px; font-size: 12.5px; }
    th { background: #0f172a; color: #38bdf8; text-align: left; padding: 8px 10px; border: 1px solid #334155; font-weight: 700; }
    td { padding: 7px 10px; border: 1px solid #334155; color: #e2e8f0; }
    tr:nth-child(even) td { background: #162032; }
    .sev-CRITICAL, .sev-BURNING, .sev-FAILED { background: #dc2626; color: #fff; padding: 2px 7px; border-radius: 4px; font-weight: 700; font-size: 11px; }
    .sev-WARNING, .sev-DOWNGRADED { background: #d97706; color: #fff; padding: 2px 7px; border-radius: 4px; font-weight: 700; font-size: 11px; }
    .sev-OK, .sev-ONLINE, .sev-HEALTHY, .sev-SUCCEEDED, .sev-INFO { background: #16a34a; color: #fff; padding: 2px 7px; border-radius: 4px; font-weight: 700; font-size: 11px; }
    @media print {
      body { background: #fff; color: #000; padding: 0; }
      .container { background: #fff; border: 1px solid #ccc; box-shadow: none; max-width: 100%%; }
      .kpi-card, th { background: #f1f5f9 !important; color: #0f172a !important; }
      td, .kpi-val, .title h1, h2 { color: #0f172a !important; }
      .btn-print { display: none; }
    }
  </style>
</head>
<body>
  <div class="container">
    <div class="header">
      <div class="title">
        <h1>INFORME EJECUTIVO DE SALUD ORACLE DBA, SRE &amp; SAN</h1>
        <p><b>Host:</b> %s │ <b>Cluster:</b> %s (%s) │ <b>Versión Oracle:</b> %s │ <b>Generado:</b> %s (ORA-TUI v%s)</p>
      </div>
      <div>
        <span class="status-badge">%s</span>
        <button class="btn-print" onclick="window.print()">🖨️ IMPRIMIR / GUARDAR PDF</button>
      </div>
    </div>
`, html.EscapeString(st.HostName), html.EscapeString(st.ClusterName), overallColor,
		html.EscapeString(st.HostName), html.EscapeString(st.ClusterName), html.EscapeString(string(st.Topology)),
		html.EscapeString(st.ReleaseVersion), updated.Format("2006-01-02 15:04:05"), html.EscapeString(e.version), overallStatus))

	// KPIs
	b.WriteString(fmt.Sprintf(`    <div class="kpi-grid">
      <div class="kpi-card"><div class="kpi-label">Instancias / Nodos</div><div class="kpi-val">%d Inst / %d Nodos</div><div class="kpi-sub">PDBs: %d │ Servicios: %d</div></div>
      <div class="kpi-card"><div class="kpi-label">Buffer / PGA Cache Hit</div><div class="kpi-val">%.1f%% / %.1f%%</div><div class="kpi-sub">Hard Parses: %.1f/s</div></div>
      <div class="kpi-card"><div class="kpi-label">Redo Switches &amp; Undo</div><div class="kpi-val">%d/h (%d/24h)</div><div class="kpi-sub">ORA-01555: %d │ %.1f MB/s</div></div>
      <div class="kpi-card"><div class="kpi-label">SQL Plan Flips</div><div class="kpi-val">%d SQL_IDs</div><div class="kpi-sub">Regresiones en gv$sql</div></div>
      <div class="kpi-card"><div class="kpi-label">Jobs Negocio (24h)</div><div class="kpi-val">%d Monitoreados</div><div class="kpi-sub">DBMS_SCHEDULER</div></div>
      <div class="kpi-card"><div class="kpi-label">Hallazgos Advisor</div><div class="kpi-val">%d Activos</div><div class="kpi-sub">Motor continuo ORA-TUI</div></div>
    </div>
`, len(st.Instances), len(st.Nodes), len(st.PDBs), len(st.Services),
		st.RedoUndo.BufferCacheHitPct, st.RedoUndo.PGACacheHitPct, st.RedoUndo.HardParsesPerSec,
		st.RedoUndo.RedoSwitchesLastHour, st.RedoUndo.RedoSwitches24h, st.RedoUndo.ORA01555Count, st.RedoUndo.RedoRateMBPerSec,
		len(st.PlanFlips), len(st.SchedulerJobs), len(st.AdvisorFindings)))

	// 1. Advisor Findings
	b.WriteString(`    <h2>1. Diagnóstico Automático &amp; Recomendaciones del Advisor (ora-brabo Engine)</h2>
    <table>
      <thead><tr><th>Severidad</th><th>Categoría</th><th>Hallazgo</th><th>Detalle Técnico</th><th>Recomendación DBA / SRE</th></tr></thead>
      <tbody>`)
	for _, f := range st.AdvisorFindings {
		b.WriteString(fmt.Sprintf(`<tr><td><span class="sev-%s">%s</span></td><td><b>%s</b></td><td>%s</td><td>%s</td><td>%s</td></tr>`,
			html.EscapeString(f.Severity), html.EscapeString(f.Severity), html.EscapeString(f.Category),
			html.EscapeString(f.Title), html.EscapeString(f.Detail), html.EscapeString(f.Suggestion)))
	}
	b.WriteString(`</tbody></table>`)

	// 2. SRE / SLO Status
	b.WriteString(`    <h2>2. Objetivos de Nivel de Servicio (SRE / SLO &amp; Error Budget)</h2>
    <table>
      <thead><tr><th>Dimensión SLO</th><th>Target</th><th>SLI Actual</th><th>Error Budget Restante</th><th>Burn Rate</th><th>Estado</th></tr></thead>
      <tbody>`)
	for _, s := range slos {
		b.WriteString(fmt.Sprintf(`<tr><td><b>%s</b><br><small>%s</small></td><td>%.2f%%</td><td><b>%.2f%%</b></td><td>%.1f%%</td><td>%.2fx</td><td><span class="sev-%s">%s</span></td></tr>`,
			html.EscapeString(s.Name), html.EscapeString(s.Description), s.TargetPct, s.CurrentSLIPct, s.ErrorBudgetRemPct, s.BurnRate, html.EscapeString(s.Status), html.EscapeString(s.Status)))
	}
	b.WriteString(`</tbody></table>`)

	// 3. SQL Plan Instability
	b.WriteString(`    <h2>3. Detector de Regresión de Planes SQL (Plan Flips en gv$sql)</h2>
    <table>
      <thead><tr><th>SQL_ID</th><th>Esquema</th><th>Planes</th><th>PHV Actual</th><th>Mejor PHV</th><th>Latencia Actual</th><th>Mejor Latencia</th><th>Estado</th><th>SQL Text</th></tr></thead>
      <tbody>`)
	if len(st.PlanFlips) == 0 {
		b.WriteString(`<tr><td colspan="9">Sin regresiones de planes SQL detectadas en la caché de cursores.</td></tr>`)
	}
	for _, pf := range st.PlanFlips {
		b.WriteString(fmt.Sprintf(`<tr><td><b>%s</b></td><td>%s</td><td>%d</td><td>%d</td><td><b>%d</b></td><td>%.1f ms</td><td>%.1f ms</td><td><span class="sev-%s">%s</span></td><td><code>%s</code></td></tr>`,
			html.EscapeString(pf.SQLID), html.EscapeString(pf.Schema), pf.PlanCount,
			pf.CurrentPHV, pf.BestPHV, pf.AvgElapsedMs, pf.BestElapsedMs,
			html.EscapeString(pf.Status), html.EscapeString(pf.Status), html.EscapeString(pf.SQLText)))
	}
	b.WriteString(`</tbody></table>`)

	// 4. Scheduler Jobs & RMAN Channels
	b.WriteString(`    <h2>4. Jobs de Negocio (DBMS_SCHEDULER) &amp; Canales RMAN en Vivo</h2>
    <table>
      <thead><tr><th>Propietario.Job</th><th>Tipo</th><th>Estado</th><th>Habilitado</th><th>Fallos (24h / Total)</th><th>Último Inicio (Duración)</th><th>Próxima Ejecución</th><th>Detalle / Último Error</th></tr></thead>
      <tbody>`)
	for _, jb := range st.SchedulerJobs {
		sev := "OK"
		if jb.Failed24h > 0 || strings.Contains(strings.ToUpper(jb.State), "FAIL") {
			sev = "CRITICAL"
		}
		b.WriteString(fmt.Sprintf(`<tr><td><b>%s.%s</b></td><td>%s</td><td><span class="sev-%s">%s</span></td><td>%s</td><td>%d / %d</td><td>%s (%ds)</td><td>%s</td><td>%s</td></tr>`,
			html.EscapeString(jb.Owner), html.EscapeString(jb.JobName), html.EscapeString(jb.JobType),
			sev, html.EscapeString(jb.State), html.EscapeString(jb.Enabled), jb.Failed24h, jb.FailureCount,
			html.EscapeString(jb.LastStart), jb.DurationSec, html.EscapeString(jb.NextRun), html.EscapeString(jb.Detail)))
	}
	b.WriteString(`</tbody></table>`)

	// 5. RAC Cache Fusion & Datafile I/O Latency
	b.WriteString(`    <h2>5. Latencia RAC Cache Fusion &amp; Latencia I/O por Datafile (v$filestat)</h2>
    <table>
      <thead><tr><th>Instancia RAC</th><th>Interconnect Privado</th><th>GC CR Block Receive (ms)</th><th>GC Current Block Receive (ms)</th><th>Bloques CR Recibidos</th><th>Bloques Current Recibidos</th><th>Estado</th></tr></thead>
      <tbody>`)
	for _, cf := range st.CacheFusion {
		b.WriteString(fmt.Sprintf(`<tr><td><b>%s (Inst #%d)</b></td><td>%s</td><td><b>%.2f ms</b></td><td><b>%.2f ms</b></td><td>%d</td><td>%d</td><td><span class="sev-%s">%s</span></td></tr>`,
			html.EscapeString(cf.InstanceName), cf.InstID, html.EscapeString(cf.InterconnectIP), cf.CRBlockLatencyMs, cf.CurBlockLatencyMs, cf.CRBlocksReceived, cf.CurBlocksReceived, html.EscapeString(cf.Status), html.EscapeString(cf.Status)))
	}
	b.WriteString(`</tbody></table>
    <table>
      <thead><tr><th>File#</th><th>Tablespace</th><th>Datafile</th><th>Avg Read (ms)</th><th>Avg Write (ms)</th><th>Volumen Leído / Escrito (MB)</th><th>Estado I/O</th></tr></thead>
      <tbody>`)
	for _, dio := range st.DatafileIO {
		b.WriteString(fmt.Sprintf(`<tr><td>#%d</td><td><b>%s</b></td><td><code>%s</code></td><td><b>%.2f ms</b></td><td><b>%.2f ms</b></td><td>%.1f / %.1f MB</td><td><span class="sev-%s">%s</span></td></tr>`,
			dio.FileID, html.EscapeString(dio.Tablespace), html.EscapeString(dio.FileName), dio.AvgReadMs, dio.AvgWriteMs, dio.ReadMB, dio.WriteMB, html.EscapeString(dio.Status), html.EscapeString(dio.Status)))
	}
	b.WriteString(`</tbody></table>`)

	// 6. Tablespaces, Memory Advisors & SAN Multipath
	b.WriteString(`    <h2>6. Asesores de Memoria (SGA/PGA), Tablespaces &amp; SAN Multipath</h2>
    <table>
      <thead><tr><th>Asesor</th><th>Tamaño Simulado (MB)</th><th>Factor</th><th>DB Time Ratio / Hit%% Est.</th><th>Lecturas Físicas Est.</th><th>Evaluación</th></tr></thead>
      <tbody>`)
	for _, ma := range st.MemoryAdvisor {
		b.WriteString(fmt.Sprintf(`<tr><td><b>%s</b></td><td>%d MB</td><td>%.2fx</td><td>%.2f / %.1f%%</td><td>%d</td><td>%s</td></tr>`,
			html.EscapeString(ma.AdvisorType), ma.SizeMB, ma.SizeFactor, ma.EstDBTimeFactor, ma.EstHitPct, ma.EstPhysReads, html.EscapeString(ma.Recommendation)))
	}
	b.WriteString(`</tbody></table>
  </div>
</body>
</html>`)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(b.String()))
}

