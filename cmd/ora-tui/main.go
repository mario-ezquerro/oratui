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
	"github.com/mario/ora-tui/internal/ui"
)

func main() {
	mockFlag := flag.Bool("mock", false, "Force interactive mock/simulation mode with sample Oracle data")
	topoFlag := flag.String("topology", "rac", "Simulated topology in mock mode: rac, restart, or single")
	previewFlag := flag.Bool("preview", false, "Print static Unicode box-drawing mockup to stdout and exit")
	intervalFlag := flag.Duration("interval", 10*time.Second, "Background collector refresh interval")
	flag.Parse()

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

	appUI := ui.NewAppUI(state)

	// Ensure Ctrl+C / SIGINT / SIGTERM always stops the TUI cleanly
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		appUI.App.Stop()
		os.Exit(0)
	}()

	// Start non-blocking background refresh ticker when connected to a live Oracle host
	if !state.IsMock && *intervalFlag > 0 {
		go func() {
			ticker := time.NewTicker(*intervalFlag)
			defer ticker.Stop()
			for range ticker.C {
				ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
				updated, _, err := collector.CollectLiveEnvironment(ctx, runner)
				cancel()
				if err == nil && updated != nil {
					appUI.App.QueueUpdateDraw(func() {
						updated.BorderMode = appUI.State.BorderMode
						appUI.State = updated
						appUI.RefreshAll()
					})
				}
			}
		}()
	}

	if err := appUI.App.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running ora-tui: %v\n", err)
		os.Exit(1)
	}
}
