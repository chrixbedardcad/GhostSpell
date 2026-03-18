package llm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/chrixbedardcad/GhostSpell/config"
)

const defaultAnthropicEndpoint = "https://api.anthropic.com/v1/messages"

// AnthropicClient implements the Client interface for Anthropic Claude.
type AnthropicClient struct {
	apiKey     string
	model      string
	endpoint   string
	maxTokens  int
	timeoutMs  int
	httpClient *http.Client
}

// newAnthropicFromDef creates a new Anthropic client from a provider definition.
func newAnthropicFromDef(def config.LLMProviderDef) *AnthropicClient {
	endpoint := def.APIEndpoint
	if endpoint == "" {
		endpoint = defaultAnthropicEndpoint
	}
	maxTokens := def.MaxTokens
	if maxTokens == 0 {
		maxTokens = 256
	}
	timeoutMs := def.TimeoutMs
	if timeoutMs == 0 {
		timeoutMs = 30000
	}

	return &AnthropicClient{
		apiKey:     def.APIKey,
		model:      def.Model,
		endpoint:   endpoint,
		maxTokens:  maxTokens,
		timeoutMs:  timeoutMs,
		httpClient: newPooledHTTPClient(timeoutMs),
	}
}

func (c *AnthropicClient) Provider() string {
	return "anthropic"
}

func (c *AnthropicClient) Close() {
	c.httpClient.CloseIdleConnections()
}

// anthropicRequest is the request body for the Anthropic Messages API.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"` // string for text-only, []anthropicContentBlock for vision
}

// anthropicContentBlock represents a content block in the Anthropic multimodal format.
type anthropicContentBlock struct {
	Type   string                 `json:"type"`
	Text   string                 `json:"text,omitempty"`
	Source *anthropicImageSource  `json:"source,omitempty"`
}

type anthropicImageSource struct {
	Type      string `json:"type"`       // "base64"
	MediaType string `json:"media_type"` // "image/png"
	Data      string `json:"data"`       // base64-encoded image
}

// anthropicResponse is the response body from the Anthropic Messages API.
type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (c *AnthropicClient) Send(ctx context.Context, req Request) (*Response, error) {
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = c.maxTokens
	}

	// Build user message — multimodal content blocks when images present,
	// plain string otherwise (preserves exact existing behavior for text-only).
	var userContent interface{}
	if len(req.Images) > 0 {
		blocks := []anthropicContentBlock{}
		// Add each image as a base64 content block.
		for _, img := range req.Images {
			blocks = append(blocks, anthropicContentBlock{
				Type: "image",
				Source: &anthropicImageSource{
					Type:      "base64",
					MediaType: "image/png",
					Data:      base64.StdEncoding.EncodeToString(img),
				},
			})
		}
		// Add text part.
		text := req.Text
		if text == "" {
			text = req.Prompt
		}
		blocks = append(blocks, anthropicContentBlock{Type: "text", Text: text})
		userContent = blocks
	} else {
		userContent = req.Text
	}

	body := anthropicRequest{
		Model:     c.model,
		MaxTokens: maxTokens,
		System:    req.Prompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userContent},
		},
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	slog.Debug("anthropic: sending request", "model", c.model, "endpoint", c.endpoint, "body_len", len(jsonBody))

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	slog.Debug("anthropic: response received", "status", resp.StatusCode)

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Debug("anthropic: HTTP error", "status", resp.StatusCode, "body", string(respBody))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return nil, fmt.Errorf("API error (%s): %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	if len(apiResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from API")
	}

	return &Response{
		Text:     apiResp.Content[0].Text,
		Provider: "anthropic",
		Model:    c.model,
	}, nil
}
