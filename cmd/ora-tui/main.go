package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mario/ora-tui/internal/collector"
	"github.com/mario/ora-tui/internal/executor"
	"github.com/mario/ora-tui/internal/model"
	"github.com/mario/ora-tui/internal/telemetry"
	"github.com/mario/ora-tui/internal/ui"
)

const AppVersion = "1.6.0"

func main() {
	mockFlag := flag.Bool("mock", false, "Force interactive mock/simulation mode with sample Oracle data")
	topoFlag := flag.String("topology", "rac", "Simulated topology in mock mode: rac, restart, or single")
	previewFlag := flag.Bool("preview", false, "Print static Unicode box-drawing mockup to stdout and exit")
	versionFlag := flag.Bool("version", false, "Print ora-tui version and exit")
	intervalFlag := flag.Duration("interval", 10*time.Second, "Background collector refresh interval")

	// OpenTelemetry & Prometheus Exporter flags (supporting both --opentelemetry and --opentlemetry / --otel)
	otelFlag := flag.Bool("opentelemetry", false, "Enable OpenTelemetry (OTLP JSON) & Prometheus (/metrics) HTTP exporter")
	otelTypoFlag := flag.Bool("opentlemetry", false, "Alias for --opentelemetry")
	otelShortFlag := flag.Bool("otel", false, "Shorthand alias for --opentelemetry")
	otelPortFlag := flag.Int("otel-port", telemetry.DefaultOTelPort, "TCP port for OpenTelemetry/Prometheus HTTP server (default 9161, free from Oracle ports)")
	portAliasFlag := flag.Int("port", 0, "Alias for --otel-port")
	otelAddrFlag := flag.String("otel-addr", "", "Bind address for OpenTelemetry/Prometheus server (e.g. 0.0.0.0:9161 or 127.0.0.1:9161)")
	daemonFlag := flag.Bool("daemon", false, "Run in headless background daemon mode (no TUI) for systemd / Prometheus / OpenTelemetry")

	flag.Parse()

	if *versionFlag {
		fmt.Printf("ora-tui v%s (MS-DOS Retro Themes + Server/SAN Health Checks + OpenTelemetry/Prometheus Exporter)\n", AppVersion)
		return
	}

	if os.Getenv("COLORTERM") == "" {
		_ = os.Setenv("COLORTERM", "truecolor")
	}

	topo := model.ModeRAC
	switch strings.ToLower(*topoFlag) {
	case "single":
		topo = model.ModeSingle
	case "restart":
		topo = model.ModeRestart
	default:
		topo = model.ModeRAC
	}

	var state *model.EnvironmentState
	runner := executor.NewRunner(60*time.Second, false)

	// Auto-detect whether a real Oracle environment exists on this machine
	hasOracle := false
	if _, err := os.Stat("/etc/oratab"); err == nil {
		hasOracle = true
	} else if _, err := os.Stat("/etc/oracle/ocr.loc"); err == nil {
		hasOracle = true
	}

	if *mockFlag || !hasOracle {
		state = collector.NewMockState(topo)
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		liveState, _, err := collector.CollectLiveEnvironment(ctx, runner)
		cancel()
		if err != nil || liveState == nil {
			state = collector.NewMockState(topo)
		} else {
			state = liveState
		}
	}

	if *previewFlag {
		fmt.Print(ui.RenderStaticASCIIMock(state))
		return
	}

	otelEnabled := *otelFlag || *otelTypoFlag || *otelShortFlag || *daemonFlag
	port := *otelPortFlag
	if *portAliasFlag > 0 {
		port = *portAliasFlag
	}
	listenAddr := *otelAddrFlag
	if listenAddr == "" {
		listenAddr = fmt.Sprintf("0.0.0.0:%d", port)
	}

	var otelExp *telemetry.Exporter
	if otelEnabled {
		otelExp = telemetry.NewExporter(*state, listenAddr, AppVersion)
		if _, err := otelExp.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "[ERROR] Failed to start OpenTelemetry/Prometheus server on %s: %v\n", listenAddr, err)
			os.Exit(1)
		}
	}

	// Detect if running without a TTY (e.g. inside systemd or nohup background job)
	isTTY := false
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) != 0 {
		isTTY = true
	}
	runHeadless := *daemonFlag || (otelEnabled && !isTTY)

	if runHeadless {
		fmt.Printf("[ORA-TUI v%s] Daemon OpenTelemetry & Prometheus iniciado en %s (Host=%s, Modo=%s, Intervalo=%s)\n",
			AppVersion, listenAddr, state.HostName, state.Topology, intervalFlag.String())
		fmt.Printf("[ORA-TUI v%s] Endpoints activos:\n", AppVersion)
		fmt.Printf("  • Prometheus / OpenMetrics : http://%s/metrics\n", listenAddr)
		fmt.Printf("  • OpenTelemetry OTLP JSON  : http://%s/otel/v1/metrics\n", listenAddr)
		fmt.Printf("  • Snapshot Estado JSON     : http://%s/api/v1/state\n", listenAddr)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		interval := *intervalFlag
		if interval <= 0 {
			interval = 15 * time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case sig := <-sigCh:
				fmt.Printf("[ORA-TUI v%s] Señal %v recibida, deteniendo daemon OpenTelemetry...\n", AppVersion, sig)
				return
			case <-ticker.C:
				if !state.IsMock {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					updated, _, err := collector.CollectLiveEnvironment(ctx, runner)
					cancel()
					if err == nil && updated != nil {
						state = updated
					}
				}
				if otelExp != nil {
					otelExp.UpdateState(*state)
				}
			}
		}
	}

	appUI := ui.NewAppUI(state)
	if otelEnabled {
		appUI.Logf("[OTEL] Exportador OpenTelemetry & Prometheus activo en [green]http://%s/metrics[-] y [green]/otel/v1/metrics[-]", listenAddr)
	}

	// Ensure Ctrl+C / SIGINT / SIGTERM always stops the TUI cleanly
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		appUI.App.Stop()
		os.Exit(0)
	}()

	// Start non-blocking background refresh ticker when connected to a live Oracle host
	if *intervalFlag > 0 {
		go func() {
			ticker := time.NewTicker(*intervalFlag)
			defer ticker.Stop()
			for range ticker.C {
				if !state.IsMock {
					ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
					updated, _, err := collector.CollectLiveEnvironment(ctx, runner)
					cancel()
					if err == nil && updated != nil {
						if otelExp != nil {
							otelExp.UpdateState(*updated)
						}
						appUI.App.QueueUpdateDraw(func() {
							updated.BorderMode = appUI.State.BorderMode
							appUI.State = updated
							appUI.RefreshAll()
						})
					}
				} else if otelExp != nil {
					otelExp.UpdateState(*appUI.State)
				}
			}
		}()
	}

	if err := appUI.App.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running ora-tui: %v\n", err)
		os.Exit(1)
	}
}
