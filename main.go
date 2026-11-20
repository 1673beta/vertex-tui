package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"vertex-tui/internal/agent"
	"vertex-tui/internal/config"
	"vertex-tui/internal/tui"
)

func main() {
	cfg := config.DefaultConfig()

	var (
		projectFlag   = flag.String("project", cfg.Project, "Google Cloud Project ID for Vertex AI (or set GOOGLE_CLOUD_PROJECT)")
		locationFlag  = flag.String("location", cfg.Location, "Google Cloud Vertex AI Location/Region (or set GOOGLE_CLOUD_LOCATION)")
		modelFlag     = flag.String("model", cfg.Model, "Initial Vertex AI model ID (can be changed anytime with /model)")
		workdirFlag   = flag.String("workdir", cfg.WorkDir, "Workspace directory for coding agent tools")
		listModels    = flag.Bool("list-models", false, "Fetch and print available Vertex AI Model Garden models and exit")
		publisherFlag = flag.String("publisher", "", "Optional Model Garden publisher filter for -list-models (e.g. google,anthropic,meta,mistralai,deepseek-ai)")
		allModelsFlag = flag.Bool("all-models", false, "Include non-LLM models when listing from Vertex AI Model Garden")
	)
	flag.Parse()

	if *projectFlag != "" {
		cfg.Project = *projectFlag
	}
	if *locationFlag != "" {
		cfg.Location = *locationFlag
	}
	if *modelFlag != "" {
		cfg.Model = config.NormalizeModelID(*modelFlag, config.FallbackModels())
	}
	if *workdirFlag != "" {
		cfg.WorkDir = *workdirFlag
	}

	ctx := context.Background()
	ag := agent.New(ctx, cfg, nil)

	if *listModels {
		fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
		defer cancel()

		opts := agent.ModelGardenListOptions{
			IncludeAll: *allModelsFlag,
		}
		if *publisherFlag != "" {
			opts.Publishers = strings.Split(*publisherFlag, ",")
		}

		models, err := ag.RefreshModelGarden(fetchCtx, opts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Note: Could not fetch live models from Vertex AI Model Garden (%v); showing fallback catalog:\n", err)
		} else {
			fmt.Println("Synced live model catalog from Vertex AI Model Garden:")
		}
		for _, m := range models {
			marker := "  "
			if strings.EqualFold(m.ID, cfg.Model) {
				marker = "* "
			}
			stage := ""
			if m.LaunchStage != "" {
				stage = fmt.Sprintf("[%s] ", m.LaunchStage)
			}
			fmt.Printf("%s%-38s %-14s %s%s\n", marker, m.ID, "["+m.Publisher+"]", stage, m.Description)
		}
		return
	}

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
