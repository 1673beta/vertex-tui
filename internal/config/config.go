package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Config holds the runtime configuration for the Vertex AI coding agent TUI.
type Config struct {
	Project         string  `json:"project"`
	Location        string  `json:"location"`
	Model           string  `json:"model"`
	APIKey          string  `json:"api_key,omitempty"`
	WorkDir         string  `json:"work_dir"`
	Temperature     float32 `json:"temperature"`
	MaxOutputTokens int32   `json:"max_output_tokens"`
	ConfigFilePath  string  `json:"-"`
}

// DefaultConfig returns a Config populated from environment variables, persisted user config,
// and sensible defaults for Vertex AI.
func DefaultConfig() *Config {
	wd, err := os.Getwd()
	if err != nil || wd == "" {
		wd = "."
	}

	cfg := &Config{
		Location:        DefaultLocation,
		Model:           DefaultModelID,
		WorkDir:         wd,
		Temperature:     0.2,
		MaxOutputTokens: 8192,
	}

	if configPath := defaultConfigFilePath(); configPath != "" {
		cfg.ConfigFilePath = configPath
		_ = cfg.LoadFromFile(configPath)
	}

	// Environment variables take precedence over persisted file config
	if p := firstNonEmptyEnv("GOOGLE_CLOUD_PROJECT", "VERTEX_PROJECT", "GCLOUD_PROJECT", "CLOUDSDK_CORE_PROJECT"); p != "" {
		cfg.Project = p
	}
	if loc := firstNonEmptyEnv("GOOGLE_CLOUD_LOCATION", "GOOGLE_CLOUD_REGION", "VERTEX_LOCATION", "VERTEX_REGION"); loc != "" {
		cfg.Location = loc
	}
	if m := firstNonEmptyEnv("VERTEX_MODEL", "GEMINI_MODEL", "GOOGLE_GENAI_MODEL"); m != "" {
		cfg.Model = NormalizeModelID(m, DefaultModels())
	}
	if key := firstNonEmptyEnv("GOOGLE_API_KEY", "VERTEX_API_KEY", "GEMINI_API_KEY"); key != "" {
		cfg.APIKey = key
	}

	// If project is still empty, try reading from gcloud config file (fast, no network needed)
	if cfg.Project == "" {
		cfg.Project = detectGcloudProject()
	}

	if cfg.Location == "" {
		cfg.Location = DefaultLocation
	}
	if cfg.Model == "" {
		cfg.Model = DefaultModelID
	}

	return cfg
}

// LoadFromFile loads persisted settings (such as the last selected model, project, and location)
// from a JSON file if it exists.
func (c *Config) LoadFromFile(path string) error {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var persisted struct {
		Project  string `json:"project"`
		Location string `json:"location"`
		Model    string `json:"model"`
	}
	if err := json.Unmarshal(data, &persisted); err != nil {
		return err
	}
	if persisted.Project != "" {
		c.Project = persisted.Project
	}
	if persisted.Location != "" {
		c.Location = persisted.Location
	}
	if persisted.Model != "" {
		c.Model = NormalizeModelID(persisted.Model, DefaultModels())
	}
	return nil
}

// SaveToFile persists the current project, location, and model selection to disk.
func (c *Config) SaveToFile(path string) error {
	if path == "" {
		path = c.ConfigFilePath
	}
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	persisted := struct {
		Project  string `json:"project,omitempty"`
		Location string `json:"location,omitempty"`
		Model    string `json:"model"`
	}{
		Project:  c.Project,
		Location: c.Location,
		Model:    c.Model,
	}
	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func defaultConfigFilePath() string {
	if custom := strings.TrimSpace(os.Getenv("VERTEX_TUI_CONFIG")); custom != "" {
		return custom
	}
	configDir, err := os.UserConfigDir()
	if err != nil || configDir == "" {
		return ""
	}
	return filepath.Join(configDir, "vertex-tui", "config.json")
}

func detectGcloudProject() string {
	// First try reading ~/.config/gcloud/configurations/config_default directly (fast & sandbox-friendly)
	home, err := os.UserHomeDir()
	if err == nil && home != "" {
		activeConfig := "default"
		if data, err := os.ReadFile(filepath.Join(home, ".config", "gcloud", "active_config")); err == nil {
			if name := strings.TrimSpace(string(data)); name != "" {
				activeConfig = name
			}
		}
		cfgFile := filepath.Join(home, ".config", "gcloud", "configurations", "config_"+activeConfig)
		if data, err := os.ReadFile(cfgFile); err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "project") && strings.Contains(line, "=") {
					parts := strings.SplitN(line, "=", 2)
					if len(parts) == 2 && strings.TrimSpace(parts[0]) == "project" {
						if p := strings.TrimSpace(parts[1]); p != "" {
							return p
						}
					}
				}
			}
		}
	}

	// Fallback: try gcloud CLI if available
	if path, err := exec.LookPath("gcloud"); err == nil && path != "" {
		out, err := exec.Command(path, "config", "get-value", "project", "--quiet").Output()
		if err == nil {
			p := strings.TrimSpace(string(out))
			if p != "" && p != "(unset)" {
				return p
			}
		}
	}
	return ""
}
