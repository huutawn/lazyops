package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type EmbeddingClient interface {
	EmbedText(text string) (string, error)
}

type DeterministicEmbeddingClient struct{}

type HTTPEmbeddingClient struct {
	BaseURL    string
	APIKey     string
	Model      string
	HTTPClient *http.Client
	MaxRetries int
}

func NewDeterministicEmbeddingClient() *DeterministicEmbeddingClient {
	return &DeterministicEmbeddingClient{}
}

func NewHTTPEmbeddingClient(baseURL, apiKey, model string) *HTTPEmbeddingClient {
	return &HTTPEmbeddingClient{
		BaseURL: strings.TrimSpace(baseURL),
		APIKey:  strings.TrimSpace(apiKey),
		Model:   strings.TrimSpace(model),
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		MaxRetries: 2,
	}
}

func (c *DeterministicEmbeddingClient) EmbedText(text string) (string, error) {
	return deterministicEmbedding(text), nil
}

func (c *HTTPEmbeddingClient) EmbedText(text string) (string, error) {
	if c == nil || c.BaseURL == "" {
		return "", fmt.Errorf("http embedding client is not configured")
	}
	var lastErr error
	attempts := c.MaxRetries + 1
	if attempts <= 0 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		embedding, err := c.embedOnce(text)
		if err == nil {
			return embedding, nil
		}
		lastErr = err
		if attempt < attempts-1 {
			time.Sleep(time.Duration(attempt+1) * 200 * time.Millisecond)
		}
	}
	return "", lastErr
}

func (c *HTTPEmbeddingClient) embedOnce(text string) (string, error) {
	payload := map[string]any{
		"text":  text,
		"model": c.Model,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.BaseURL, bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("embedding provider returned status %d", resp.StatusCode)
	}
	var body struct {
		Embedding any `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	switch value := body.Embedding.(type) {
	case string:
		return strings.TrimSpace(value), nil
	case []any:
		parts := make([]string, 0, len(value))
		for _, item := range value {
			switch number := item.(type) {
			case float64:
				parts = append(parts, formatFloat(number))
			case int:
				parts = append(parts, formatFloat(float64(number)))
			}
		}
		if len(parts) == 0 {
			return "", fmt.Errorf("embedding provider returned empty vector")
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	default:
		return "", fmt.Errorf("embedding provider returned unsupported embedding payload")
	}
}

func deterministicEmbedding(text string) string {
	const dims = 16
	v := make([]float64, dims)
	normalized := strings.ToLower(strings.TrimSpace(text))
	for idx, b := range []byte(normalized) {
		bucket := idx % dims
		v[bucket] += float64(int(b)%31 + 1)
	}
	var magnitude float64
	for _, item := range v {
		magnitude += item * item
	}
	if magnitude > 0 {
		magnitude = math.Sqrt(magnitude)
		for i := range v {
			v[i] = v[i] / magnitude
		}
	}
	parts := make([]string, 0, dims)
	for _, item := range v {
		parts = append(parts, formatFloat(item))
	}
	return "[" + strings.Join(parts, ",") + "]"
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 8, 64)
}
