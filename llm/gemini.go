package llm

import (
	"net/http"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
)

const defaultGeminiEndpoint = "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"

// newGeminiFromDef creates a Google Gemini client from a provider definition.
// Gemini supports an OpenAI-compatible API, so we reuse OpenAIClient with a
// different default endpoint.
func newGeminiFromDef(def config.LLMProviderDef) *OpenAIClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultGeminiEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}
	timeoutMs := def.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 30000
	}

	return &OpenAIClient{
		apiKey:       def.APIKey,
		model:        def.Model,
		endpoint:     endpoint,
		maxTokens:    maxTokens,
		timeoutMs:    timeoutMs,
		providerName: "gemini",
		httpClient:   &http.Client{Timeout: 120 * time.Second},
	}
}
