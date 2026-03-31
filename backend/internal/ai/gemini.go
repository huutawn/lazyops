package ai

type GeminiClient struct {
	APIKey string
}

func NewGeminiClient(apiKey string) *GeminiClient {
	return &GeminiClient{APIKey: apiKey}
}
