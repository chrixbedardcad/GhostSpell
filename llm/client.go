package llm

import (
	"context"
	"fmt"

	"github.com/chrixbedardcad/GhostSpell/config"
)

// Request represents a request to an LLM provider.
type Request struct {
	Prompt    string
	Text      string
	MaxTokens int
}

// Response represents a response from an LLM provider.
type Response struct {
	Text     string
	Provider string
	Model    string
}

// Client is the interface all LLM providers must implement.
type Client interface {
	// Send sends a prompt with user text to the LLM and returns the response.
	Send(ctx context.Context, req Request) (*Response, error)

	// Provider returns the name of the provider.
	Provider() string

	// Close releases resources (HTTP connections) held by the client.
	Close()
}

// NewClientFromDef creates an LLM client from a provider definition.
// Model tags like "cheap" are resolved to actual model names before
// creating the client.
func NewClientFromDef(def config.LLMProviderDef) (Client, error) {
	def.Model = ResolveModelTag(def.Provider, def.Model)

	switch def.Provider {
	case "anthropic":
		return newAnthropicFromDef(def), nil
	case "openai":
		return newOpenAIFromDef(def), nil
	case "gemini":
		return newGeminiFromDef(def), nil
	case "xai":
		return newXAIFromDef(def), nil
	case "deepseek":
		return newDeepSeekFromDef(def), nil
	case "ollama":
		return newOllamaFromDef(def), nil
	case "lmstudio":
		return newLMStudioFromDef(def), nil
	case "local":
		return newGhostAIFromDef(LLMProviderDefCompat{
			Model:     def.Model,
			MaxTokens: def.MaxTokens,
			TimeoutMs: def.TimeoutMs,
			KeepAlive: def.KeepAlive,
		})
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", def.Provider)
	}
}
