package config

import (
	"fmt"
	"sort"
	"strings"
)

// ModelSource indicates where a model entry originated.
const (
	SourceModelGarden = "Model Garden"
	SourceVertexTuned = "Vertex AI Tuned"
	SourceFallback    = "FallbackPreset"
	SourceCustom      = "Custom"
)

// DefaultModelGardenPublishers lists the Vertex AI Model Garden publishers queried by default.
var DefaultModelGardenPublishers = []string{
	"google",
	"anthropic",
	"meta",
	"mistralai",
	"deepseek-ai",
	"qwen",
	"cohere",
}

// ModelInfo describes a Vertex AI / Model Garden model available in the TUI.
type ModelInfo struct {
	ID               string   `json:"id"`
	ResourceName     string   `json:"resource_name,omitempty"`
	DisplayName      string   `json:"display_name"`
	Publisher        string   `json:"publisher"`
	Provider         string   `json:"provider"`
	Description      string   `json:"description"`
	Version          string   `json:"version,omitempty"`
	LaunchStage      string   `json:"launch_stage,omitempty"`
	Source           string   `json:"source,omitempty"`
	ContextWindow    string   `json:"context_window"`
	MaxOutput        string   `json:"max_output"`
	SupportsThinking bool     `json:"supports_thinking"`
	SupportsTools    bool     `json:"supports_tools"`
	IsGenerativeLLM  bool     `json:"is_generative_llm"`
	Recommended      bool     `json:"recommended"`
	Aliases          []string `json:"aliases,omitempty"`
}

// DefaultModelID is the default Vertex AI model used for coding tasks.
const DefaultModelID = "gemini-2.5-pro"

// DefaultLocation is the default Google Cloud Vertex AI location.
const DefaultLocation = "us-central1"

// FallbackModels returns the seed/fallback catalog of Vertex AI Model Garden models.
// When connected to Google Cloud, vertex-tui dynamically fetches the live model list from
// the Vertex AI Model Garden API (publishers/*/models) and uses these metadata definitions
// to enrich raw API responses (or as an offline fallback before authentication).
func FallbackModels() []ModelInfo {
	return []ModelInfo{
		{
			ID:               "gemini-2.5-pro",
			ResourceName:     "publishers/google/models/gemini-2.5-pro",
			DisplayName:      "Gemini 2.5 Pro",
			Publisher:        "google",
			Provider:         "Google (Model Garden)",
			Description:      "最先端のコーディング・複雑な推論・マルチファイル編集向けフラッグシップモデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "1M tokens",
			MaxOutput:        "65K tokens",
			SupportsThinking: true,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      true,
			Aliases:          []string{"pro", "2.5-pro", "gemini-pro"},
		},
		{
			ID:               "gemini-2.5-flash",
			ResourceName:     "publishers/google/models/gemini-2.5-flash",
			DisplayName:      "Gemini 2.5 Flash",
			Publisher:        "google",
			Provider:         "Google (Model Garden)",
			Description:      "高速・高精度な次世代エージェント＆コーディングモデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "1M tokens",
			MaxOutput:        "65K tokens",
			SupportsThinking: true,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      true,
			Aliases:          []string{"flash", "2.5-flash", "gemini-flash"},
		},
		{
			ID:               "gemini-2.5-flash-lite",
			ResourceName:     "publishers/google/models/gemini-2.5-flash-lite",
			DisplayName:      "Gemini 2.5 Flash-Lite",
			Publisher:        "google",
			Provider:         "Google (Model Garden)",
			Description:      "超低遅延・高スループットな軽量モデル",
			LaunchStage:      "PUBLIC_PREVIEW",
			Source:           SourceFallback,
			ContextWindow:    "1M tokens",
			MaxOutput:        "65K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"flash-lite", "2.5-flash-lite", "lite"},
		},
		{
			ID:               "gemini-2.0-flash",
			ResourceName:     "publishers/google/models/gemini-2.0-flash",
			DisplayName:      "Gemini 2.0 Flash",
			Publisher:        "google",
			Provider:         "Google (Model Garden)",
			Description:      "マルチモーダル・ツール実行に最適化された高速モデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "1M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"2.0-flash", "gemini-2.0-flash-001"},
		},
		{
			ID:               "gemini-2.0-flash-lite",
			ResourceName:     "publishers/google/models/gemini-2.0-flash-lite",
			DisplayName:      "Gemini 2.0 Flash-Lite",
			Publisher:        "google",
			Provider:         "Google (Model Garden)",
			Description:      "コスト効率と速度を重視した軽量モデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "1M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"2.0-flash-lite", "2.0-lite", "gemini-2.0-flash-lite-001"},
		},
		{
			ID:               "gemini-1.5-pro",
			ResourceName:     "publishers/google/models/gemini-1.5-pro",
			DisplayName:      "Gemini 1.5 Pro",
			Publisher:        "google",
			Provider:         "Google (Model Garden)",
			Description:      "2Mトークンの超長文コンテキスト対応モデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "2M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"1.5-pro", "gemini-1.5-pro-002"},
		},
		{
			ID:               "gemini-1.5-flash",
			ResourceName:     "publishers/google/models/gemini-1.5-flash",
			DisplayName:      "Gemini 1.5 Flash",
			Publisher:        "google",
			Provider:         "Google (Model Garden)",
			Description:      "長文コンテキスト対応の高速モデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "1M tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"1.5-flash", "gemini-1.5-flash-002"},
		},
		{
			ID:               "anthropic/claude-3-7-sonnet@20250219",
			ResourceName:     "publishers/anthropic/models/claude-3-7-sonnet@20250219",
			DisplayName:      "Claude 3.7 Sonnet (Model Garden)",
			Publisher:        "anthropic",
			Provider:         "Anthropic (Model Garden)",
			Description:      "Vertex AI Model Garden の Anthropic Claude 3.7 Sonnet コーディング・推論モデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "200K tokens",
			MaxOutput:        "64K tokens",
			SupportsThinking: true,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      true,
			Aliases:          []string{"claude-3-7-sonnet", "claude-sonnet", "claude"},
		},
		{
			ID:               "anthropic/claude-3-5-sonnet-v2@20241022",
			ResourceName:     "publishers/anthropic/models/claude-3-5-sonnet-v2@20241022",
			DisplayName:      "Claude 3.5 Sonnet v2 (Model Garden)",
			Publisher:        "anthropic",
			Provider:         "Anthropic (Model Garden)",
			Description:      "Vertex AI Model Garden の Anthropic Claude 3.5 Sonnet v2 モデル",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "200K tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"claude-3-5-sonnet", "claude-3-5-sonnet-v2"},
		},
		{
			ID:               "meta/llama-3.3-70b-instruct-maas",
			ResourceName:     "publishers/meta/models/llama-3.3-70b-instruct-maas",
			DisplayName:      "Llama 3.3 70B Instruct MaaS (Model Garden)",
			Publisher:        "meta",
			Provider:         "Meta (Model Garden MaaS)",
			Description:      "Vertex AI Model Garden の Meta Llama 3.3 70B マネージドAPI (MaaS)",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "128K tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"llama-3.3-70b", "llama-3.3", "llama"},
		},
		{
			ID:               "mistralai/codestral-2501",
			ResourceName:     "publishers/mistralai/models/codestral-2501",
			DisplayName:      "Codestral 25.01 (Model Garden)",
			Publisher:        "mistralai",
			Provider:         "Mistral AI (Model Garden)",
			Description:      "Vertex AI Model Garden のコード生成特化モデル Codestral",
			LaunchStage:      "GA",
			Source:           SourceFallback,
			ContextWindow:    "256K tokens",
			MaxOutput:        "8K tokens",
			SupportsThinking: false,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"codestral"},
		},
		{
			ID:               "deepseek-ai/deepseek-r1-0528-maas",
			ResourceName:     "publishers/deepseek-ai/models/deepseek-r1-0528-maas",
			DisplayName:      "DeepSeek R1 MaaS (Model Garden)",
			Publisher:        "deepseek-ai",
			Provider:         "DeepSeek (Model Garden MaaS)",
			Description:      "Vertex AI Model Garden の DeepSeek-R1 推論・コーディング MaaS モデル",
			LaunchStage:      "PUBLIC_PREVIEW",
			Source:           SourceFallback,
			ContextWindow:    "128K tokens",
			MaxOutput:        "32K tokens",
			SupportsThinking: true,
			SupportsTools:    true,
			IsGenerativeLLM:  true,
			Recommended:      false,
			Aliases:          []string{"deepseek-r1", "deepseek"},
		},
	}
}

// DefaultModels is an alias for FallbackModels for backwards compatibility.
func DefaultModels() []ModelInfo {
	return FallbackModels()
}

// ParsePublisherModelResource extracts the publisher and short model ID from a Vertex AI Model Garden
// resource name like "publishers/google/models/gemini-2.5-pro" or
// "projects/p/locations/l/publishers/anthropic/models/claude-3-7-sonnet".
// For google models, canonical ID is "gemini-2.5-pro".
// For third-party publishers (e.g. anthropic, meta, mistralai), canonical ID is "<publisher>/<model>"
// which google.golang.org/genai automatically expands to "publishers/<publisher>/models/<model>".
func ParsePublisherModelResource(resourceName string) (publisher string, canonicalID string) {
	trimmed := strings.TrimSpace(resourceName)
	if trimmed == "" {
		return "", ""
	}

	// Check for "publishers/<pub>/models/<id>"
	if idx := strings.LastIndex(trimmed, "publishers/"); idx != -1 {
		rest := trimmed[idx+len("publishers/"):]
		parts := strings.SplitN(rest, "/models/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			pub := parts[0]
			modelID := parts[1]
			if pub == "google" {
				return pub, modelID
			}
			return pub, fmt.Sprintf("%s/%s", pub, modelID)
		}
	}

	if strings.HasPrefix(trimmed, "models/") {
		return "google", strings.TrimPrefix(trimmed, "models/")
	}

	if strings.Contains(trimmed, "/") && !strings.HasPrefix(trimmed, "projects/") {
		parts := strings.SplitN(trimmed, "/", 2)
		if parts[0] == "google" {
			return "google", parts[1]
		}
		return parts[0], trimmed
	}

	return "google", trimmed
}

// NormalizeModelID normalizes a Vertex AI / Model Garden model identifier or alias.
func NormalizeModelID(input string, catalog []ModelInfo) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return ""
	}
	if len(catalog) == 0 {
		catalog = FallbackModels()
	}

	_, shortCandidate := ParsePublisherModelResource(trimmed)
	if shortCandidate == "" {
		shortCandidate = trimmed
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

// IsGenerativeCodingModel determines whether a Model Garden model ID / description represents
// a generative LLM / coding model suitable for interactive agent use (as opposed to vision-only
// backbones like ResNet, YOLO, ViT, BERT, or pure embedding models).
func IsGenerativeCodingModel(id string, displayName string, description string) bool {
	combined := strings.ToLower(id + " " + displayName + " " + description)

	// Exclude pure embedding, vision classification/detection, speech-only, or legacy non-chat models
	excludeKeywords := []string{
		"embedding", "gecko", "imagen", "chirp", "resnet", "bert", "roberta",
		"vit-", "/vit", "yolo", "detr", "deeplab", "maskrcnn", "efficientnet",
		"mobilenet", "swin", "blip", "clip", "sam-", "segment-anything",
		"stable-diffusion", "sdxl", "veo-", "lyria", "aqa", "bison", "unicorn",
		"codechat-bison", "chat-bison", "text-bison", "code-bison",
	}
	for _, kw := range excludeKeywords {
		if strings.Contains(combined, kw) {
			return false
		}
	}

	// Include known generative & coding model families in Model Garden
	includeKeywords := []string{
		"gemini", "gemma", "codegemma", "claude", "llama", "codellama",
		"mistral", "mixtral", "codestral", "ministral", "pixtral",
		"deepseek", "qwen", "command", "jamba", "starcoder", " falcon",
		"phi-", "yi-", "nemotron", "instruct", "chat", "coder",
	}
	for _, kw := range includeKeywords {
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

// EnrichModelGardenInfo enriches a raw ModelInfo discovered from the Vertex AI Model Garden API
// with known preset metadata (descriptions, token limits, aliases, thinking support) and provider labels.
func EnrichModelGardenInfo(m ModelInfo) ModelInfo {
	pub, canonical := ParsePublisherModelResource(firstNonEmptyStr(m.ResourceName, m.ID))
	if canonical != "" {
		m.ID = canonical
	}
	if m.Publisher == "" {
		m.Publisher = pub
	}
	if m.ResourceName == "" {
		if m.Publisher != "" && !strings.Contains(m.ID, "/") {
			m.ResourceName = fmt.Sprintf("publishers/%s/models/%s", m.Publisher, m.ID)
		} else if strings.Contains(m.ID, "/") {
			parts := strings.SplitN(m.ID, "/", 2)
			m.ResourceName = fmt.Sprintf("publishers/%s/models/%s", parts[0], parts[1])
		}
	}
	if m.Source == "" {
		m.Source = SourceModelGarden
	}

	// Match against known seed metadata for richer description & context window
	for _, preset := range FallbackModels() {
		if strings.EqualFold(preset.ID, m.ID) || sliceContainsFold(preset.Aliases, m.ID) || strings.HasPrefix(strings.ToLower(m.ID), strings.ToLower(preset.ID)) {
			if m.DisplayName == "" || m.DisplayName == m.ID || strings.HasPrefix(m.DisplayName, "publishers/") {
				if strings.EqualFold(preset.ID, m.ID) {
					m.DisplayName = preset.DisplayName
				} else {
					m.DisplayName = fmt.Sprintf("%s (%s)", preset.DisplayName, m.ID)
				}
			}
			if m.Description == "" || strings.HasPrefix(m.Description, "Vertex AI model") {
				m.Description = preset.Description
			}
			if m.ContextWindow == "" || m.ContextWindow == "Vertex AI" {
				m.ContextWindow = preset.ContextWindow
			}
			if m.MaxOutput == "" || m.MaxOutput == "Vertex AI" {
				m.MaxOutput = preset.MaxOutput
			}
			if strings.EqualFold(preset.ID, m.ID) && len(m.Aliases) == 0 {
				m.Aliases = preset.Aliases
				m.Recommended = preset.Recommended
			}
			m.SupportsThinking = m.SupportsThinking || preset.SupportsThinking
			m.SupportsTools = true
			m.IsGenerativeLLM = true
			break
		}
	}

	if m.DisplayName == "" || strings.HasPrefix(m.DisplayName, "publishers/") {
		m.DisplayName = formatHumanModelName(m.ID)
	}
	if m.Provider == "" {
		m.Provider = formatPublisherProvider(m.Publisher, m.Source)
	}
	if m.ContextWindow == "" {
		m.ContextWindow = "Model Garden"
	}
	if m.MaxOutput == "" {
		m.MaxOutput = "Model Garden"
	}
	lowerID := strings.ToLower(m.ID)
	if strings.Contains(lowerID, "2.5-pro") || strings.Contains(lowerID, "2.5-flash") ||
		strings.Contains(lowerID, "thinking") || strings.Contains(lowerID, "deepseek-r1") ||
		strings.Contains(lowerID, "claude-3-7") {
		m.SupportsThinking = true
	}
	m.SupportsTools = true
	if !m.IsGenerativeLLM {
		m.IsGenerativeLLM = IsGenerativeCodingModel(m.ID, m.DisplayName, m.Description)
	}
	if m.Description == "" {
		stage := ""
		if m.LaunchStage != "" {
			stage = fmt.Sprintf(" [%s]", m.LaunchStage)
		}
		m.Description = fmt.Sprintf("Vertex AI Model Garden (%s)%s", m.ResourceName, stage)
	}
	return m
}

func formatPublisherProvider(publisher string, source string) string {
	switch strings.ToLower(publisher) {
	case "google":
		return fmt.Sprintf("Google (%s)", firstNonEmptyStr(source, SourceModelGarden))
	case "anthropic":
		return "Anthropic (Model Garden)"
	case "meta":
		return "Meta (Model Garden)"
	case "mistralai":
		return "Mistral AI (Model Garden)"
	case "deepseek-ai":
		return "DeepSeek (Model Garden)"
	case "qwen":
		return "Qwen (Model Garden)"
	case "cohere":
		return "Cohere (Model Garden)"
	default:
		if publisher != "" {
			return fmt.Sprintf("%s (Model Garden)", publisher)
		}
		return "Vertex AI Model Garden"
	}
}

func formatHumanModelName(id string) string {
	short := id
	if idx := strings.Index(short, "/"); idx != -1 {
		short = short[idx+1:]
	}
	parts := strings.Split(short, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func sliceContainsFold(list []string, target string) bool {
	for _, item := range list {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}

// FindModel looks up a ModelInfo by ID or alias from the provided catalog (or FallbackModels if empty).
func FindModel(modelID string, catalog []ModelInfo) (ModelInfo, bool) {
	if len(catalog) == 0 {
		catalog = FallbackModels()
	}
	canonical := NormalizeModelID(modelID, catalog)
	for _, m := range catalog {
		if strings.EqualFold(m.ID, canonical) {
			return m, true
		}
	}
	custom := EnrichModelGardenInfo(ModelInfo{
		ID:              canonical,
		Source:          SourceCustom,
		IsGenerativeLLM: true,
	})
	return custom, false
}

// FilterModels filters a slice of ModelInfo by a search query across ID, DisplayName, Description,
// Publisher, Provider, LaunchStage, and Aliases.
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
			strings.Contains(strings.ToLower(m.Provider), q) ||
			strings.Contains(strings.ToLower(m.Publisher), q) ||
			strings.Contains(strings.ToLower(m.LaunchStage), q) {
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

// MergeModels merges dynamically discovered Vertex AI Model Garden models with an existing catalog.
// If discovered models match existing entries, live Model Garden fields (Source, Version, LaunchStage, ResourceName)
// update the entry in place so it reflects live Model Garden state.
func MergeModels(base []ModelInfo, discovered []ModelInfo) []ModelInfo {
	indexByKey := make(map[string]int, len(base)+len(discovered))
	result := make([]ModelInfo, 0, len(base)+len(discovered))

	for _, m := range base {
		enriched := EnrichModelGardenInfo(m)
		key := strings.ToLower(enriched.ID)
		if key == "" {
			continue
		}
		if _, exists := indexByKey[key]; !exists {
			indexByKey[key] = len(result)
			result = append(result, enriched)
		}
	}

	for _, raw := range discovered {
		enriched := EnrichModelGardenInfo(raw)
		canonical := NormalizeModelID(enriched.ID, base)
		if canonical != "" {
			enriched.ID = canonical
			enriched = EnrichModelGardenInfo(enriched)
		}
		key := strings.ToLower(enriched.ID)
		if key == "" {
			continue
		}
		if idx, exists := indexByKey[key]; exists {
			existing := result[idx]
			if enriched.Source == SourceModelGarden || enriched.Source == SourceVertexTuned {
				existing.Source = enriched.Source
				existing.Provider = formatPublisherProvider(existing.Publisher, enriched.Source)
			}
			if enriched.Version != "" {
				existing.Version = enriched.Version
			}
			if enriched.LaunchStage != "" {
				existing.LaunchStage = enriched.LaunchStage
			}
			if enriched.ResourceName != "" {
				existing.ResourceName = enriched.ResourceName
			}
			result[idx] = existing
			continue
		}
		indexByKey[key] = len(result)
		result = append(result, enriched)
	}

	sort.SliceStable(result, func(i, j int) bool {
		return modelPriorityScore(result[i]) > modelPriorityScore(result[j])
	})

	return result
}

func modelPriorityScore(m ModelInfo) int {
	score := 0
	if m.Recommended {
		score += 1000
	}
	if m.IsGenerativeLLM {
		score += 500
	}
	id := strings.ToLower(m.ID)
	switch {
	case strings.HasPrefix(id, "gemini-2.5-pro"):
		score += 400
	case strings.HasPrefix(id, "gemini-2.5-flash"):
		score += 380
	case strings.HasPrefix(id, "gemini-2.5"):
		score += 360
	case strings.HasPrefix(id, "gemini-2.0"):
		score += 320
	case strings.HasPrefix(id, "gemini-1.5"):
		score += 280
	case strings.HasPrefix(id, "gemini"):
		score += 250
	case strings.Contains(id, "claude-3-7") || strings.Contains(id, "claude-3-5"):
		score += 240
	case strings.Contains(id, "claude"):
		score += 220
	case strings.Contains(id, "llama-3.3") || strings.Contains(id, "llama-3.2") || strings.Contains(id, "llama-3.1"):
		score += 200
	case strings.Contains(id, "codestral") || strings.Contains(id, "deepseek"):
		score += 190
	case strings.Contains(id, "qwen") || strings.Contains(id, "gemma"):
		score += 170
	}
	if m.LaunchStage == "GA" {
		score += 20
	}
	return score
}
