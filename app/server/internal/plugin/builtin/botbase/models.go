package botbase

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type listModelsInput struct {
	Protocol string
	BaseURL  string
	APIKey   string
}

func listRemoteModels(ctx context.Context, in listModelsInput) ([]string, error) {
	protocol := strings.TrimSpace(in.Protocol)
	baseURL := strings.TrimRight(strings.TrimSpace(in.BaseURL), "/")
	apiKey := strings.TrimSpace(in.APIKey)
	if protocol == "" {
		return nil, fmt.Errorf("protocol is required")
	}
	if baseURL == "" {
		return nil, fmt.Errorf("base_url is required")
	}
	if _, err := url.ParseRequestURI(baseURL); err != nil {
		return nil, fmt.Errorf("invalid base_url: %w", err)
	}

	switch protocol {
	case "openai-compatible", "custom-http":
		return listOpenAICompatibleModels(ctx, baseURL, apiKey)
	case "ollama":
		// Ollama native tags API, fallback to OpenAI-compatible /models
		models, err := listOllamaModels(ctx, baseURL)
		if err == nil && len(models) > 0 {
			return models, nil
		}
		return listOpenAICompatibleModels(ctx, baseURL, apiKey)
	case "gemini", "gemini-response":
		return listGeminiModels(ctx, baseURL, apiKey)
	case "anthropic":
		// Anthropic 没有稳定公开 models 列表接口；保留手工输入
		return []string{}, nil
	default:
		return nil, fmt.Errorf("unsupported protocol %q", protocol)
	}
}

func listOpenAICompatibleModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	endpoint := joinURL(baseURL, "/models")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	req.Header.Set("Accept", "application/json")

	body, status, err := doJSON(req)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("list models failed: HTTP %d: %s", status, truncate(string(body), 240))
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
		Models []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse models response: %w", err)
	}

	set := map[string]struct{}{}
	for _, item := range payload.Data {
		if id := strings.TrimSpace(item.ID); id != "" {
			set[id] = struct{}{}
		}
	}
	for _, item := range payload.Models {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			id = strings.TrimSpace(item.Name)
		}
		if id != "" {
			set[id] = struct{}{}
		}
	}
	return sortedKeys(set), nil
}

func listOllamaModels(ctx context.Context, baseURL string) ([]string, error) {
	// base may be http://host:11434 or http://host:11434/v1
	root := strings.TrimSuffix(baseURL, "/v1")
	endpoint := joinURL(root, "/api/tags")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	body, status, err := doJSON(req)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("list ollama models failed: HTTP %d: %s", status, truncate(string(body), 240))
	}
	var payload struct {
		Models []struct {
			Name string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, item := range payload.Models {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimSpace(item.Model)
		}
		if name != "" {
			set[name] = struct{}{}
		}
	}
	return sortedKeys(set), nil
}

func listGeminiModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	endpoint := joinURL(baseURL, "/models")
	if apiKey != "" {
		u, err := url.Parse(endpoint)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("key", apiKey)
		u.RawQuery = q.Encode()
		endpoint = u.String()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("x-goog-api-key", apiKey)
	}
	req.Header.Set("Accept", "application/json")
	body, status, err := doJSON(req)
	if err != nil {
		return nil, err
	}
	if status >= 400 {
		return nil, fmt.Errorf("list gemini models failed: HTTP %d: %s", status, truncate(string(body), 240))
	}
	var payload struct {
		Models []struct {
			Name                       string   `json:"name"`
			SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	for _, item := range payload.Models {
		name := strings.TrimSpace(item.Name)
		name = strings.TrimPrefix(name, "models/")
		if name == "" {
			continue
		}
		// 优先保留可 generateContent 的模型；无标记时也保留
		if len(item.SupportedGenerationMethods) > 0 {
			ok := false
			for _, m := range item.SupportedGenerationMethods {
				if m == "generateContent" || m == "generateText" {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		set[name] = struct{}{}
	}
	return sortedKeys(set), nil
}

func doJSON(req *http.Request) ([]byte, int, error) {
	client := &http.Client{Timeout: 12 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}

func joinURL(base, path string) string {
	base = strings.TrimRight(base, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return base + path
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
