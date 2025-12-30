// Package agent provides the ADK-based browser automation agent.
package agent

import (
	"context"
	"sync"

	"google.golang.org/genai"
)

// Tokenizer provides accurate token counting using the Gemini API.
// It caches token counts for identical content to reduce API calls.
type Tokenizer struct {
	client    *genai.Client
	model     string
	cache     map[string]int
	cacheMu   sync.RWMutex
	estimator *TokenCounter // Fallback for when API is unavailable
}

// TokenizerConfig holds configuration for creating a Tokenizer.
type TokenizerConfig struct {
	// APIKey is the Gemini API key.
	APIKey string

	// Model is the model ID for token counting. Default: "gemini-2.5-flash"
	Model string

	// MaxTokens for the fallback estimator. Default: 1048576
	MaxTokens int
}

// NewTokenizer creates a new Tokenizer with the given configuration.
// It initializes a genai client for accurate token counting.
func NewTokenizer(ctx context.Context, cfg TokenizerConfig) (*Tokenizer, error) {
	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 1048576
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, err
	}

	return &Tokenizer{
		client:    client,
		model:     cfg.Model,
		cache:     make(map[string]int),
		estimator: NewTokenCounter(cfg.MaxTokens),
	}, nil
}

// CountTextTokens returns the exact token count for the given text.
// Uses the Gemini API for accurate counting, falls back to estimation on error.
func (t *Tokenizer) CountTextTokens(ctx context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}

	// Check cache first
	t.cacheMu.RLock()
	if count, ok := t.cache[text]; ok {
		t.cacheMu.RUnlock()
		return count, nil
	}
	t.cacheMu.RUnlock()

	// Call API for accurate count
	result, err := t.client.Models.CountTokens(ctx, t.model, genai.Text(text), nil)
	if err != nil {
		// Fall back to estimation
		return t.estimator.EstimateTextTokens(text), nil
	}

	count := int(result.TotalTokens)

	// Cache result (only cache reasonably sized texts to prevent memory bloat)
	if len(text) < 10000 {
		t.cacheMu.Lock()
		t.cache[text] = count
		t.cacheMu.Unlock()
	}

	return count, nil
}

// CountTokens returns the exact token count for mixed content (text and images).
// Uses the Gemini API for accurate counting.
func (t *Tokenizer) CountTokens(ctx context.Context, parts ...*genai.Part) (int, error) {
	if len(parts) == 0 {
		return 0, nil
	}

	// Wrap parts in Content for the API
	contents := []*genai.Content{{Parts: parts}}
	result, err := t.client.Models.CountTokens(ctx, t.model, contents, nil)
	if err != nil {
		// Fall back to estimation for each part
		total := 0
		for _, part := range parts {
			if part.Text != "" {
				total += t.estimator.EstimateTextTokens(part.Text)
			} else if part.InlineData != nil {
				// Rough estimate for images based on data size
				total += t.estimator.EstimateImageTokens(800, 600) // Assume typical size
			}
		}
		return total, nil
	}

	return int(result.TotalTokens), nil
}

// CountImageTokens returns the token count for an image.
// Uses the Gemini API for accurate counting.
func (t *Tokenizer) CountImageTokens(ctx context.Context, imageData []byte, mimeType string) (int, error) {
	if len(imageData) == 0 {
		return 0, nil
	}

	part := &genai.Part{
		InlineData: &genai.Blob{
			Data:     imageData,
			MIMEType: mimeType,
		},
	}

	// Wrap parts in Content for the API
	contents := []*genai.Content{{Parts: []*genai.Part{part}}}
	result, err := t.client.Models.CountTokens(ctx, t.model, contents, nil)
	if err != nil {
		// Fall back to estimation
		return t.estimator.EstimateImageTokens(800, 600), nil
	}

	return int(result.TotalTokens), nil
}

// EstimateTextTokens provides a quick estimate without API call.
// Use this for non-critical counting or when API quota is a concern.
func (t *Tokenizer) EstimateTextTokens(text string) int {
	return t.estimator.EstimateTextTokens(text)
}

// EstimateImageTokens provides a quick estimate for image tokens.
func (t *Tokenizer) EstimateImageTokens(width, height int) int {
	return t.estimator.EstimateImageTokens(width, height)
}

// ClearCache clears the token count cache.
func (t *Tokenizer) ClearCache() {
	t.cacheMu.Lock()
	t.cache = make(map[string]int)
	t.cacheMu.Unlock()
}

// Close releases resources associated with the tokenizer.
func (t *Tokenizer) Close() {
	// The genai client doesn't have a Close method, but we clear cache
	t.ClearCache()
}
