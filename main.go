package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"vertex-tui/internal/agent"
	"vertex-tui/internal/config"
	"vertex-tui/internal/tui"
)

func main() {
	cfg := config.DefaultConfig()

	var (
		projectFlag  = flag.String("project", cfg.Project, "Google Cloud Project ID for Vertex AI (or set GOOGLE_CLOUD_PROJECT)")
		locationFlag = flag.String("location", cfg.Location, "Google Cloud Vertex AI Location/Region (or set GOOGLE_CLOUD_LOCATION)")
		modelFlag    = flag.String("model", cfg.Model, "Initial Vertex AI model ID (can be changed anytime with /model)")
		workdirFlag  = flag.String("workdir", cfg.WorkDir, "Workspace directory for coding agent tools")
		listModels   = flag.Bool("list-models", false, "Print available Vertex AI models and exit")
	)
	flag.Parse()

	if *projectFlag != "" {
		cfg.Project = *projectFlag
	}
	if *locationFlag != "" {
		cfg.Location = *locationFlag
	}
	if *modelFlag != "" {
		cfg.Model = config.NormalizeModelID(*modelFlag, config.DefaultModels())
	}
	if *workdirFlag != "" {
		cfg.WorkDir = *workdirFlag
	}

	if *listModels {
		fmt.Println("Available Vertex AI (Agent Platform) Models:")
		for _, m := range config.DefaultModels() {
			marker := "  "
			if m.ID == cfg.Model {
				marker = "* "
			}
			fmt.Printf("%s%-24s %-22s %s\n", marker, m.ID, "("+m.DisplayName+")", m.Description)
		}
		return
	}

	ctx := context.Background()
	ag := agent.New(ctx, cfg, nil)
	app := tui.NewApp(ctx, ag)

	p := tea.NewProgram(
		app,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running vertex-tui: %v\n", err)
		os.Exit(1)
	}
}
