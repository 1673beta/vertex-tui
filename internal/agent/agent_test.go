package agent

import (
	"context"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/genai"

	"vertex-tui/internal/config"
)

type mockBackend struct {
	usedModels []string
	responses  [][]*genai.GenerateContentResponse
	callIndex  int
	listed     []config.ModelInfo
}

func (m *mockBackend) GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	m.usedModels = append(m.usedModels, model)
	idx := m.callIndex
	m.callIndex++
	var chunks []*genai.GenerateContentResponse
	if idx < len(m.responses) {
		chunks = m.responses[idx]
	}
	return func(yield func(*genai.GenerateContentResponse, error) bool) {
		for _, c := range chunks {
			if !yield(c, nil) {
				return
			}
		}
	}
}

func (m *mockBackend) ListModels(ctx context.Context) ([]config.ModelInfo, error) {
	return m.listed, nil
}

func TestAgentSetModelAndPreservesHistory(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.Config{
		Project:        "test-proj",
		Location:       "us-central1",
		Model:          "gemini-2.5-pro",
		WorkDir:        tmpDir,
		ConfigFilePath: filepath.Join(tmpDir, "config.json"),
	}

	mb := &mockBackend{
		responses: [][]*genai.GenerateContentResponse{
			{
				{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Role:  genai.RoleModel,
								Parts: []*genai.Part{{Text: "Response from 2.5 Pro"}},
							},
						},
					},
				},
			},
			{
				{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Role:  genai.RoleModel,
								Parts: []*genai.Part{{Text: "Response from 2.5 Flash"}},
							},
						},
					},
				},
			},
		},
	}

	ag := New(context.Background(), cfg, mb)
	if ag.GetModel() != "gemini-2.5-pro" {
		t.Fatalf("expected initial model gemini-2.5-pro, got %s", ag.GetModel())
	}

	// Turn 1 with gemini-2.5-pro
	var events []StreamEvent
	if err := ag.RunTurn(context.Background(), "Hello Pro", func(ev StreamEvent) {
		events = append(events, ev)
	}); err != nil {
		t.Fatalf("RunTurn 1 failed: %v", err)
	}

	// Switch model via SetModel using alias "flash"
	prev, info, known, err := ag.SetModel("flash")
	if err != nil {
		t.Fatalf("SetModel failed: %v", err)
	}
	if prev != "gemini-2.5-pro" || info.ID != "gemini-2.5-flash" || !known {
		t.Fatalf("unexpected SetModel result: prev=%s info=%+v known=%v", prev, info, known)
	}
	if ag.GetModel() != "gemini-2.5-flash" {
		t.Fatalf("expected GetModel() = gemini-2.5-flash, got %s", ag.GetModel())
	}

	// Switch to a custom Vertex AI model and back to gemini-2.5-flash
	_, customInfo, customKnown, err := ag.SetModel("publishers/google/models/gemini-exp-custom")
	if err != nil {
		t.Fatalf("SetModel custom failed: %v", err)
	}
	if customKnown || customInfo.ID != "gemini-exp-custom" {
		t.Fatalf("unexpected custom model result: known=%v info=%+v", customKnown, customInfo)
	}

	_, _, _, _ = ag.SetModel("gemini-2.5-flash")

	// Turn 2 with gemini-2.5-flash
	if err := ag.RunTurn(context.Background(), "Hello Flash", func(ev StreamEvent) {
		events = append(events, ev)
	}); err != nil {
		t.Fatalf("RunTurn 2 failed: %v", err)
	}

	if len(mb.usedModels) != 2 || mb.usedModels[0] != "gemini-2.5-pro" || mb.usedModels[1] != "gemini-2.5-flash" {
		t.Fatalf("expected usedModels [gemini-2.5-pro, gemini-2.5-flash], got %v", mb.usedModels)
	}
	if ag.HistoryCount() != 4 {
		t.Fatalf("expected 4 history turns preserved across model switch, got %d", ag.HistoryCount())
	}
}

func TestAgentToolCallingLoop(t *testing.T) {
	tmpDir := t.TempDir()
	sampleFile := filepath.Join(tmpDir, "main.go")
	if err := os.WriteFile(sampleFile, []byte("package main\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Project:  "test-proj",
		Location: "us-central1",
		Model:    "gemini-2.5-pro",
		WorkDir:  tmpDir,
	}

	mb := &mockBackend{
		responses: [][]*genai.GenerateContentResponse{
			// Step 1: Model requests read_file("main.go")
			{
				{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Role: genai.RoleModel,
								Parts: []*genai.Part{
									{
										FunctionCall: &genai.FunctionCall{
											Name: "read_file",
											Args: map[string]any{"path": "main.go"},
										},
									},
								},
							},
						},
					},
				},
			},
			// Step 2: Model responds with explanation after seeing FunctionResponse
			{
				{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Role: genai.RoleModel,
								Parts: []*genai.Part{
									{Text: "main.go contains package main."},
								},
							},
						},
					},
				},
			},
		},
	}

	ag := New(context.Background(), cfg, mb)
	var sawToolCall, sawToolResult, sawFinalText bool
	err := ag.RunTurn(context.Background(), "Read main.go", func(ev StreamEvent) {
		switch ev.Type {
		case EventToolCall:
			if ev.ToolName == "read_file" {
				sawToolCall = true
			}
		case EventToolResult:
			if strings.Contains(ev.ToolResult, "package main") {
				sawToolResult = true
			}
		case EventTextDelta:
			if strings.Contains(ev.Text, "main.go contains package main.") {
				sawFinalText = true
			}
		}
	})
	if err != nil {
		t.Fatalf("RunTurn with tool call failed: %v", err)
	}
	if !sawToolCall || !sawToolResult || !sawFinalText {
		t.Fatalf("missing expected events: toolCall=%v toolResult=%v finalText=%v", sawToolCall, sawToolResult, sawFinalText)
	}
}
