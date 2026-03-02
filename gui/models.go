package gui

// KnownModels returns a curated list of models for the given provider.
func KnownModels(provider string) []string {
	switch provider {
	case "anthropic":
		return []string{
			"claude-sonnet-4-5-20250929",
			"claude-haiku-4-5-20251001",
			"claude-3-5-sonnet-20241022",
			"claude-3-5-haiku-20241022",
		}
	case "openai":
		return []string{
			"gpt-4o",
			"gpt-4o-mini",
			"gpt-4-turbo",
			"gpt-3.5-turbo",
		}
	case "gemini":
		return []string{
			"gemini-2.0-flash",
			"gemini-1.5-pro",
			"gemini-1.5-flash",
		}
	case "xai":
		return []string{
			"grok-2",
			"grok-2-mini",
		}
	case "ollama":
		return []string{
			"llama3",
			"mistral",
			"codellama",
			"gemma",
			"phi3",
		}
	default:
		return nil
	}
}
