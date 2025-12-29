// Package agent provides tests for the browser automation agent.
package agent

import (
	"strings"
	"sync"
	"testing"
)

// TestNewBrowserAgent tests agent creation with various configurations.
func TestNewBrowserAgent(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		wantIter int
		wantTok  int
		wantMod  string
	}{
		{
			name:     "default values",
			cfg:      Config{},
			wantIter: 50,
			wantTok:  1048576,
			wantMod:  "gemini-3-flash-preview",
		},
		{
			name: "custom iterations",
			cfg: Config{
				MaxIterations: 100,
			},
			wantIter: 100,
			wantTok:  1048576,
			wantMod:  "gemini-3-flash-preview",
		},
		{
			name: "custom model",
			cfg: Config{
				Model: "gemini-pro",
			},
			wantIter: 50,
			wantTok:  1048576,
			wantMod:  "gemini-pro",
		},
		{
			name: "all custom values",
			cfg: Config{
				MaxIterations: 25,
				MaxTokens:     500000,
				Model:         "custom-model",
				Debug:         true,
			},
			wantIter: 25,
			wantTok:  500000,
			wantMod:  "custom-model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent := New(tt.cfg, nil)
			if agent == nil {
				t.Fatal("New() returned nil")
			}
			if agent.config.MaxIterations != tt.wantIter {
				t.Errorf("MaxIterations = %d, want %d", agent.config.MaxIterations, tt.wantIter)
			}
			if agent.config.MaxTokens != tt.wantTok {
				t.Errorf("MaxTokens = %d, want %d", agent.config.MaxTokens, tt.wantTok)
			}
			if agent.config.Model != tt.wantMod {
				t.Errorf("Model = %q, want %q", agent.config.Model, tt.wantMod)
			}
			// Verify findings slice is initialized
			if agent.findings == nil {
				t.Error("findings should be initialized, got nil")
			}
			if len(agent.findings) != 0 {
				t.Errorf("findings should be empty initially, got %d", len(agent.findings))
			}
		})
	}
}

// TestFindingsStore tests the in-memory findings storage functionality.
func TestFindingsStore(t *testing.T) {
	agent := New(Config{}, nil)

	t.Run("empty initially", func(t *testing.T) {
		findings := agent.GetFindings()
		if len(findings) != 0 {
			t.Errorf("expected 0 findings initially, got %d", len(findings))
		}
	})

	t.Run("add single finding", func(t *testing.T) {
		// Simulate adding a finding directly (normally done by save_finding tool)
		agent.findingsMu.Lock()
		agent.findings = append(agent.findings, map[string]any{
			"category": "test",
			"title":    "Test Finding",
			"details":  "Test details",
		})
		agent.findingsMu.Unlock()

		findings := agent.GetFindings()
		if len(findings) != 1 {
			t.Errorf("expected 1 finding, got %d", len(findings))
		}
		if findings[0]["title"] != "Test Finding" {
			t.Errorf("title = %v, want 'Test Finding'", findings[0]["title"])
		}
	})

	t.Run("add multiple findings", func(t *testing.T) {
		agent.findingsMu.Lock()
		agent.findings = append(agent.findings,
			map[string]any{"category": "lead", "title": "Lead 1"},
			map[string]any{"category": "contact", "title": "Contact 1"},
		)
		agent.findingsMu.Unlock()

		findings := agent.GetFindings()
		if len(findings) != 3 {
			t.Errorf("expected 3 findings, got %d", len(findings))
		}
	})

	t.Run("GetFindings returns copy", func(t *testing.T) {
		findings := agent.GetFindings()
		// Modify the returned copy
		findings[0]["title"] = "Modified"

		// Original should be unchanged
		original := agent.GetFindings()
		if original[0]["title"] == "Modified" {
			t.Error("GetFindings should return a copy, but original was modified")
		}
	})
}

// TestFindingsStoreConcurrency tests thread-safety of the findings store.
func TestFindingsStoreConcurrency(t *testing.T) {
	agent := New(Config{}, nil)

	const numGoroutines = 100
	const findingsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < findingsPerGoroutine; j++ {
				agent.findingsMu.Lock()
				agent.findings = append(agent.findings, map[string]any{
					"category": "concurrent",
					"title":    "Finding from goroutine",
					"id":       id*findingsPerGoroutine + j,
				})
				agent.findingsMu.Unlock()
			}
		}(i)
	}

	// Concurrent reads while writes are happening
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < findingsPerGoroutine; j++ {
				_ = agent.GetFindings()
			}
		}()
	}

	wg.Wait()

	// Verify all findings were added
	expected := numGoroutines * findingsPerGoroutine
	actual := len(agent.GetFindings())
	if actual != expected {
		t.Errorf("expected %d findings after concurrent writes, got %d", expected, actual)
	}
}

// TestFindingsSearchByCategory tests category-based filtering of findings.
func TestFindingsSearchByCategory(t *testing.T) {
	agent := New(Config{}, nil)

	// Add findings with different categories
	agent.findingsMu.Lock()
	agent.findings = []map[string]any{
		{"category": "lead", "title": "Lead 1", "details": "First lead"},
		{"category": "lead", "title": "Lead 2", "details": "Second lead"},
		{"category": "contact", "title": "Contact 1", "details": "First contact"},
		{"category": "product", "title": "Product A", "details": "Product details"},
	}
	agent.findingsMu.Unlock()

	tests := []struct {
		category string
		expected int
	}{
		{"lead", 2},
		{"contact", 1},
		{"product", 1},
		{"nonexistent", 0},
		{"", 4}, // Empty category returns all
	}

	for _, tt := range tests {
		t.Run("category_"+tt.category, func(t *testing.T) {
			agent.findingsMu.RLock()
			allFindings := make([]map[string]any, len(agent.findings))
			copy(allFindings, agent.findings)
			agent.findingsMu.RUnlock()

			var results []map[string]any
			if tt.category != "" {
				for _, finding := range allFindings {
					cat, _ := finding["category"].(string)
					if cat == tt.category {
						results = append(results, finding)
					}
				}
			} else {
				results = allFindings
			}

			if len(results) != tt.expected {
				t.Errorf("category %q: expected %d results, got %d", tt.category, tt.expected, len(results))
			}
		})
	}
}

// TestFindingsSearchByQuery tests query-based filtering of findings.
func TestFindingsSearchByQuery(t *testing.T) {
	agent := New(Config{}, nil)

	// Add findings with searchable content
	agent.findingsMu.Lock()
	agent.findings = []map[string]any{
		{"category": "lead", "title": "John Doe", "details": "CEO at TechCorp"},
		{"category": "lead", "title": "Jane Smith", "details": "CTO at DataInc"},
		{"category": "contact", "title": "Support Team", "details": "support@example.com"},
		{"category": "product", "title": "Widget Pro", "details": "Enterprise solution"},
	}
	agent.findingsMu.Unlock()

	tests := []struct {
		query    string
		expected int
	}{
		{"John", 1},
		{"CEO", 1},
		{"support", 1},
		{"Pro", 1},         // Matches "Widget Pro"
		{"TechCorp", 1},    // Matches in details
		{"enterprise", 1},  // Case-insensitive
		{"JOHN", 1},        // Case-insensitive
		{"nonexistent", 0}, // No matches
		{"", 4},            // Empty query returns all
		{"at", 2},          // Matches "at TechCorp" and "at DataInc"
	}

	for _, tt := range tests {
		t.Run("query_"+tt.query, func(t *testing.T) {
			agent.findingsMu.RLock()
			allFindings := make([]map[string]any, len(agent.findings))
			copy(allFindings, agent.findings)
			agent.findingsMu.RUnlock()

			var results []map[string]any

			if tt.query != "" {
				queryLower := strings.ToLower(tt.query)
				for _, finding := range allFindings {
					title, _ := finding["title"].(string)
					details, _ := finding["details"].(string)
					if strings.Contains(strings.ToLower(title), queryLower) ||
						strings.Contains(strings.ToLower(details), queryLower) {
						results = append(results, finding)
					}
				}
			} else {
				results = allFindings
			}

			if len(results) != tt.expected {
				t.Errorf("query %q: expected %d results, got %d", tt.query, tt.expected, len(results))
			}
		})
	}
}

// TestSanitizeFilename tests the filename sanitization function.
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with spaces", "with_spaces"},
		{"with/slashes", "with_slashes"},
		{"with\\backslashes", "with_backslashes"},
		{"with:colons", "with_colons"},
		{"with*stars", "with_stars"},
		{"with?questions", "with_questions"},
		{"with\"quotes", "with_quotes"},
		{"with<brackets>", "with_brackets_"},
		{"with|pipes", "with_pipes"},
		{"mixed spaces and/slashes", "mixed_spaces_and_slashes"},
		{"", ""},
		{"   ", ""},          // Leading non-alphanumeric are skipped
		{"abc123", "abc123"}, // Pure alphanumeric unchanged
		{"a__b", "a_b"},      // No consecutive underscores (implementation dedupes)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestConfigDefaults verifies default configuration values.
func TestConfigDefaults(t *testing.T) {
	t.Run("zero values get defaults", func(t *testing.T) {
		cfg := Config{}
		agent := New(cfg, nil)

		if agent.config.MaxIterations != 50 {
			t.Errorf("default MaxIterations = %d, want 50", agent.config.MaxIterations)
		}
		if agent.config.MaxTokens != 1048576 {
			t.Errorf("default MaxTokens = %d, want 1048576", agent.config.MaxTokens)
		}
		if agent.config.Model != "gemini-3-flash-preview" {
			t.Errorf("default Model = %q, want 'gemini-3-flash-preview'", agent.config.Model)
		}
	})

	t.Run("explicit values preserved", func(t *testing.T) {
		cfg := Config{
			MaxIterations: 1,
			MaxTokens:     1,
			Model:         "custom",
		}
		agent := New(cfg, nil)

		if agent.config.MaxIterations != 1 {
			t.Errorf("MaxIterations = %d, want 1", agent.config.MaxIterations)
		}
		if agent.config.MaxTokens != 1 {
			t.Errorf("MaxTokens = %d, want 1", agent.config.MaxTokens)
		}
		if agent.config.Model != "custom" {
			t.Errorf("Model = %q, want 'custom'", agent.config.Model)
		}
	})
}

// TestLoggerCreation verifies logger is properly initialized.
func TestLoggerCreation(t *testing.T) {
	t.Run("debug mode logger", func(t *testing.T) {
		agent := New(Config{Debug: true}, nil)
		if agent.logger == nil {
			t.Error("logger should not be nil")
		}
	})

	t.Run("non-debug mode logger", func(t *testing.T) {
		agent := New(Config{Debug: false}, nil)
		if agent.logger == nil {
			t.Error("logger should not be nil even in non-debug mode")
		}
	})
}

// TestGetBrowser verifies browser accessor.
func TestGetBrowser(t *testing.T) {
	t.Run("nil browser", func(t *testing.T) {
		agent := New(Config{}, nil)
		if agent.GetBrowser() != nil {
			t.Error("GetBrowser should return nil when no browser set")
		}
	})
}

// TestGetLogger verifies logger accessor.
func TestGetLogger(t *testing.T) {
	agent := New(Config{Debug: true}, nil)
	logger := agent.GetLogger()
	if logger == nil {
		t.Error("GetLogger should not return nil")
	}
}

// Benchmark tests
func BenchmarkFindingsAdd(b *testing.B) {
	agent := New(Config{}, nil)
	finding := map[string]any{
		"category": "benchmark",
		"title":    "Benchmark Finding",
		"details":  "Details for benchmark",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		agent.findingsMu.Lock()
		agent.findings = append(agent.findings, finding)
		agent.findingsMu.Unlock()
	}
}

func BenchmarkFindingsGet(b *testing.B) {
	agent := New(Config{}, nil)
	// Pre-populate with some findings
	for i := 0; i < 100; i++ {
		agent.findings = append(agent.findings, map[string]any{
			"category": "benchmark",
			"title":    "Benchmark Finding",
			"details":  "Details for benchmark",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = agent.GetFindings()
	}
}

func BenchmarkFindingsGetConcurrent(b *testing.B) {
	agent := New(Config{}, nil)
	// Pre-populate
	for i := 0; i < 100; i++ {
		agent.findings = append(agent.findings, map[string]any{
			"category": "benchmark",
			"title":    "Benchmark Finding",
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = agent.GetFindings()
		}
	})
}
