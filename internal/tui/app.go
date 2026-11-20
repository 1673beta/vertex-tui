package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"vertex-tui/internal/agent"
	"vertex-tui/internal/config"
)

// MessageRole represents the sender of a chat message in the TUI transcript.
type MessageRole string

const (
	RoleUser     MessageRole = "user"
	RoleModel    MessageRole = "model"
	RoleThinking MessageRole = "thinking"
	RoleTool     MessageRole = "tool"
	RoleSystem   MessageRole = "system"
	RoleError    MessageRole = "error"
)

// ChatMessage is an entry in the TUI conversation transcript.
type ChatMessage struct {
	Role      MessageRole
	Title     string
	Content   string
	Model     string
	Timestamp time.Time
}

// Bubble Tea messages for async streaming from the Vertex AI agent and Model Garden discovery.
type (
	agentStreamEventMsg struct {
		event agent.StreamEvent
		ch    <-chan tea.Msg
	}
	agentTurnDoneMsg struct {
		err error
	}
	modelsRefreshedMsg struct {
		models []config.ModelInfo
		err    error
		silent bool
	}
)

// App is the Bubble Tea model for the Vertex AI Coding Agent TUI.
type App struct {
	ctx    context.Context
	agent  *agent.Agent
	styles Styles

	width  int
	height int
	ready  bool

	viewport viewport.Model
	spinner  spinner.Model

	// Chat transcript state
	messages      []ChatMessage
	isStreaming   bool
	cancelTurn    context.CancelFunc
	lastTokensIn  int32
	lastTokensOut int32

	// Text input editor state (Unicode rune-safe)
	inputRunes []rune
	cursorPos  int
	inputHist  []string
	histIdx    int
	suggestIdx int

	// Interactive /model modal state
	showModelModal   bool
	modelFilterRunes []rune
	selectedModelIdx int
	refreshingModels bool
	includeAllGarden bool
}

// NewApp creates a new Vertex AI Coding Agent TUI application.
func NewApp(ctx context.Context, ag *agent.Agent) *App {
	if ctx == nil {
		ctx = context.Background()
	}
	if ag == nil {
		ag = agent.New(ctx, config.DefaultConfig(), nil)
	}

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#7DCFFF"))

	vp := viewport.New(80, 20)
	vp.MouseWheelEnabled = true

	app := &App{
		ctx:      ctx,
		agent:    ag,
		styles:   DefaultStyles(),
		viewport: vp,
		spinner:  sp,
		width:    80,
		height:   24,
		histIdx:  -1,
	}

	cfg := ag.Config()
	modelInfo := ag.CurrentModelInfo()
	welcome := fmt.Sprintf(
		"Google Cloud Vertex AI (Agent Platform) Coding Agent TUI へようこそ！\n"+
			"  • 現在のモデル (Active Model): %s (%s)\n"+
			"  • Project / Location: %s / %s\n"+
			"  • Workspace: %s\n"+
			"  • モデル変更: `/model` で Model Garden 対応の対話型セレクタを開くか、`/model <モデル名>` を入力してください。\n"+
			"  • Model Garden 取得: 起動時および `/model` 表示時に Vertex AI Model Garden (`publishers/*/models`) から自動取得します (`/model garden [publisher]` も利用可)。\n"+
			"  • ヘルプ表示: `/help` | ショートカット: `Ctrl+O` でモデル選択画面を表示",
		modelInfo.ID,
		modelInfo.DisplayName,
		emptyFallback(cfg.Project, "(未設定 - /project <id> または GOOGLE_CLOUD_PROJECT で設定)"),
		cfg.Location,
		cfg.WorkDir,
	)
	app.appendMessage(RoleSystem, "Vertex AI Agent", welcome)
	if err := ag.ConnectionError(); err != nil {
		app.appendMessage(
			RoleSystem,
			"接続案内",
			fmt.Sprintf("現在オフライン/未認証モードで起動しています (%v)。\n`/model` でモデル変更や `/project <project-id>`・`/location <region>` の設定が可能です。", err),
		)
	}

	return app
}

// Init initializes the Bubble Tea program and triggers an asynchronous Vertex AI Model Garden fetch.
func (a *App) Init() tea.Cmd {
	a.refreshingModels = true
	return tea.Batch(
		a.spinner.Tick,
		a.refreshModelGardenCmd(agent.ModelGardenListOptions{}, true),
	)
}

// Update handles Bubble Tea messages (key presses, window resize, streaming agent events).
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.recalculateLayout()
		a.refreshViewportContent(true)
		return a, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		a.spinner, cmd = a.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case agentStreamEventMsg:
		a.handleStreamEvent(msg.event)
		cmds = append(cmds, waitForNextStreamMsg(msg.ch))
		return a, tea.Batch(cmds...)

	case agentTurnDoneMsg:
		a.isStreaming = false
		a.cancelTurn = nil
		if msg.err != nil && msg.err != context.Canceled {
			a.appendMessage(RoleError, "Vertex AI Error", msg.err.Error())
		}
		a.refreshViewportContent(true)
		return a, nil

	case modelsRefreshedMsg:
		a.refreshingModels = false
		if msg.err != nil {
			if !msg.silent {
				a.appendMessage(RoleError, "Model Garden", fmt.Sprintf("Vertex AI Model Garden からのモデル一覧取得に失敗しました: %v", msg.err))
			}
		} else {
			if !msg.silent {
				a.appendMessage(RoleSystem, "Model Garden", fmt.Sprintf("Vertex AI Model Garden からモデル一覧を取得・更新しました (合計 %d モデル)。", len(msg.models)))
			}
			a.syncSelectedModelIndex()
		}
		a.refreshViewportContent(true)
		return a, nil

	case tea.MouseMsg:
		if !a.showModelModal {
			var vpCmd tea.Cmd
			a.viewport, vpCmd = a.viewport.Update(msg)
			cmds = append(cmds, vpCmd)
		}

	case tea.KeyMsg:
		if a.showModelModal {
			return a.handleModelModalKey(msg)
		}
		return a.handleMainKey(msg)
	}

	return a, tea.Batch(cmds...)
}

func (a *App) handleModelModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := a.filteredModalModels()

	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		a.showModelModal = false
		return a, nil

	case tea.KeyUp, tea.KeyCtrlP, tea.KeyShiftTab:
		if len(filtered) > 0 {
			a.selectedModelIdx--
			if a.selectedModelIdx < 0 {
				a.selectedModelIdx = len(filtered) - 1
			}
		}
		return a, nil

	case tea.KeyDown, tea.KeyCtrlN, tea.KeyTab:
		if len(filtered) > 0 {
			a.selectedModelIdx = (a.selectedModelIdx + 1) % len(filtered)
		}
		return a, nil

	case tea.KeyCtrlR:
		a.refreshingModels = true
		return a, a.refreshModelGardenCmd(agent.ModelGardenListOptions{IncludeAll: a.includeAllGarden}, false)

	case tea.KeyCtrlA:
		a.includeAllGarden = !a.includeAllGarden
		a.refreshingModels = true
		return a, a.refreshModelGardenCmd(agent.ModelGardenListOptions{IncludeAll: a.includeAllGarden}, false)

	case tea.KeyEnter:
		filterText := strings.TrimSpace(string(a.modelFilterRunes))
		var targetModelID string
		if len(filtered) > 0 && a.selectedModelIdx >= 0 && a.selectedModelIdx < len(filtered) {
			targetModelID = filtered[a.selectedModelIdx].ID
		} else if filterText != "" {
			targetModelID = filterText
		}
		a.showModelModal = false
		a.modelFilterRunes = nil
		if targetModelID != "" {
			a.switchModel(targetModelID)
		}
		return a, nil

	case tea.KeyBackspace:
		if len(a.modelFilterRunes) > 0 {
			a.modelFilterRunes = a.modelFilterRunes[:len(a.modelFilterRunes)-1]
			a.selectedModelIdx = 0
		}
		return a, nil

	case tea.KeyCtrlU:
		a.modelFilterRunes = nil
		a.selectedModelIdx = 0
		return a, nil

	case tea.KeyRunes, tea.KeySpace:
		if len(msg.Runes) > 0 {
			a.modelFilterRunes = append(a.modelFilterRunes, msg.Runes...)
			a.selectedModelIdx = 0
		}
		return a, nil
	}

	return a, nil
}

func (a *App) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC:
		if a.isStreaming && a.cancelTurn != nil {
			a.cancelTurn()
			a.isStreaming = false
			a.cancelTurn = nil
			a.appendMessage(RoleSystem, "キャンセル", "モデルの生成を中断しました。")
			return a, nil
		}
		return a, tea.Quit

	case tea.KeyEsc:
		if a.isStreaming && a.cancelTurn != nil {
			a.cancelTurn()
			a.isStreaming = false
			a.cancelTurn = nil
			a.appendMessage(RoleSystem, "キャンセル", "モデルの生成を中断しました。")
			return a, nil
		}
		if len(a.inputRunes) > 0 {
			a.setInput("")
		}
		return a, nil

	case tea.KeyCtrlO, tea.KeyF2:
		return a, a.openModelModalCmd()

	case tea.KeyCtrlL:
		a.messages = nil
		a.refreshViewportContent(true)
		return a, nil

	case tea.KeyPgUp:
		a.viewport.HalfViewUp()
		return a, nil

	case tea.KeyPgDown:
		a.viewport.HalfViewDown()
		return a, nil

	case tea.KeyTab:
		suggestions := SuggestInput(string(a.inputRunes), a.agent.AvailableModels())
		if len(suggestions) > 0 {
			idx := a.suggestIdx % len(suggestions)
			completed := suggestions[idx]
			if completed == "/model" {
				completed = "/model "
			}
			a.setInput(completed)
			a.suggestIdx = (idx + 1) % len(suggestions)
		}
		return a, nil

	case tea.KeyUp:
		if len(a.inputHist) > 0 {
			if a.histIdx == -1 {
				a.histIdx = len(a.inputHist) - 1
			} else if a.histIdx > 0 {
				a.histIdx--
			}
			a.setInput(a.inputHist[a.histIdx])
		} else {
			a.viewport.LineUp(1)
		}
		return a, nil

	case tea.KeyDown:
		if a.histIdx != -1 {
			if a.histIdx < len(a.inputHist)-1 {
				a.histIdx++
				a.setInput(a.inputHist[a.histIdx])
			} else {
				a.histIdx = -1
				a.setInput("")
			}
		} else {
			a.viewport.LineDown(1)
		}
		return a, nil

	case tea.KeyLeft:
		if a.cursorPos > 0 {
			a.cursorPos--
		}
		return a, nil

	case tea.KeyRight:
		if a.cursorPos < len(a.inputRunes) {
			a.cursorPos++
		}
		return a, nil

	case tea.KeyHome, tea.KeyCtrlA:
		a.cursorPos = 0
		return a, nil

	case tea.KeyEnd, tea.KeyCtrlE:
		a.cursorPos = len(a.inputRunes)
		return a, nil

	case tea.KeyCtrlU:
		a.inputRunes = a.inputRunes[a.cursorPos:]
		a.cursorPos = 0
		a.suggestIdx = 0
		return a, nil

	case tea.KeyBackspace:
		if a.cursorPos > 0 && len(a.inputRunes) > 0 {
			a.inputRunes = append(a.inputRunes[:a.cursorPos-1], a.inputRunes[a.cursorPos:]...)
			a.cursorPos--
			a.suggestIdx = 0
		}
		return a, nil

	case tea.KeyDelete:
		if a.cursorPos < len(a.inputRunes) {
			a.inputRunes = append(a.inputRunes[:a.cursorPos], a.inputRunes[a.cursorPos+1:]...)
			a.suggestIdx = 0
		}
		return a, nil

	case tea.KeyEnter:
		raw := strings.TrimSpace(string(a.inputRunes))
		if raw == "" {
			return a, nil
		}
		a.inputHist = append(a.inputHist, raw)
		a.histIdx = -1
		a.setInput("")
		return a.handleSubmittedInput(raw)

	case tea.KeySpace:
		a.insertRunes([]rune{' '})
		return a, nil

	case tea.KeyRunes:
		if len(msg.Runes) > 0 {
			a.insertRunes(msg.Runes)
		}
		return a, nil
	}

	return a, nil
}

func (a *App) insertRunes(r []rune) {
	clean := make([]rune, 0, len(r))
	for _, ch := range r {
		if ch == '\r' || ch == '\n' {
			clean = append(clean, ' ')
		} else {
			clean = append(clean, ch)
		}
	}
	left := make([]rune, a.cursorPos)
	copy(left, a.inputRunes[:a.cursorPos])
	right := a.inputRunes[a.cursorPos:]
	a.inputRunes = append(append(left, clean...), right...)
	a.cursorPos += len(clean)
	a.suggestIdx = 0
}

func (a *App) setInput(s string) {
	a.inputRunes = []rune(s)
	a.cursorPos = len(a.inputRunes)
	a.suggestIdx = 0
}

func (a *App) handleSubmittedInput(raw string) (tea.Model, tea.Cmd) {
	if strings.HasPrefix(raw, "/") {
		return a.executeSlashCommand(raw)
	}

	if a.isStreaming {
		a.appendMessage(RoleSystem, "Busy", "現在応答を生成中です (`Esc` でキャンセルできます)。")
		return a, nil
	}

	a.appendMessage(RoleUser, "You", raw)
	return a, a.startAgentTurnCmd(raw)
}

func (a *App) executeSlashCommand(raw string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return a, nil
	}
	cmd := strings.ToLower(fields[0])
	arg := strings.TrimSpace(strings.TrimPrefix(raw, fields[0]))

	switch cmd {
	case "/model", "/models":
		return a.handleModelCommand(arg)

	case "/project":
		if arg == "" {
			cfg := a.agent.Config()
			a.appendMessage(RoleSystem, "Project", fmt.Sprintf("現在の Google Cloud Project: %s\n変更するには `/project <project-id>` を実行してください。", emptyFallback(cfg.Project, "(未設定)")))
			return a, nil
		}
		err := a.agent.SetProject(a.ctx, arg)
		if err != nil {
			a.appendMessage(RoleSystem, "Project", fmt.Sprintf("Project を %q に設定しました (クライアント接続状態: %v)", arg, err))
			return a, nil
		}
		a.appendMessage(RoleSystem, "Project", fmt.Sprintf("Google Cloud Project を %q に変更し、Vertex AI クライアントを再接続しました。Model Garden からモデル一覧を取得します...", arg))
		a.refreshingModels = true
		return a, a.refreshModelGardenCmd(agent.ModelGardenListOptions{IncludeAll: a.includeAllGarden}, true)

	case "/location", "/region":
		if arg == "" {
			cfg := a.agent.Config()
			a.appendMessage(RoleSystem, "Location", fmt.Sprintf("現在の Vertex AI Location: %s\n変更するには `/location <region>` (例: `/location us-central1`, `/location global`, `/location asia-northeast1`) を実行してください。", cfg.Location))
			return a, nil
		}
		err := a.agent.SetLocation(a.ctx, arg)
		if err != nil {
			a.appendMessage(RoleSystem, "Location", fmt.Sprintf("Location を %q に設定しました (クライアント接続状態: %v)", arg, err))
			return a, nil
		}
		a.appendMessage(RoleSystem, "Location", fmt.Sprintf("Vertex AI Location を %q に変更し、クライアントを再接続しました。Model Garden からモデル一覧を取得します...", arg))
		a.refreshingModels = true
		return a, a.refreshModelGardenCmd(agent.ModelGardenListOptions{IncludeAll: a.includeAllGarden}, true)

	case "/tools":
		var sb strings.Builder
		sb.WriteString("利用可能なコーディングエージェント・ツール (Vertex AI Function Calling):\n")
		for _, spec := range a.agent.ToolSpecs() {
			fmt.Fprintf(&sb, "  • %-14s : %s\n", spec.Name, spec.Description)
		}
		a.appendMessage(RoleSystem, "Tools", strings.TrimRight(sb.String(), "\n"))
		return a, nil

	case "/status":
		cfg := a.agent.Config()
		m := a.agent.CurrentModelInfo()
		connStatus := "Connected (Vertex AI Backend)"
		if !a.agent.IsConnected() {
			if err := a.agent.ConnectionError(); err != nil {
				connStatus = fmt.Sprintf("Disconnected (%v)", err)
			} else {
				connStatus = "Disconnected"
			}
		}
		gardenStatus := "未同期 (オフラインキャッシュ / `/model refresh` で取得)"
		if a.agent.IsModelGardenSynced() {
			gardenStatus = fmt.Sprintf("同期済み (%d models loaded from Vertex AI Model Garden)", len(a.agent.AvailableModels()))
		}
		statusText := fmt.Sprintf(
			"Vertex AI Agent Status:\n"+
				"  • Model         : %s (%s)\n"+
				"  • Resource      : %s\n"+
				"  • Provider      : %s\n"+
				"  • Model Garden  : %s\n"+
				"  • Project       : %s\n"+
				"  • Location      : %s\n"+
				"  • Backend       : %s\n"+
				"  • Workspace     : %s\n"+
				"  • History       : %d turns",
			m.ID, m.DisplayName,
			emptyFallback(m.ResourceName, "publishers/google/models/"+m.ID),
			m.Provider,
			gardenStatus,
			emptyFallback(cfg.Project, "(unset)"),
			cfg.Location,
			connStatus,
			cfg.WorkDir,
			a.agent.HistoryCount(),
		)
		a.appendMessage(RoleSystem, "Status", statusText)
		return a, nil

	case "/clear":
		a.agent.ClearHistory()
		a.messages = nil
		a.appendMessage(RoleSystem, "Clear", fmt.Sprintf("会話履歴をクリアしました (現在のモデル: %s)。", a.agent.GetModel()))
		return a, nil

	case "/help", "/?":
		var sb strings.Builder
		sb.WriteString("利用可能なスラッシュコマンド (Slash Commands):\n")
		for _, s := range SlashCommands() {
			fmt.Fprintf(&sb, "  • %-54s : %s\n", s.Usage, s.Description)
		}
		sb.WriteString("\nModel Garden コマンド例:\n")
		sb.WriteString("  • /model                     : Model Garden 対応の対話型モデル選択画面を開く\n")
		sb.WriteString("  • /model garden              : Vertex AI Model Garden (google, anthropic, meta, mistralai, deepseek-ai 等) から最新一覧を取得\n")
		sb.WriteString("  • /model garden anthropic    : 指定パブリッシャー (publishers/anthropic/models) のモデルを Model Garden から取得\n")
		sb.WriteString("  • /model garden all          : 非LLMモデルも含めて Model Garden の全モデルを取得\n")
		sb.WriteString("\nキーボードショートカット:\n")
		sb.WriteString("  • Enter       : メッセージ送信 / コマンド実行\n")
		sb.WriteString("  • Tab         : スラッシュコマンド・モデル名の自動補完\n")
		sb.WriteString("  • Ctrl+O / F2 : `/model` モデル選択モーダルを開く\n")
		sb.WriteString("  • Ctrl+R      : (モデル選択画面内) Vertex AI Model Garden から再取得\n")
		sb.WriteString("  • Ctrl+A      : (モデル選択画面内) 生成AIモデルのみ / Model Garden 全モデル表示の切替\n")
		sb.WriteString("  • Esc         : 生成のキャンセル / モーダルを閉じる\n")
		sb.WriteString("  • PgUp / PgDn : チャット履歴のスクロール\n")
		sb.WriteString("  • Ctrl+C      : 生成中断または終了")
		a.appendMessage(RoleSystem, "Help", sb.String())
		return a, nil

	case "/exit", "/quit", "/q":
		return a, tea.Quit

	default:
		a.appendMessage(RoleError, "Unknown Command", fmt.Sprintf("不明なコマンド %q です。`/help` でコマンド一覧を確認してください。", cmd))
		return a, nil
	}
}

func (a *App) handleModelCommand(arg string) (tea.Model, tea.Cmd) {
	trimmedArg := strings.TrimSpace(arg)
	fields := strings.Fields(trimmedArg)
	subCmd := ""
	if len(fields) > 0 {
		subCmd = strings.ToLower(fields[0])
	}

	switch subCmd {
	case "":
		return a, a.openModelModalCmd()

	case "list", "ls":
		a.appendMessage(RoleSystem, "Models", a.formatModelList())
		if !a.agent.IsModelGardenSynced() && !a.refreshingModels {
			a.refreshingModels = true
			return a, a.refreshModelGardenCmd(agent.ModelGardenListOptions{IncludeAll: a.includeAllGarden}, false)
		}
		return a, nil

	case "current", "info":
		info := a.agent.CurrentModelInfo()
		text := fmt.Sprintf(
			"現在のモデル (Current Vertex AI / Model Garden Model):\n"+
				"  • ID            : %s\n"+
				"  • Resource Name : %s\n"+
				"  • Display Name  : %s\n"+
				"  • Publisher     : %s (%s)\n"+
				"  • Launch Stage  : %s\n"+
				"  • Source        : %s\n"+
				"  • Description   : %s\n"+
				"  • Context       : %s (Max Output: %s)\n"+
				"  • Aliases       : %s",
			info.ID,
			emptyFallback(info.ResourceName, "publishers/google/models/"+info.ID),
			info.DisplayName,
			emptyFallback(info.Publisher, "google"),
			info.Provider,
			emptyFallback(info.LaunchStage, "GA"),
			emptyFallback(info.Source, config.SourceModelGarden),
			info.Description,
			info.ContextWindow,
			info.MaxOutput,
			emptyFallback(strings.Join(info.Aliases, ", "), "-"),
		)
		a.appendMessage(RoleSystem, "Current Model", text)
		return a, nil

	case "refresh", "sync":
		a.refreshingModels = true
		a.appendMessage(RoleSystem, "Model Garden", "Vertex AI Model Garden (`publishers/*/models`) から利用可能なモデル一覧を取得しています...")
		return a, a.refreshModelGardenCmd(agent.ModelGardenListOptions{IncludeAll: a.includeAllGarden}, false)

	case "garden":
		opts := agent.ModelGardenListOptions{
			IncludeAll: a.includeAllGarden,
		}
		if len(fields) > 1 {
			pubArg := strings.ToLower(fields[1])
			if pubArg == "all" || pubArg == "--all" {
				opts.IncludeAll = true
				a.includeAllGarden = true
			} else {
				opts.Publishers = strings.Split(pubArg, ",")
				// Also pre-filter the modal to this publisher if modal is opened
			}
		}
		a.refreshingModels = true
		pubLabel := strings.Join(config.DefaultModelGardenPublishers, ", ")
		if len(opts.Publishers) > 0 {
			pubLabel = strings.Join(opts.Publishers, ", ")
		}
		a.appendMessage(
			RoleSystem,
			"Model Garden",
			fmt.Sprintf("Vertex AI Model Garden (publishers: %s) からモデル一覧を取得しています...", pubLabel),
		)
		return a, a.refreshModelGardenCmd(opts, false)

	default:
		resolved := ResolveModelArg(trimmedArg, a.agent.AvailableModels())
		a.switchModel(resolved)
		return a, nil
	}
}

func (a *App) openModelModalCmd() tea.Cmd {
	a.showModelModal = true
	a.modelFilterRunes = nil
	a.syncSelectedModelIndex()
	if !a.agent.IsModelGardenSynced() && !a.refreshingModels {
		a.refreshingModels = true
		return a.refreshModelGardenCmd(agent.ModelGardenListOptions{IncludeAll: a.includeAllGarden}, true)
	}
	return nil
}

func (a *App) syncSelectedModelIndex() {
	current := a.agent.GetModel()
	models := a.filteredModalModels()
	a.selectedModelIdx = 0
	for i, m := range models {
		if strings.EqualFold(m.ID, current) {
			a.selectedModelIdx = i
			return
		}
	}
}

func (a *App) filteredModalModels() []config.ModelInfo {
	return config.FilterModels(a.agent.AvailableModels(), string(a.modelFilterRunes))
}

func (a *App) switchModel(targetModelID string) {
	prev, info, known, err := a.agent.SetModel(targetModelID)
	if err != nil {
		a.appendMessage(RoleError, "Model Switch Error", err.Error())
		return
	}
	if strings.EqualFold(prev, info.ID) {
		a.appendMessage(
			RoleSystem,
			"Model",
			fmt.Sprintf("モデルは既に %s (%s) に設定されています。", info.ID, info.DisplayName),
		)
		return
	}

	var note string
	if !known {
		note = fmt.Sprintf("\n  • Resource: %s (カスタム Model Garden / Endpoint モデル)", info.ResourceName)
	} else if info.ResourceName != "" {
		note = fmt.Sprintf("\n  • Model Garden Resource: %s", info.ResourceName)
	}
	a.appendMessage(
		RoleSystem,
		"Model Changed",
		fmt.Sprintf(
			"モデルを `%s` から `%s` (%s) に変更しました。\n  • %s%s",
			prev,
			info.ID,
			info.DisplayName,
			info.Description,
			note,
		),
	)
}

func (a *App) formatModelList() string {
	models := a.agent.AvailableModels()
	current := a.agent.GetModel()
	var sb strings.Builder
	sourceStatus := "オフラインプリセット (`/model garden` で Vertex AI Model Garden API から取得可能)"
	if a.agent.IsModelGardenSynced() {
		sourceStatus = "Vertex AI Model Garden API 同期済み"
	}
	fmt.Fprintf(&sb, "利用可能な Vertex AI / Model Garden モデル一覧 [%s]:\n", sourceStatus)
	for i, m := range models {
		marker := "  "
		activeSuffix := ""
		if strings.EqualFold(m.ID, current) {
			marker = "● "
			activeSuffix = " [現在のモデル / current]"
		}
		stageBadge := ""
		if m.LaunchStage != "" {
			stageBadge = fmt.Sprintf(" [%s]", m.LaunchStage)
		}
		aliasText := ""
		if len(m.Aliases) > 0 {
			aliasText = fmt.Sprintf(" (aliases: %s)", strings.Join(m.Aliases, ", "))
		}
		fmt.Fprintf(&sb, "%s%2d. %-38s - %s%s%s%s\n      %s | %s\n",
			marker, i+1, m.ID, m.DisplayName, stageBadge, aliasText, activeSuffix, m.Provider, m.Description)
	}
	sb.WriteString("\n使い方: `/model` で対話選択、`/model <モデルID|番号|エイリアス>` で直接変更、`/model garden [publisher]` で Model Garden から再取得")
	return sb.String()
}

func (a *App) startAgentTurnCmd(prompt string) tea.Cmd {
	turnCtx, cancel := context.WithCancel(a.ctx)
	a.cancelTurn = cancel
	a.isStreaming = true

	ch := make(chan tea.Msg, 64)
	go func() {
		defer close(ch)
		err := a.agent.RunTurn(turnCtx, prompt, func(ev agent.StreamEvent) {
			ch <- agentStreamEventMsg{event: ev, ch: ch}
		})
		ch <- agentTurnDoneMsg{err: err}
	}()

	return waitForNextStreamMsg(ch)
}

func waitForNextStreamMsg(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return agentTurnDoneMsg{}
		}
		return msg
	}
}

func (a *App) refreshModelGardenCmd(opts agent.ModelGardenListOptions, silent bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 15*time.Second)
		defer cancel()
		models, err := a.agent.RefreshModelGarden(ctx, opts)
		return modelsRefreshedMsg{models: models, err: err, silent: silent}
	}
}

func (a *App) handleStreamEvent(ev agent.StreamEvent) {
	switch ev.Type {
	case agent.EventThinkingDelta:
		a.appendOrExtendLastMessage(RoleThinking, fmt.Sprintf("Thinking (%s)", ev.Model), ev.Text, ev.Model)
	case agent.EventTextDelta:
		a.appendOrExtendLastMessage(RoleModel, fmt.Sprintf("Vertex AI (%s)", ev.Model), ev.Text, ev.Model)
	case agent.EventToolCall:
		a.appendMessage(RoleTool, fmt.Sprintf("Tool Call: %s", ev.ToolName), fmt.Sprintf("Args: %s", ev.ToolArgs))
	case agent.EventToolResult:
		summary := ev.ToolResult
		if len(summary) > 400 {
			summary = summary[:400] + "... (truncated)"
		}
		a.appendMessage(RoleTool, fmt.Sprintf("Tool Result: %s", ev.ToolName), summary)
	case agent.EventTurnComplete:
		if ev.InputTokens > 0 || ev.OutputTokens > 0 {
			a.lastTokensIn = ev.InputTokens
			a.lastTokensOut = ev.OutputTokens
		}
	}
	a.refreshViewportContent(true)
}

func (a *App) appendMessage(role MessageRole, title, content string) {
	a.messages = append(a.messages, ChatMessage{
		Role:      role,
		Title:     title,
		Content:   content,
		Model:     a.agent.GetModel(),
		Timestamp: time.Now(),
	})
	a.refreshViewportContent(true)
}

func (a *App) appendOrExtendLastMessage(role MessageRole, title, delta, model string) {
	if len(a.messages) > 0 {
		last := &a.messages[len(a.messages)-1]
		if last.Role == role && last.Model == model {
			last.Content += delta
			return
		}
	}
	a.messages = append(a.messages, ChatMessage{
		Role:      role,
		Title:     title,
		Content:   delta,
		Model:     model,
		Timestamp: time.Now(),
	})
}

func (a *App) recalculateLayout() {
	w := a.width
	if w < 40 {
		w = 40
	}
	h := a.height
	if h < 12 {
		h = 12
	}
	a.viewport.Width = w
	vpHeight := h - 7
	if vpHeight < 4 {
		vpHeight = 4
	}
	a.viewport.Height = vpHeight
	a.ready = true
}

func (a *App) refreshViewportContent(scrollToBottom bool) {
	var sb strings.Builder

	for i, m := range a.messages {
		if i > 0 {
			sb.WriteString("\n")
		}
		timeStr := m.Timestamp.Format("15:04:05")
		switch m.Role {
		case RoleUser:
			header := a.styles.UserLabel.Render("❯ You") + " " + a.styles.DimStyle.Render("["+timeStr+"]")
			sb.WriteString(header + "\n")
			sb.WriteString(indentBlock(m.Content, "  ") + "\n")

		case RoleModel:
			header := a.styles.ModelLabel.Render("✦ "+m.Title) + " " + a.styles.DimStyle.Render("["+timeStr+"]")
			sb.WriteString(header + "\n")
			sb.WriteString(indentBlock(m.Content, "  ") + "\n")

		case RoleThinking:
			header := a.styles.ThinkingStyle.Render("💭 " + m.Title)
			sb.WriteString(header + "\n")
			sb.WriteString(a.styles.ThinkingStyle.Render(indentBlock(m.Content, "  ")) + "\n")

		case RoleTool:
			header := a.styles.ToolLabel.Render("🔧 " + m.Title)
			sb.WriteString(header + "\n")
			sb.WriteString(a.styles.DimStyle.Render(indentBlock(m.Content, "  ")) + "\n")

		case RoleSystem:
			header := a.styles.SystemLabel.Render("ℹ " + m.Title)
			sb.WriteString(header + "\n")
			sb.WriteString(indentBlock(m.Content, "  ") + "\n")

		case RoleError:
			header := a.styles.ErrorStyle.Render("✖ " + m.Title)
			sb.WriteString(header + "\n")
			sb.WriteString(a.styles.ErrorStyle.Render(indentBlock(m.Content, "  ")) + "\n")
		}
	}

	if a.isStreaming {
		sb.WriteString("\n  " + a.spinner.View() + " " + a.styles.HighlightStyle.Render(fmt.Sprintf("Vertex AI (%s) が応答を生成中... (Esc で中断)", a.agent.GetModel())) + "\n")
	}

	a.viewport.SetContent(sb.String())
	if scrollToBottom {
		a.viewport.GotoBottom()
	}
}

// View renders the full TUI screen.
func (a *App) View() string {
	header := a.renderHeader()
	footer := a.renderFooter()

	if a.showModelModal {
		modal := a.renderModelModal()
		return lipgloss.JoinVertical(lipgloss.Left, header, modal, footer)
	}

	chatView := a.viewport.View()
	inputView := a.renderInputArea()
	return lipgloss.JoinVertical(lipgloss.Left, header, chatView, inputView, footer)
}

func (a *App) renderHeader() string {
	cfg := a.agent.Config()
	modelInfo := a.agent.CurrentModelInfo()

	title := a.styles.HeaderTitle.Render("◆ Vertex AI Coding Agent")
	modelBadge := a.styles.ModelBadge.Render(fmt.Sprintf("Model: %s", modelInfo.ID))

	projLoc := fmt.Sprintf("GCP: %s/%s", emptyFallback(cfg.Project, "unset"), cfg.Location)
	var statusBadge string
	if a.agent.IsConnected() {
		if a.agent.IsModelGardenSynced() {
			statusBadge = a.styles.StatusConnected.Render("● Vertex AI (Model Garden)")
		} else {
			statusBadge = a.styles.StatusConnected.Render("● Vertex AI")
		}
	} else {
		statusBadge = a.styles.StatusWarn.Render("○ Setup Needed")
	}

	line := fmt.Sprintf("%s  %s  %s  %s", title, modelBadge, a.styles.DimStyle.Render(projLoc), statusBadge)
	return a.styles.HeaderBar.Width(a.width).Render(line)
}

func (a *App) renderInputArea() string {
	var sections []string

	inputStr := string(a.inputRunes)
	suggestions := SuggestInput(inputStr, a.agent.AvailableModels())
	if len(suggestions) > 0 {
		maxShow := 5
		if len(suggestions) < maxShow {
			maxShow = len(suggestions)
		}
		var items []string
		activeIdx := a.suggestIdx % len(suggestions)
		for i := 0; i < maxShow; i++ {
			if i == activeIdx {
				items = append(items, a.styles.HighlightStyle.Render("["+suggestions[i]+"]"))
			} else {
				items = append(items, suggestions[i])
			}
		}
		hint := fmt.Sprintf("候補 (Tabで補完): %s", strings.Join(items, "  "))
		sections = append(sections, a.styles.SuggestionBar.Width(a.width).Render(hint))
	}

	left := string(a.inputRunes[:a.cursorPos])
	cursorChar := "█"
	right := ""
	if a.cursorPos < len(a.inputRunes) {
		right = string(a.inputRunes[a.cursorPos:])
	}

	var promptLine string
	if len(a.inputRunes) == 0 {
		placeholder := a.styles.DimStyle.Render("メッセージを入力、または /model で Model Garden からモデル変更 (/help で一覧)...")
		promptLine = fmt.Sprintf("❯ %s%s", cursorChar, placeholder)
	} else {
		promptLine = fmt.Sprintf("❯ %s%s%s", left, cursorChar, right)
	}

	boxWidth := a.width - 2
	if boxWidth < 20 {
		boxWidth = 20
	}
	sections = append(sections, a.styles.InputBoxFocused.Width(boxWidth).Render(promptLine))
	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

func (a *App) renderModelModal() string {
	currentModelID := a.agent.GetModel()
	filterText := string(a.modelFilterRunes)
	filtered := a.filteredModalModels()

	var sb strings.Builder
	gardenBadge := a.styles.StatusWarn.Render("[Fallback Catalog - Ctrl+R で Model Garden から取得]")
	if a.agent.IsModelGardenSynced() {
		gardenBadge = a.styles.StatusConnected.Render(fmt.Sprintf("[● Model Garden API 同期済 (%d models)]", len(a.agent.AvailableModels())))
	}
	sb.WriteString(a.styles.ModalTitle.Render("✦ Vertex AI Model Garden モデル選択 (/model)") + "  " + gardenBadge + "\n")
	sb.WriteString(a.styles.DimStyle.Render("↑/↓: 選択移動 | Enter: 変更 | Esc: 閉じる | Ctrl+R: Model Garden再取得 | Ctrl+A: 全モデル表示切替") + "\n\n")

	filterDisplay := filterText
	if filterDisplay == "" {
		filterDisplay = a.styles.DimStyle.Render("モデル名・パブリッシャー (google, anthropic, meta, mistralai, deepseek-ai) で検索、または直接入力...")
	} else {
		filterDisplay = a.styles.HighlightStyle.Render(filterText) + "█"
	}
	sb.WriteString(fmt.Sprintf("🔍 検索 / Model Garden ID: %s\n\n", filterDisplay))

	if a.refreshingModels {
		sb.WriteString("  " + a.spinner.View() + " Vertex AI Model Garden (`publishers/*/models`) からモデル一覧を取得中...\n\n")
	}

	if len(filtered) == 0 {
		sb.WriteString(a.styles.ModalItemSelected.Render(fmt.Sprintf("❯ カスタム Vertex AI / Model Garden モデルを使用: %q", strings.TrimSpace(filterText))) + "\n")
		sb.WriteString(a.styles.ModalDesc.Render("Enter キーを押すとこのモデルIDに切り替えます (例: publishers/<publisher>/models/<model>)。") + "\n")
	} else {
		// Show a scrollable window around selectedModelIdx so large Model Garden lists fit cleanly on screen
		maxVisible := 8
		if a.height > 26 {
			maxVisible = (a.height - 14) / 2
			if maxVisible < 5 {
				maxVisible = 5
			}
		}
		startIdx := 0
		if a.selectedModelIdx >= maxVisible {
			startIdx = a.selectedModelIdx - maxVisible + 1
		}
		endIdx := startIdx + maxVisible
		if endIdx > len(filtered) {
			endIdx = len(filtered)
		}

		if startIdx > 0 {
			sb.WriteString(a.styles.DimStyle.Render(fmt.Sprintf("  ▲ 他 %d 件のモデル (↑ キーでスクロール)\n", startIdx)))
		}

		for i := startIdx; i < endIdx; i++ {
			m := filtered[i]
			isSelected := i == a.selectedModelIdx
			isCurrent := strings.EqualFold(m.ID, currentModelID)

			prefix := "  "
			if isSelected {
				prefix = "❯ "
			}

			line := fmt.Sprintf("%s%-34s (%s)", prefix, m.ID, m.DisplayName)
			if isSelected {
				line = a.styles.ModalItemSelected.Render(line)
			} else {
				line = a.styles.ModalItem.Render(line)
			}

			if isCurrent {
				line += " " + a.styles.ModalActiveBadge.Render("[● Active]")
			}
			if m.Recommended {
				line += " " + a.styles.HighlightStyle.Render("[推奨]")
			}
			if m.LaunchStage != "" {
				line += " " + a.styles.DimStyle.Render("["+m.LaunchStage+"]")
			}
			if m.Source == config.SourceModelGarden {
				line += " " + a.styles.HeaderBadge.Render("[Model Garden]")
			} else if m.Source == config.SourceVertexTuned {
				line += " " + a.styles.HeaderBadge.Render("[Tuned]")
			}
			if m.SupportsThinking {
				line += " " + a.styles.DimStyle.Render("[Thinking]")
			}

			sb.WriteString(line + "\n")
			descLine := fmt.Sprintf("[%s] %s | Context: %s", emptyFallback(m.Publisher, "google"), m.Description, m.ContextWindow)
			sb.WriteString(a.styles.ModalDesc.Render(descLine) + "\n")
		}

		if endIdx < len(filtered) {
			sb.WriteString(a.styles.DimStyle.Render(fmt.Sprintf("  ▼ 他 %d 件のモデル (↓ キーでスクロール)\n", len(filtered)-endIdx)))
		}
	}

	modalWidth := a.width - 4
	if modalWidth < 40 {
		modalWidth = 40
	}
	return a.styles.ModalBox.Width(modalWidth).Render(strings.TrimRight(sb.String(), "\n"))
}

func (a *App) renderFooter() string {
	modelID := a.agent.GetModel()
	tokenInfo := ""
	if a.lastTokensIn > 0 || a.lastTokensOut > 0 {
		tokenInfo = fmt.Sprintf(" | Tokens: in=%d out=%d", a.lastTokensIn, a.lastTokensOut)
	}
	help := fmt.Sprintf("/model: モデル変更 (現在: %s) | /model garden: Model Garden取得 | Ctrl+O: 選択画面 | /help | Ctrl+C: 終了%s", modelID, tokenInfo)
	return a.styles.FooterBar.Width(a.width).Render(help)
}

// GetActiveModel returns the currently selected Vertex AI model ID.
func (a *App) GetActiveModel() string {
	return a.agent.GetModel()
}

// IsModelModalOpen reports whether the `/model` interactive modal is currently visible.
func (a *App) IsModelModalOpen() bool {
	return a.showModelModal
}

// Messages returns a copy of the chat transcript messages.
func (a *App) Messages() []ChatMessage {
	out := make([]ChatMessage, len(a.messages))
	copy(out, a.messages)
	return out
}

func indentBlock(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = indent + l
	}
	return strings.Join(lines, "\n")
}

func emptyFallback(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}
