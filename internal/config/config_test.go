package config

import (
	"path/filepath"
	"testing"
)

func TestNormalizeModelID(t *testing.T) {
	catalog := DefaultModels()

	tests := []struct {
		input string
		want  string
	}{
		{"gemini-2.5-pro", "gemini-2.5-pro"},
		{"GEMINI-2.5-FLASH", "gemini-2.5-flash"},
		{"pro", "gemini-2.5-pro"},
		{"flash", "gemini-2.5-flash"},
		{"flash-lite", "gemini-2.5-flash-lite"},
		{"2.5-pro", "gemini-2.5-pro"},
		{"2.0-flash", "gemini-2.0-flash"},
		{"publishers/google/models/gemini-2.5-flash", "gemini-2.5-flash"},
		{"projects/my-proj/locations/us-central1/publishers/google/models/gemini-2.5-pro", "gemini-2.5-pro"},
		{"models/gemini-1.5-pro", "gemini-1.5-pro"},
		{"claude-3-7-sonnet@20250219", "claude-3-7-sonnet@20250219"},
		{"  gemini-2.5-flash  ", "gemini-2.5-flash"},
		{"", ""},
	}

	for _, tc := range tests {
		got := NormalizeModelID(tc.input, catalog)
		if got != tc.want {
			t.Errorf("NormalizeModelID(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestFindModel(t *testing.T) {
	catalog := DefaultModels()

	info, known := FindModel("flash", catalog)
	if !known {
		t.Fatalf("expected 'flash' alias to be recognized as known preset")
	}
	if info.ID != "gemini-2.5-flash" {
		t.Fatalf("expected ID 'gemini-2.5-flash', got %q", info.ID)
	}

	custom, known := FindModel("custom-tuned-model-001", catalog)
	if known {
		t.Fatalf("expected custom model to report known=false")
	}
	if custom.ID != "custom-tuned-model-001" {
		t.Fatalf("expected custom model ID 'custom-tuned-model-001', got %q", custom.ID)
	}
}

func TestFilterModels(t *testing.T) {
	catalog := DefaultModels()

	all := FilterModels(catalog, "")
	if len(all) != len(catalog) {
		t.Fatalf("expected %d models for empty query, got %d", len(catalog), len(all))
	}

	proOnly := FilterModels(catalog, "2.5-pro")
	if len(proOnly) != 1 || proOnly[0].ID != "gemini-2.5-pro" {
		t.Fatalf("unexpected filter result for '2.5-pro': %+v", proOnly)
	}

	liteModels := FilterModels(catalog, "lite")
	if len(liteModels) < 2 {
		t.Fatalf("expected at least 2 lite models, got %d", len(liteModels))
	}
}

func TestMergeModels(t *testing.T) {
	base := DefaultModels()
	discovered := []ModelInfo{
		{ID: "publishers/google/models/gemini-2.5-pro", Source: SourceModelGarden, LaunchStage: "GA"},
		{ID: " publishers/google/models/gemini-exp-1206 ", DisplayName: "Gemini Exp 1206", Source: SourceModelGarden},
		{ID: "publishers/anthropic/models/claude-3-7-sonnet@20250219", Source: SourceModelGarden},
	}

	merged := MergeModels(base, discovered)
	if len(merged) != len(base)+1 {
		t.Fatalf("expected %d merged models, got %d", len(base)+1, len(merged))
	}
	foundExp := false
	for _, m := range merged {
		if m.ID == "gemini-exp-1206" {
			foundExp = true
		}
		if m.ID == "gemini-2.5-pro" && m.Source != SourceModelGarden {
			t.Fatalf("expected gemini-2.5-pro source to be updated to %q, got %q", SourceModelGarden, m.Source)
		}
	}
	if !foundExp {
		t.Fatalf("expected merged models to include 'gemini-exp-1206'")
	}
}

func TestConfigSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg := &Config{
		Project:        "test-gcp-project",
		Location:       "asia-northeast1",
		Model:          "gemini-2.5-flash",
		ConfigFilePath: cfgPath,
	}
	if err := cfg.SaveToFile(cfgPath); err != nil {
		t.Fatalf("SaveToFile failed: %v", err)
	}

	loaded := &Config{}
	if err := loaded.LoadFromFile(cfgPath); err != nil {
		t.Fatalf("LoadFromFile failed: %v", err)
	}
	if loaded.Project != "test-gcp-project" || loaded.Location != "asia-northeast1" || loaded.Model != "gemini-2.5-flash" {
		t.Fatalf("unexpected loaded config: %+v", loaded)
	}
}
