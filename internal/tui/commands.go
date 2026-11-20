package tui

import (
	"fmt"
	"strconv"
	"strings"

	"vertex-tui/internal/config"
)

// SlashCommandSpec describes an available slash command in the TUI.
type SlashCommandSpec struct {
	Command     string
	Usage       string
	Description string
}

// SlashCommands returns the built-in slash commands supported by vertex-tui.
func SlashCommands() []SlashCommandSpec {
	return []SlashCommandSpec{
		{
			Command:     "/model",
			Usage:       "/model [model-id | list | current | refresh]",
			Description: "使用するVertex AIモデルを変更・一覧表示します (Interactive model picker or direct switch)",
		},
		{
			Command:     "/project",
			Usage:       "/project [gcp-project-id]",
			Description: "Google Cloud プロジェクトIDの確認・変更を行います",
		},
		{
			Command:     "/location",
			Usage:       "/location [region]",
			Description: "Vertex AI ロケーション(リージョン)の確認・変更を行います (例: us-central1, global, asia-northeast1)",
		},
		{
			Command:     "/tools",
			Usage:       "/tools",
			Description: "コーディングエージェントが利用可能なツール一覧を表示します",
		},
		{
			Command:     "/status",
			Usage:       "/status",
			Description: "現在のVertex AI接続状態・モデル・ワークスペース情報を表示します",
		},
		{
			Command:     "/clear",
			Usage:       "/clear",
			Description: "会話履歴と画面表示をクリアします",
		},
		{
			Command:     "/help",
			Usage:       "/help",
			Description: "ヘルプとコマンド一覧を表示します",
		},
		{
			Command:     "/exit",
			Usage:       "/exit",
			Description: "アプリケーションを終了します (/quit)",
		},
	}
}

// SuggestInput returns autocomplete suggestions for a slash command prefix in the input bar.
func SuggestInput(input string, models []config.ModelInfo) []string {
	trimmedLeft := strings.TrimLeft(input, " ")
	if !strings.HasPrefix(trimmedLeft, "/") {
		return nil
	}

	lower := strings.ToLower(trimmedLeft)

	// If user is typing "/model <prefix>" or "/models <prefix>", suggest matching model IDs
	if strings.HasPrefix(lower, "/model ") || strings.HasPrefix(lower, "/models ") {
		parts := strings.SplitN(trimmedLeft, " ", 2)
		cmdPrefix := parts[0]
		argPrefix := ""
		if len(parts) > 1 {
			argPrefix = strings.ToLower(strings.TrimSpace(parts[1]))
		}
		var out []string
		for _, m := range models {
			if argPrefix == "" || strings.HasPrefix(strings.ToLower(m.ID), argPrefix) || strings.Contains(strings.ToLower(m.ID), argPrefix) {
				out = append(out, fmt.Sprintf("%s %s", cmdPrefix, m.ID))
			}
			for _, alias := range m.Aliases {
				if argPrefix != "" && strings.HasPrefix(strings.ToLower(alias), argPrefix) && !strings.HasPrefix(strings.ToLower(m.ID), argPrefix) {
					out = append(out, fmt.Sprintf("%s %s", cmdPrefix, m.ID))
					break
				}
			}
		}
		for _, sub := range []string{"list", "current", "refresh"} {
			if argPrefix != "" && strings.HasPrefix(sub, argPrefix) {
				out = append(out, fmt.Sprintf("%s %s", cmdPrefix, sub))
			}
		}
		return out
	}

	// Otherwise suggest matching slash commands
	var out []string
	for _, spec := range SlashCommands() {
		if strings.HasPrefix(spec.Command, lower) {
			out = append(out, spec.Command)
		}
	}
	return out
}

// ResolveModelArg resolves a `/model <arg>` argument, supporting:
// - 1-based numeric index (e.g. `/model 1`, `/model 2`)
// - preset ID or alias (e.g. `gemini-2.5-flash`, `flash`, `pro`)
// - arbitrary custom Vertex AI model ID
func ResolveModelArg(arg string, models []config.ModelInfo) string {
	trimmed := strings.TrimSpace(arg)
	if trimmed == "" {
		return ""
	}
	if idx, err := strconv.Atoi(trimmed); err == nil && idx >= 1 && idx <= len(models) {
		return models[idx-1].ID
	}
	return config.NormalizeModelID(trimmed, models)
}
