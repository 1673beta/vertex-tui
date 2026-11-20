package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/httptransport"
	"google.golang.org/genai"

	"vertex-tui/internal/config"
)

// ModelGardenListOptions controls how models are fetched from Vertex AI Model Garden.
type ModelGardenListOptions struct {
	// Publishers specifies which Vertex AI Model Garden publishers to query (e.g. ["google", "anthropic", "meta"]).
	// If empty, config.DefaultModelGardenPublishers is used.
	Publishers []string
	// IncludeAll includes non-LLM / vision / embedding models from Model Garden in addition to generative coding models.
	IncludeAll bool
}

// publisherModelsAPIResponse represents the JSON structure returned by
// GET https://{location}-aiplatform.googleapis.com/v1beta1/publishers/{publisher}/models
type publisherModelsAPIResponse struct {
	PublisherModels []publisherModelEntry `json:"publisherModels"`
	NextPageToken   string                `json:"nextPageToken"`
}

type publisherModelEntry struct {
	Name                   string         `json:"name"`
	VersionID              string         `json:"versionId"`
	OpenSourceCategory     string         `json:"openSourceCategory"`
	LaunchStage            string         `json:"launchStage"`
	VersionState           string         `json:"versionState"`
	PublisherModelTemplate string         `json:"publisherModelTemplate"`
	DisplayName            string         `json:"displayName"`
	Description            string         `json:"description"`
	SupportedActions       map[string]any `json:"supportedActions"`
}

// FetchModelGardenModels queries Vertex AI Model Garden via:
//  1. Google GenAI SDK `client.Models.All(ctx)` (for `publishers/google/models`)
//  2. Google GenAI SDK `client.Models.List(ctx, &genai.ListModelsConfig{QueryBase: genai.Ptr(false)})` (for project-tuned models)
//  3. Vertex AI Model Garden REST API `GET /v1beta1/publishers/{publisher}/models` across requested publishers
//     (e.g. google, anthropic, meta, mistralai, deepseek-ai, qwen, cohere) to retrieve full Model Garden metadata
//     (launchStage, versionId, publisherModelTemplate, partner models).
func (v *VertexBackend) FetchModelGardenModels(ctx context.Context, opts ModelGardenListOptions) ([]config.ModelInfo, error) {
	publishers := opts.Publishers
	if len(publishers) == 0 {
		publishers = config.DefaultModelGardenPublishers
	}

	var (
		mu         sync.Mutex
		discovered []config.ModelInfo
		firstErr   error
	)

	addModel := func(m config.ModelInfo) {
		enriched := config.EnrichModelGardenInfo(m)
		if !opts.IncludeAll && !enriched.IsGenerativeLLM {
			return
		}
		mu.Lock()
		discovered = append(discovered, enriched)
		mu.Unlock()
	}

	// 1. Query via official Google GenAI SDK client.Models.All(ctx) (publishers/google/models)
	if v.client != nil && v.client.Models != nil && shouldQueryPublisher(publishers, "google") {
		for m, err := range v.client.Models.All(ctx) {
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				break
			}
			if m == nil || m.Name == "" {
				continue
			}
			addModel(genaiModelToModelInfo(m, config.SourceModelGarden))
		}

		// 2. Also check project-scoped tuned/deployed models via QueryBase=false if Project is set
		cc := v.client.ClientConfig()
		if cc.Project != "" && cc.Location != "" {
			if page, err := v.client.Models.List(ctx, &genai.ListModelsConfig{QueryBase: genai.Ptr(false)}); err == nil {
				for _, m := range page.Items {
					if m == nil || m.Name == "" {
						continue
					}
					info := genaiModelToModelInfo(m, config.SourceVertexTuned)
					info.IsGenerativeLLM = true
					addModel(info)
				}
			}
		}
	}

	// 3. Query Vertex AI Model Garden Publisher Models API (v1beta1/publishers/{publisher}/models)
	// in parallel across requested publishers for rich metadata & partner models (Anthropic, Meta, Mistral, DeepSeek, Qwen, etc.)
	if v.httpClient != nil && v.baseURL != "" {
		var wg sync.WaitGroup
		for _, pub := range publishers {
			pub := strings.TrimSpace(pub)
			if pub == "" {
				continue
			}
			wg.Add(1)
			go func(publisher string) {
				defer wg.Done()
				models, err := v.fetchPublisherModelsREST(ctx, publisher)
				if err != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					mu.Unlock()
					return
				}
				for _, m := range models {
					addModel(m)
				}
			}(pub)
		}
		wg.Wait()
	}

	if len(discovered) == 0 && firstErr != nil {
		return nil, firstErr
	}

	// Deduplicate and sort discovered Model Garden models
	return config.MergeModels(nil, discovered), nil
}

func (v *VertexBackend) fetchPublisherModelsREST(ctx context.Context, publisher string) ([]config.ModelInfo, error) {
	base := strings.TrimRight(v.baseURL, "/")
	apiVersion := v.apiVersion
	if apiVersion == "" {
		apiVersion = "v1beta1"
	}

	endpoint := fmt.Sprintf("%s/%s/publishers/%s/models", base, apiVersion, url.PathEscape(publisher))
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	var results []config.ModelInfo
	pageToken := ""
	// Fetch up to 3 pages (300 models per publisher)
	for page := 0; page < 3; page++ {
		q := u.Query()
		q.Set("view", "PUBLISHER_MODEL_VIEW_BASIC")
		q.Set("pageSize", "100")
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return results, err
		}
		if v.apiKey != "" {
			req.Header.Set("x-goog-api-key", v.apiKey)
		}

		resp, err := v.httpClient.Do(req)
		if err != nil {
			return results, err
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return results, readErr
		}
		if resp.StatusCode != http.StatusOK {
			return results, fmt.Errorf("Model Garden API (%s) returned HTTP %d: %s", publisher, resp.StatusCode, truncateErrBody(body))
		}

		var parsed publisherModelsAPIResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return results, err
		}

		for _, entry := range parsed.PublisherModels {
			if entry.Name == "" {
				continue
			}
			pub, canonicalID := config.ParsePublisherModelResource(entry.Name)
			if pub == "" {
				pub = publisher
			}
			desc := entry.Description
			if desc == "" && entry.LaunchStage != "" {
				desc = fmt.Sprintf("Vertex AI Model Garden (%s) [%s]", entry.Name, entry.LaunchStage)
			}
			results = append(results, config.ModelInfo{
				ID:           canonicalID,
				ResourceName: entry.Name,
				DisplayName:  entry.DisplayName,
				Publisher:    pub,
				Description:  desc,
				Version:      entry.VersionID,
				LaunchStage:  entry.LaunchStage,
				Source:       config.SourceModelGarden,
			})
		}

		if parsed.NextPageToken == "" {
			break
		}
		pageToken = parsed.NextPageToken
	}

	return results, nil
}

func genaiModelToModelInfo(m *genai.Model, source string) config.ModelInfo {
	pub, canonicalID := config.ParsePublisherModelResource(m.Name)
	ctxWindow := ""
	if m.InputTokenLimit > 0 {
		ctxWindow = fmt.Sprintf("%d tokens", m.InputTokenLimit)
	}
	maxOut := ""
	if m.OutputTokenLimit > 0 {
		maxOut = fmt.Sprintf("%d tokens", m.OutputTokenLimit)
	}
	return config.ModelInfo{
		ID:               canonicalID,
		ResourceName:     m.Name,
		DisplayName:      m.DisplayName,
		Publisher:        pub,
		Description:      m.Description,
		Version:          m.Version,
		Source:           source,
		ContextWindow:    ctxWindow,
		MaxOutput:        maxOut,
		SupportsThinking: m.Thinking,
		SupportsTools:    true,
	}
}

func shouldQueryPublisher(publishers []string, target string) bool {
	for _, p := range publishers {
		if strings.EqualFold(strings.TrimSpace(p), target) {
			return true
		}
	}
	return false
}

func truncateErrBody(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 200 {
		return s[:200] + "..."
	}
	return s
}

func buildVertexHTTPClient(ctx context.Context, cfg *config.Config) (*http.Client, string, error) {
	baseURL := "https://aiplatform.googleapis.com"
	loc := strings.TrimSpace(cfg.Location)
	if loc != "" && loc != "global" && cfg.APIKey == "" {
		if loc == "us" || loc == "eu" {
			baseURL = fmt.Sprintf("https://aiplatform.%s.rep.googleapis.com", loc)
		} else {
			baseURL = fmt.Sprintf("https://%s-aiplatform.googleapis.com", loc)
		}
	}

	if cfg.APIKey != "" && cfg.Project == "" {
		return &http.Client{}, baseURL, nil
	}

	cred, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
	})
	if err != nil {
		return nil, baseURL, err
	}

	headers := http.Header{}
	if qp, err := cred.QuotaProjectID(ctx); err == nil && qp != "" {
		headers.Set("X-Goog-User-Project", qp)
	} else if cfg.Project != "" {
		headers.Set("X-Goog-User-Project", cfg.Project)
	}

	client, err := httptransport.NewClient(&httptransport.Options{
		Credentials: cred,
		Headers:     headers,
	})
	if err != nil {
		return nil, baseURL, err
	}
	_ = auth.Credentials{}
	return client, baseURL, nil
}
