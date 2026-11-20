package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"strings"
	"sync"

	"google.golang.org/genai"

	"vertex-tui/internal/config"
	"vertex-tui/internal/tools"
)

// EventType identifies the kind of streaming event emitted during an agent turn.
type EventType string

const (
	EventThinkingDelta EventType = "thinking_delta"
	EventTextDelta     EventType = "text_delta"
	EventToolCall      EventType = "tool_call"
	EventToolResult    EventType = "tool_result"
	EventTurnComplete  EventType = "turn_complete"
)

// StreamEvent is emitted by Agent.RunTurn to update the TUI in real time.
type StreamEvent struct {
	Type         EventType
	Text         string
	ToolName     string
	ToolArgs     string
	ToolResult   string
	Model        string
	InputTokens  int32
	OutputTokens int32
}

// Backend abstracts the Google GenAI SDK Vertex AI operations so the agent and TUI
// can operate against live Google Cloud Vertex AI or deterministic test doubles.
type Backend interface {
	GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error]
	ListModels(ctx context.Context) ([]config.ModelInfo, error)
}

// VertexBackend implements Backend using the official Google GenAI SDK (google.golang.org/genai)
// configured with BackendVertexAI.
type VertexBackend struct {
	client *genai.Client
}

// NewVertexBackend creates a Google GenAI SDK client targeting Google Cloud Vertex AI (Agent Platform).
func NewVertexBackend(ctx context.Context, cfg *config.Config) (*VertexBackend, error) {
	cc := &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  cfg.Project,
		Location: cfg.Location,
	}
	if cfg.APIKey != "" && cfg.Project == "" {
		cc.APIKey = cfg.APIKey
	}
	client, err := genai.NewClient(ctx, cc)
	if err != nil {
		return nil, err
	}
	return &VertexBackend{client: client}, nil
}

// Client returns the underlying genai.Client.
func (v *VertexBackend) Client() *genai.Client {
	return v.client
}

// GenerateContentStream delegates to client.Models.GenerateContentStream.
func (v *VertexBackend) GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	return v.client.Models.GenerateContentStream(ctx, model, contents, cfg)
}

// ListModels queries Vertex AI for available models via client.Models.All.
func (v *VertexBackend) ListModels(ctx context.Context) ([]config.ModelInfo, error) {
	var discovered []config.ModelInfo
	for m, err := range v.client.Models.All(ctx) {
		if err != nil {
			if len(discovered) > 0 {
				break
			}
			return nil, err
		}
		if m == nil || m.Name == "" {
			continue
		}
		id := config.NormalizeModelID(m.Name, config.DefaultModels())
		displayName := m.DisplayName
		if displayName == "" {
			displayName = id
		}
		desc := m.Description
		if desc == "" {
			desc = fmt.Sprintf("Vertex AI model (%s)", m.Name)
		}
		ctxWindow := "Vertex AI"
		if m.InputTokenLimit > 0 {
			ctxWindow = fmt.Sprintf("%d tokens", m.InputTokenLimit)
		}
		maxOut := "Vertex AI"
		if m.OutputTokenLimit > 0 {
			maxOut = fmt.Sprintf("%d tokens", m.OutputTokenLimit)
		}
		discovered = append(discovered, config.ModelInfo{
			ID:               id,
			DisplayName:      displayName,
			Provider:         "Google / Vertex AI",
			Description:      desc,
			ContextWindow:    ctxWindow,
			MaxOutput:        maxOut,
			SupportsThinking: m.Thinking,
			SupportsTools:    true,
		})
	}
	return discovered, nil
}

// Agent coordinates the Vertex AI model, conversation history, and coding tools.
type Agent struct {
	mu          sync.RWMutex
	cfg         *config.Config
	models      []config.ModelInfo
	history     []*genai.Content
	toolExec    *tools.Executor
	backend     Backend
	initErr     error
	maxToolTurn int
}

// New creates a new coding Agent for Vertex AI.
// If backend is nil, it attempts to initialize a live VertexBackend via google.golang.org/genai.
// Even if live client initialization fails (e.g. GOOGLE_CLOUD_PROJECT not set yet),
// the Agent is still returned so the TUI can launch and allow `/model`, `/project`, `/location`, etc.
func New(ctx context.Context, cfg *config.Config, backend Backend) *Agent {
	if cfg == nil {
		cfg = config.DefaultConfig()
	}
	catalog := config.DefaultModels()
	cfg.Model = config.NormalizeModelID(cfg.Model, catalog)
	if cfg.Model == "" {
		cfg.Model = config.DefaultModelID
	}

	var initErr error
	if backend == nil {
		vb, err := NewVertexBackend(ctx, cfg)
		if err != nil {
			initErr = err
		} else {
			backend = vb
		}
	}

	return &Agent{
		cfg:         cfg,
		models:      catalog,
		history:     make([]*genai.Content, 0),
		toolExec:    tools.NewExecutor(cfg.WorkDir),
		backend:     backend,
		initErr:     initErr,
		maxToolTurn: 15,
	}
}

// Reconnect attempts to (re-)initialize the Vertex AI GenAI SDK client with current config.
func (a *Agent) Reconnect(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	vb, err := NewVertexBackend(ctx, a.cfg)
	if err != nil {
		a.initErr = err
		return err
	}
	a.backend = vb
	a.initErr = nil
	return nil
}

// ConnectionError returns any client initialization error.
func (a *Agent) ConnectionError() error {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.initErr
}

// IsConnected reports whether a GenAI backend is ready.
func (a *Agent) IsConnected() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.backend != nil
}

// Config returns a copy of the current configuration.
func (a *Agent) Config() config.Config {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return *a.cfg
}

// GetModel returns the active Vertex AI model ID.
func (a *Agent) GetModel() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.cfg.Model
}

// CurrentModelInfo returns metadata for the currently selected model.
func (a *Agent) CurrentModelInfo() config.ModelInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	info, _ := config.FindModel(a.cfg.Model, a.models)
	return info
}

// SetModel changes the active Vertex AI model used by the coding agent.
// It normalizes aliases (e.g. "pro" -> "gemini-2.5-pro", "flash" -> "gemini-2.5-flash")
// while also allowing any custom Vertex AI or Model Garden model ID.
// It returns the previous model ID, the new resolved ModelInfo, and whether it matched a preset in the catalog.
func (a *Agent) SetModel(modelID string) (string, config.ModelInfo, bool, error) {
	trimmed := strings.TrimSpace(modelID)
	if trimmed == "" {
		return "", config.ModelInfo{}, false, fmt.Errorf("モデル名を指定してください (model ID cannot be empty)")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	prev := a.cfg.Model
	canonical := config.NormalizeModelID(trimmed, a.models)
	info, known := config.FindModel(canonical, a.models)
	a.cfg.Model = info.ID

	// If it's a custom model not yet in the session catalog, add it so it appears in `/model` picker
	if !known {
		a.models = config.MergeModels(a.models, []config.ModelInfo{info})
	}

	_ = a.cfg.SaveToFile("")
	return prev, info, known, nil
}

// AvailableModels returns a copy of the current model catalog.
func (a *Agent) AvailableModels() []config.ModelInfo {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]config.ModelInfo, len(a.models))
	copy(out, a.models)
	return out
}

// RefreshModels queries Vertex AI for models and merges them into the available catalog.
func (a *Agent) RefreshModels(ctx context.Context) ([]config.ModelInfo, error) {
	a.mu.RLock()
	b := a.backend
	a.mu.RUnlock()

	if b == nil {
		if err := a.Reconnect(ctx); err != nil {
			return a.AvailableModels(), err
		}
		a.mu.RLock()
		b = a.backend
		a.mu.RUnlock()
	}

	discovered, err := b.ListModels(ctx)
	if err != nil {
		return a.AvailableModels(), err
	}

	a.mu.Lock()
	a.models = config.MergeModels(a.models, discovered)
	out := make([]config.ModelInfo, len(a.models))
	copy(out, a.models)
	a.mu.Unlock()
	return out, nil
}

// SetProject updates the Google Cloud project ID and reconnects the Vertex AI client.
func (a *Agent) SetProject(ctx context.Context, project string) error {
	project = strings.TrimSpace(project)
	if project == "" {
		return fmt.Errorf("プロジェクトIDを指定してください (project ID cannot be empty)")
	}
	a.mu.Lock()
	a.cfg.Project = project
	_ = a.cfg.SaveToFile("")
	a.mu.Unlock()
	return a.Reconnect(ctx)
}

// SetLocation updates the Google Cloud location/region and reconnects the Vertex AI client.
func (a *Agent) SetLocation(ctx context.Context, location string) error {
	location = strings.TrimSpace(location)
	if location == "" {
		return fmt.Errorf("ロケーション(リージョン)を指定してください (location cannot be empty)")
	}
	a.mu.Lock()
	a.cfg.Location = location
	_ = a.cfg.SaveToFile("")
	a.mu.Unlock()
	return a.Reconnect(ctx)
}

// ClearHistory resets the conversation transcript.
func (a *Agent) ClearHistory() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.history = make([]*genai.Content, 0)
}

// HistoryCount returns the number of Content turns in the conversation history.
func (a *Agent) HistoryCount() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.history)
}

// ToolSpecs returns the list of coding tools registered with the agent.
func (a *Agent) ToolSpecs() []tools.ToolSpec {
	return a.toolExec.Specs()
}

func (a *Agent) systemInstruction() *genai.Content {
	a.mu.RLock()
	workDir := a.cfg.WorkDir
	modelID := a.cfg.Model
	project := a.cfg.Project
	location := a.cfg.Location
	a.mu.RUnlock()

	prompt := fmt.Sprintf(
		"You are an expert AI coding agent running on Google Cloud Vertex AI (Agent Platform) via the Google GenAI SDK.\n"+
			"Current Model: %s\n"+
			"Vertex AI Project: %s (Location: %s)\n"+
			"Workspace Directory: %s\n\n"+
			"You have access to workspace coding tools: read_file, write_file, edit_file, list_files, search_files, and run_command.\n"+
			"When asked to inspect, create, modify, debug, or test code, proactively use these tools to examine the workspace and verify your changes.\n"+
			"Keep explanations clear, accurate, and concise.",
		modelID, project, location, workDir,
	)
	return genai.NewContentFromText(prompt, genai.RoleUser)
}

// RunTurn sends userPrompt to the active Vertex AI model, streams responses via onEvent,
// and automatically executes any requested coding tool calls in an agentic loop.
func (a *Agent) RunTurn(ctx context.Context, userPrompt string, onEvent func(StreamEvent)) error {
	a.mu.RLock()
	b := a.backend
	a.mu.RUnlock()

	if b == nil {
		if err := a.Reconnect(ctx); err != nil {
			return fmt.Errorf("Vertex AI クライアントが未接続です (GOOGLE_CLOUD_PROJECT または /project を設定してください): %w", err)
		}
		a.mu.RLock()
		b = a.backend
		a.mu.RUnlock()
	}

	userContent := genai.NewContentFromText(userPrompt, genai.RoleUser)
	a.mu.Lock()
	a.history = append(a.history, userContent)
	a.mu.Unlock()

	var totalIn, totalOut int32

	for step := 0; step < a.maxToolTurn; step++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		a.mu.RLock()
		currentModel := a.cfg.Model
		temp := a.cfg.Temperature
		maxTokens := a.cfg.MaxOutputTokens
		historySnapshot := make([]*genai.Content, len(a.history))
		copy(historySnapshot, a.history)
		a.mu.RUnlock()

		genCfg := &genai.GenerateContentConfig{
			SystemInstruction: a.systemInstruction(),
			Temperature:       genai.Ptr(temp),
			MaxOutputTokens:   maxTokens,
			Tools:             a.toolExec.Declarations(),
		}

		var (
			collectedParts []*genai.Part
			functionCalls  []*genai.FunctionCall
		)

		stream := b.GenerateContentStream(ctx, currentModel, historySnapshot, genCfg)
		for chunk, err := range stream {
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if chunk == nil {
				continue
			}
			if chunk.UsageMetadata != nil {
				if chunk.UsageMetadata.PromptTokenCount > 0 {
					totalIn = chunk.UsageMetadata.PromptTokenCount
				}
				if chunk.UsageMetadata.CandidatesTokenCount > 0 {
					totalOut += chunk.UsageMetadata.CandidatesTokenCount
				}
			}
			if len(chunk.Candidates) == 0 || chunk.Candidates[0].Content == nil {
				continue
			}
			for _, part := range chunk.Candidates[0].Content.Parts {
				if part == nil {
					continue
				}
				collectedParts = append(collectedParts, part)
				if part.Text != "" {
					if part.Thought {
						if onEvent != nil {
							onEvent(StreamEvent{
								Type:  EventThinkingDelta,
								Text:  part.Text,
								Model: currentModel,
							})
						}
					} else {
						if onEvent != nil {
							onEvent(StreamEvent{
								Type:  EventTextDelta,
								Text:  part.Text,
								Model: currentModel,
							})
						}
					}
				}
				if part.FunctionCall != nil {
					functionCalls = append(functionCalls, part.FunctionCall)
				}
			}
		}

		if len(collectedParts) > 0 {
			modelContent := &genai.Content{
				Role:  genai.RoleModel,
				Parts: mergeStreamParts(collectedParts),
			}
			a.mu.Lock()
			a.history = append(a.history, modelContent)
			a.mu.Unlock()
		}

		if len(functionCalls) == 0 {
			if onEvent != nil {
				onEvent(StreamEvent{
					Type:         EventTurnComplete,
					Model:        currentModel,
					InputTokens:  totalIn,
					OutputTokens: totalOut,
				})
			}
			return nil
		}

		// Execute requested function calls and send FunctionResponses back to the model
		var responseParts []*genai.Part
		for _, fc := range functionCalls {
			argsJSON, _ := json.Marshal(fc.Args)
			if onEvent != nil {
				onEvent(StreamEvent{
					Type:     EventToolCall,
					ToolName: fc.Name,
					ToolArgs: string(argsJSON),
					Model:    currentModel,
				})
			}

			result := a.toolExec.Execute(ctx, fc)
			resJSON, _ := json.Marshal(result)
			if onEvent != nil {
				onEvent(StreamEvent{
					Type:       EventToolResult,
					ToolName:   fc.Name,
					ToolResult: string(resJSON),
					Model:      currentModel,
				})
			}

			fnPart := genai.NewPartFromFunctionResponse(fc.Name, result)
			if fc.ID != "" && fnPart.FunctionResponse != nil {
				fnPart.FunctionResponse.ID = fc.ID
			}
			responseParts = append(responseParts, fnPart)
		}

		toolResponseContent := &genai.Content{
			Role:  genai.RoleUser,
			Parts: responseParts,
		}
		a.mu.Lock()
		a.history = append(a.history, toolResponseContent)
		a.mu.Unlock()
	}

	return fmt.Errorf("maximum tool execution turns (%d) reached", a.maxToolTurn)
}

// mergeStreamParts coalesces adjacent text parts from streaming chunks while keeping
// function calls and thought flags intact for clean conversation history.
func mergeStreamParts(parts []*genai.Part) []*genai.Part {
	if len(parts) == 0 {
		return nil
	}
	var merged []*genai.Part
	for _, p := range parts {
		if p == nil {
			continue
		}
		if p.Text != "" && p.FunctionCall == nil && p.FunctionResponse == nil && len(merged) > 0 {
			last := merged[len(merged)-1]
			if last.Text != "" && last.Thought == p.Thought && last.FunctionCall == nil && last.FunctionResponse == nil {
				last.Text += p.Text
				continue
			}
		}
		cp := *p
		merged = append(merged, &cp)
	}
	return merged
}
