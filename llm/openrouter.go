package llm

import (
	"net/http"
	"time"

	"github.com/chrixbedardcad/GhostType/config"
)

const defaultOpenRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"

// newOpenRouterFromDef creates an OpenRouter client from a provider definition.
// OpenRouter uses an OpenAI-compatible API with a different base URL and
// optional attribution headers.
func newOpenRouterFromDef(def config.LLMProviderDef) *OpenAIClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultOpenRouterEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 2048
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
		providerName: "openrouter",
		httpClient:   &http.Client{Timeout: 120 * time.Second},
	}
}
