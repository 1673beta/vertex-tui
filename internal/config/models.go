package config

import (
	"fmt"
	"strings"
)

// ModelInfo describes a Vertex AI (Agent Platform) model available in the TUI.
type ModelInfo struct {
	ID               string   `json:"id"`
	DisplayName      string   `json:"display_name"`
	Provider         string   `json:"provider"`
	Description      string   `json:"description"`
	ContextWindow    string   `json:"context_window"`
	MaxOutput        string   `json:"max_output"`
	SupportsThinking bool     `json:"supports_thinking"`
	SupportsTools    bool     `json:"supports_tools"`
	Recommended      bool     `json:"recommended"`
	Aliases          []string `json:"aliases,omitempty"`
}

// DefaultModelID is the default Vertex AI model used for coding tasks.
const DefaultModelID = "gemini-2.5-pro"

// DefaultLocation is the default Google Cloud Vertex AI location.
const DefaultLocation = "us-central1"

// DefaultModels returns the curated catalog of Google Cloud Vertex AI (Agent Platform)
// models optimized for coding and agentic workflows.
func DefaultModels() []ModelInfo {
	return []ModelInfo{
		{
			ID:               "gemini-2.5-pro",
			DisplayName:      "Gemini 2.5 Pro",
			Provider:         "Google / Vertex AI",
			Description:      "最先端のコーディング・複雑な推論・マルチファイル編集向けフラッグシップモデル (Best for complex coding & reasoning)",
			ContextWindow:    "1M tokens",
			MaxOutput:        "65K tokens",
			SupportsThinking: true,
			SupportsTools:    true,
			Recommended:      true,
			Aliases:          []string{"pro", "2.5-pro", "gemini-pro"},
		},
		{
			ID:               "gemini-2.5-flash",
			DisplayName:      "Gemini 2.5 Flash",
			Provider:         "Google / Vertex AI",
			Description:      "高速・高精度な次世代エージェント＆コーディングモデル (Fast & balanced agentic coding)",
			ContextWindow:    "1M tokens",
			MaxOutput:        "65K tokens",
			SupportsThinking: true,
			SupportsTools:    true,
			Recommended:      true,
			Aliases:          []string{"flash", "2.5-flash", "gemini-flash"},
		},
		{
			ID:               "gemini-2.5-flash-lite",
			DisplayName:      "Gemini 2.5 Flash-Lite",
			Provider:         "Google / Vertex AI",
			Description:      "超低遅延・高スループットな軽量モデル (Ultra-fast, low-latency tasks)",
			ContextWindow:    "1M tokens",
			MaxOutput:        "65K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			Recommended:      false,
			Aliases:          []string{"flash-lite", "2.5-flash-lite", "lite"},
		},
		{
			ID:               "gemini-2.0-flash",
			DisplayName:      "Gemini 2.0 Flash",
			Provider:         "Google / Vertex AI",
			Description:      "マルチモーダル・ツール実行に最適化された高速モデル (General purpose fast agent model)",
			ContextWindow:    "1M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			Recommended:      false,
			Aliases:          []string{"2.0-flash"},
		},
		{
			ID:               "gemini-2.0-flash-lite",
			DisplayName:      "Gemini 2.0 Flash-Lite",
			Provider:         "Google / Vertex AI",
			Description:      "コスト効率と速度を重視した軽量モデル (Cost-efficient high-speed model)",
			ContextWindow:    "1M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			Recommended:      false,
			Aliases:          []string{"2.0-flash-lite", "2.0-lite"},
		},
		{
			ID:               "gemini-1.5-pro",
			DisplayName:      "Gemini 1.5 Pro",
			Provider:         "Google / Vertex AI",
			Description:      "2Mトークンの超長文コンテキスト対応モデル (2M token context window for large codebases)",
			ContextWindow:    "2M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			Recommended:      false,
			Aliases:          []string{"1.5-pro"},
		},
		{
			ID:               "gemini-1.5-flash",
			DisplayName:      "Gemini 1.5 Flash",
			Provider:         "Google / Vertex AI",
			Description:      "長文コンテキスト対応の高速モデル (Fast 1M context window model)",
			ContextWindow:    "1M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			Recommended:      false,
			Aliases:          []string{"1.5-flash"},
		},
	}
}

// NormalizeModelID normalizes a Vertex AI model identifier or alias.
// If the input matches a known preset ID or alias (case-insensitive), it returns the canonical model ID.
// If it is a full Vertex publisher path like "publishers/google/models/gemini-2.5-pro", it strips
// the standard google publisher prefix or preserves custom endpoints/publishers as appropriate.
func NormalizeModelID(input string, catalog []ModelInfo) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if len(catalog) == 0 {
		catalog = DefaultModels()
	}

	// Strip standard "publishers/google/models/" or "models/" prefix if it matches a known model
	shortCandidate := trimmed
	if idx := strings.LastIndex(trimmed, "publishers/google/models/"); idx != -1 {
		shortCandidate = trimmed[idx+len("publishers/google/models/"):]
	} else if strings.HasPrefix(trimmed, "models/") {
		shortCandidate = strings.TrimPrefix(trimmed, "models/")
	}

	lowerCandidate := strings.ToLower(shortCandidate)
	for _, m := range catalog {
		if strings.ToLower(m.ID) == lowerCandidate || strings.ToLower(m.DisplayName) == lowerCandidate {
			return m.ID
		}
		for _, alias := range m.Aliases {
			if strings.ToLower(alias) == lowerCandidate {
				return m.ID
			}
		}
	}

	return shortCandidate
}

// FindModel looks up a ModelInfo by ID or alias from the provided catalog (or DefaultModels if empty).
// If not found in the catalog, it returns a synthesized ModelInfo for custom Vertex AI models
// alongside false for knownPreset.
func FindModel(modelID string, catalog []ModelInfo) (ModelInfo, bool) {
	if len(catalog) == 0 {
		catalog = DefaultModels()
	}
	canonical := NormalizeModelID(modelID, catalog)
	for _, m := range catalog {
		if strings.EqualFold(m.ID, canonical) {
			return m, true
		}
	}
	return ModelInfo{
		ID:               canonical,
		DisplayName:      canonical,
		Provider:         "Vertex AI (Custom / Model Garden)",
		Description:      "カスタム指定されたVertex AIモデル (Custom Vertex AI model endpoint)",
		ContextWindow:    "Custom",
		MaxOutput:        "Custom",
		SupportsThinking: strings.Contains(strings.ToLower(canonical), "2.5") || strings.Contains(strings.ToLower(canonical), "thinking"),
		SupportsTools:    true,
	}, false
}

// FilterModels filters a slice of ModelInfo by a search query across ID, DisplayName, Description, Provider, and Aliases.
func FilterModels(models []ModelInfo, query string) []ModelInfo {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		out := make([]ModelInfo, len(models))
		copy(out, models)
		return out
	}

	var matched []ModelInfo
	for _, m := range models {
		if strings.Contains(strings.ToLower(m.ID), q) ||
			strings.Contains(strings.ToLower(m.DisplayName), q) ||
			strings.Contains(strings.ToLower(m.Description), q) ||
			strings.Contains(strings.ToLower(m.Provider), q) {
			matched = append(matched, m)
			continue
		}
		for _, alias := range m.Aliases {
			if strings.Contains(strings.ToLower(alias), q) {
				matched = append(matched, m)
				break
			}
		}
	}
	return matched
}

// MergeModels merges dynamically discovered Vertex AI models with the default catalog without duplicates.
func MergeModels(base []ModelInfo, discovered []ModelInfo) []ModelInfo {
	seen := make(map[string]bool, len(base))
	result := make([]ModelInfo, 0, len(base)+len(discovered))
	for _, m := range base {
		key := strings.ToLower(m.ID)
		if !seen[key] {
			seen[key] = true
			result = append(result, m)
		}
	}
	for _, m := range discovered {
		canonical := NormalizeModelID(m.ID, base)
		key := strings.ToLower(canonical)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		if m.DisplayName == "" {
			m.DisplayName = canonical
		}
		if m.Provider == "" {
			m.Provider = "Google / Vertex AI"
		}
		if m.Description == "" {
			m.Description = fmt.Sprintf("Vertex AI model (%s)", canonical)
		}
		m.ID = canonical
		result = append(result, m)
	}
	return result
}
