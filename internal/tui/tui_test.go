package tui

import (
	"context"
	"iter"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"google.golang.org/genai"

	"vertex-tui/internal/agent"
	"vertex-tui/internal/config"
)

type fakeVertexBackend struct {
	usedModels []string
	listed     []config.ModelInfo
}

func (f *fakeVertexBackend) GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	f.usedModels = append(f.usedModels, model)
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		resp := &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: genai.RoleModel,
						Parts: []*genai.Part{
							{Text: "Hello from " + model},
						},
					},
				},
			},
		}
		yield(resp, nil)
	}
}

func (f *fakeVertexBackend) ListModels(ctx context.Context) ([]config.ModelInfo, error) {
	return f.listed, nil
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Project:        "test-vertex-project",
		Location:       "us-central1",
		Model:          "gemini-2.5-pro",
		WorkDir:        tmpDir,
		ConfigFilePath: filepath.Join(tmpDir, "config.json"),
	}
	ag := agent.New(context.Background(), cfg, nil)
	return NewApp(context.Background(), ag)
}

func typeString(app *App, s string) {
	for _, r := range s {
		if r == ' ' {
			app.Update(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		} else {
			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
	}
}

func pressKey(app *App, kt tea.KeyType) {
	app.Update(tea.KeyMsg{Type: kt})
}

func TestSlashModelInteractiveModalSelection(t *testing.T) {
	app := newTestApp(t)

	if app.GetActiveModel() != "gemini-2.5-pro" {
		t.Fatalf("expected initial model gemini-2.5-pro, got %s", app.GetActiveModel())
	}

	// 1. Type "/model" and press Enter -> opens interactive model selector modal
	typeString(app, "/model")
	pressKey(app, tea.KeyEnter)

	if !app.IsModelModalOpen() {
		t.Fatalf("expected /model with no args to open the interactive model modal")
	}

	view := app.View()
	if !strings.Contains(view, "モデル選択 (/model)") || !strings.Contains(view, "gemini-2.5-flash") {
		t.Fatalf("expected modal view to display available Vertex AI models, got:\n%s", view)
	}

	// 2. Press Down arrow to select "gemini-2.5-flash" and press Enter
	pressKey(app, tea.KeyDown)
	pressKey(app, tea.KeyEnter)

	if app.IsModelModalOpen() {
		t.Fatalf("expected modal to close after pressing Enter")
	}
	if app.GetActiveModel() != "gemini-2.5-flash" {
		t.Fatalf("expected active model to change to gemini-2.5-flash, got %s", app.GetActiveModel())
	}

	// Verify header/footer and system confirmation message reflect the new model
	afterView := app.View()
	if !strings.Contains(afterView, "Model: gemini-2.5-flash") {
		t.Fatalf("expected header badge to show Model: gemini-2.5-flash, got:\n%s", afterView)
	}
}

func TestSlashModelDirectCommandAndAliases(t *testing.T) {
	app := newTestApp(t)

	// 1. Direct model switch: `/model gemini-2.5-flash`
	typeString(app, "/model gemini-2.5-flash")
	pressKey(app, tea.KeyEnter)
	if app.GetActiveModel() != "gemini-2.5-flash" {
		t.Fatalf("expected active model gemini-2.5-flash, got %s", app.GetActiveModel())
	}

	// 2. Alias model switch: `/model flash-lite`
	typeString(app, "/model flash-lite")
	pressKey(app, tea.KeyEnter)
	if app.GetActiveModel() != "gemini-2.5-flash-lite" {
		t.Fatalf("expected active model gemini-2.5-flash-lite, got %s", app.GetActiveModel())
	}

	// 3. Numeric index switch: `/model 1` (1 = gemini-2.5-pro)
	typeString(app, "/model 1")
	pressKey(app, tea.KeyEnter)
	if app.GetActiveModel() != "gemini-2.5-pro" {
		t.Fatalf("expected active model gemini-2.5-pro from `/model 1`, got %s", app.GetActiveModel())
	}

	// 4. Custom Vertex AI model switch: `/model gemini-exp-custom-01`
	typeString(app, "/model gemini-exp-custom-01")
	pressKey(app, tea.KeyEnter)
	if app.GetActiveModel() != "gemini-exp-custom-01" {
		t.Fatalf("expected custom model gemini-exp-custom-01, got %s", app.GetActiveModel())
	}

	// 5. `/model list` and `/model current`
	typeString(app, "/model list")
	pressKey(app, tea.KeyEnter)

	typeString(app, "/model current")
	pressKey(app, tea.KeyEnter)

	msgs := app.Messages()
	lastMsg := msgs[len(msgs)-1]
	if !strings.Contains(lastMsg.Content, "gemini-exp-custom-01") {
		t.Fatalf("expected /model current to report gemini-exp-custom-01, got %s", lastMsg.Content)
	}
}

func TestSlashModelModalFilterAndCustomInput(t *testing.T) {
	app := newTestApp(t)

	// Open modal with Ctrl+O
	pressKey(app, tea.KeyCtrlO)
	if !app.IsModelModalOpen() {
		t.Fatalf("expected Ctrl+O to open model modal")
	}

	// Type filter "2.0-flash-lite" inside modal and press Enter
	typeString(app, "2.0-flash-lite")
	pressKey(app, tea.KeyEnter)

	if app.IsModelModalOpen() {
		t.Fatalf("expected modal to close after Enter")
	}
	if app.GetActiveModel() != "gemini-2.0-flash-lite" {
		t.Fatalf("expected model gemini-2.0-flash-lite, got %s", app.GetActiveModel())
	}
}

func TestTabAutocompleteForModelCommand(t *testing.T) {
	app := newTestApp(t)

	typeString(app, "/mod")
	pressKey(app, tea.KeyTab)
	if string(app.inputRunes) != "/model " {
		t.Fatalf("expected Tab on '/mod' to complete to '/model ', got %q", string(app.inputRunes))
	}

	typeString(app, "2.5-fl")
	pressKey(app, tea.KeyTab)
	if string(app.inputRunes) != "/model gemini-2.5-flash" {
		t.Fatalf("expected Tab on '/model 2.5-fl' to complete to '/model gemini-2.5-flash', got %q", string(app.inputRunes))
	}
}

func TestEndToEndChatAndModelSwitch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Project:        "test-vertex-project",
		Location:       "us-central1",
		Model:          "gemini-2.5-pro",
		WorkDir:        tmpDir,
		ConfigFilePath: filepath.Join(tmpDir, "config.json"),
	}
	fb := &fakeVertexBackend{
		listed: []config.ModelInfo{
			{ID: "gemini-exp-vertex-001", DisplayName: "Gemini Exp Vertex 001"},
		},
	}
	ag := agent.New(context.Background(), cfg, fb)
	app := NewApp(context.Background(), ag)

	// 1. Send a chat message with default model (gemini-2.5-pro)
	typeString(app, "Write a hello world in Go")
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		_, cmd = app.Update(msg)
	}

	if len(fb.usedModels) != 1 || fb.usedModels[0] != "gemini-2.5-pro" {
		t.Fatalf("expected first turn to use gemini-2.5-pro, got %v", fb.usedModels)
	}

	// 2. Refresh models via `/model refresh`
	typeString(app, "/model refresh")
	_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd != nil {
		msg := cmd()
		app.Update(msg)
	}

	// 3. Switch to dynamically discovered model via `/model gemini-exp-vertex-001`
	typeString(app, "/model gemini-exp-vertex-001")
	pressKey(app, tea.KeyEnter)
	if app.GetActiveModel() != "gemini-exp-vertex-001" {
		t.Fatalf("expected active model gemini-exp-vertex-001, got %s", app.GetActiveModel())
	}

	// 4. Send a second chat message and verify the new model is used
	typeString(app, "Now add tests")
	_, cmd = app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		_, cmd = app.Update(msg)
	}

	if len(fb.usedModels) != 2 || fb.usedModels[1] != "gemini-exp-vertex-001" {
		t.Fatalf("expected second turn to use gemini-exp-vertex-001, got %v", fb.usedModels)
	}
}

