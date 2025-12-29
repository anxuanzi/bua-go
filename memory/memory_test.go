package memory

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	cfg := &Config{
		ShortTermLimit: 5,
	}

	m := NewManager(cfg)
	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.config.ShortTermLimit != 5 {
		t.Errorf("ShortTermLimit = %d, want 5", m.config.ShortTermLimit)
	}
}

func TestNewManager_DefaultLimit(t *testing.T) {
	m := NewManager(&Config{})
	if m.config.ShortTermLimit != 10 {
		t.Errorf("Default ShortTermLimit = %d, want 10", m.config.ShortTermLimit)
	}
}

func TestManager_StartTask(t *testing.T) {
	m := NewManager(&Config{})
	m.StartTask("Test task")

	ctx := m.GetTaskContext()
	if ctx == "" {
		t.Error("GetTaskContext() returned empty string after StartTask")
	}
}

func TestManager_AddObservation(t *testing.T) {
	m := NewManager(&Config{ShortTermLimit: 3})
	m.StartTask("Test task")

	// Add observations
	for i := 0; i < 5; i++ {
		m.AddObservation(&Observation{
			URL:   "https://example.com",
			Title: "Test",
		})
	}

	// Should have compacted
	obs := m.GetRecentObservations(0)
	if len(obs) > 3 {
		t.Errorf("Observations not compacted: got %d, want <= 3", len(obs))
	}
}

func TestManager_GetRecentObservations(t *testing.T) {
	m := NewManager(&Config{ShortTermLimit: 10})
	m.StartTask("Test task")

	// Add 5 observations
	for i := 0; i < 5; i++ {
		m.AddObservation(&Observation{
			URL:   "https://example.com",
			Title: "Test",
		})
	}

	// Get 3 most recent
	obs := m.GetRecentObservations(3)
	if len(obs) != 3 {
		t.Errorf("GetRecentObservations(3) returned %d, want 3", len(obs))
	}

	// Get all
	obs = m.GetRecentObservations(0)
	if len(obs) != 5 {
		t.Errorf("GetRecentObservations(0) returned %d, want 5", len(obs))
	}

	// Request more than available
	obs = m.GetRecentObservations(10)
	if len(obs) != 5 {
		t.Errorf("GetRecentObservations(10) returned %d, want 5", len(obs))
	}
}

func TestManager_LongTermMemory(t *testing.T) {
	m := NewManager(&Config{})

	entry := &LongTermEntry{
		Key:     "test-key",
		Type:    "pattern",
		Content: "Test content",
	}

	m.AddLongTermMemory(entry)

	// Retrieve it
	got, ok := m.GetLongTermMemory("test-key")
	if !ok {
		t.Fatal("GetLongTermMemory() returned false")
	}
	if got.Content != "Test content" {
		t.Errorf("Content = %s, want 'Test content'", got.Content)
	}
	if got.AccessCount != 1 {
		t.Errorf("AccessCount = %d, want 1", got.AccessCount)
	}

	// Not found case
	_, ok = m.GetLongTermMemory("nonexistent")
	if ok {
		t.Error("GetLongTermMemory('nonexistent') should return false")
	}
}

func TestManager_RecordSuccess(t *testing.T) {
	m := NewManager(&Config{})
	m.RecordSuccess("example.com", "click_login", "Successfully logged in")

	results := m.SearchLongTermMemory("login", "example.com")
	if len(results) == 0 {
		t.Error("RecordSuccess() entry not found in search")
	}
}

func TestManager_RecordFailure(t *testing.T) {
	m := NewManager(&Config{})
	m.RecordFailure("example.com", "click_submit", "Button not found")

	results := m.SearchLongTermMemory("submit", "example.com")
	if len(results) == 0 {
		t.Error("RecordFailure() entry not found in search")
	}
}

func TestManager_SaveLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "bua-memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create manager and add data
	m1 := NewManager(&Config{StorageDir: tmpDir})
	m1.AddLongTermMemory(&LongTermEntry{
		Key:     "test-key",
		Type:    "pattern",
		Content: "Test content",
	})

	// Save
	ctx := context.Background()
	if err := m1.Save(ctx); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	// Verify file exists
	memFile := filepath.Join(tmpDir, "memory.json")
	if _, err := os.Stat(memFile); os.IsNotExist(err) {
		t.Fatal("Memory file was not created")
	}

	// Create new manager and load
	m2 := NewManager(&Config{StorageDir: tmpDir})
	if err := m2.Load(ctx); err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// Verify data was loaded
	got, ok := m2.GetLongTermMemory("test-key")
	if !ok {
		t.Fatal("Loaded manager missing test-key")
	}
	if got.Content != "Test content" {
		t.Errorf("Loaded Content = %s, want 'Test content'", got.Content)
	}
}

func TestManager_Clear(t *testing.T) {
	m := NewManager(&Config{ShortTermLimit: 10})
	m.StartTask("Test task")
	m.AddObservation(&Observation{URL: "test"})
	m.AddLongTermMemory(&LongTermEntry{Key: "test"})

	m.Clear()

	obs := m.GetRecentObservations(0)
	if len(obs) != 0 {
		t.Errorf("Clear() didn't clear short-term: got %d observations", len(obs))
	}

	stats := m.Stats()
	if stats.LongTermCount != 0 {
		t.Errorf("Clear() didn't clear long-term: got %d entries", stats.LongTermCount)
	}
}

func TestManager_ClearShortTerm(t *testing.T) {
	m := NewManager(&Config{ShortTermLimit: 10})
	m.StartTask("Test task")
	m.AddObservation(&Observation{URL: "test"})
	m.AddLongTermMemory(&LongTermEntry{Key: "test"})

	m.ClearShortTerm()

	obs := m.GetRecentObservations(0)
	if len(obs) != 0 {
		t.Errorf("ClearShortTerm() didn't clear: got %d observations", len(obs))
	}

	// Long-term should still exist
	_, ok := m.GetLongTermMemory("test")
	if !ok {
		t.Error("ClearShortTerm() also cleared long-term")
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager(&Config{ShortTermLimit: 10})
	m.StartTask("Test task")
	m.AddObservation(&Observation{URL: "test"})
	m.AddLongTermMemory(&LongTermEntry{Key: "test1"})
	m.AddLongTermMemory(&LongTermEntry{Key: "test2"})

	stats := m.Stats()

	if stats.ShortTermCount != 1 {
		t.Errorf("ShortTermCount = %d, want 1", stats.ShortTermCount)
	}
	if stats.ShortTermLimit != 10 {
		t.Errorf("ShortTermLimit = %d, want 10", stats.ShortTermLimit)
	}
	if stats.LongTermCount != 2 {
		t.Errorf("LongTermCount = %d, want 2", stats.LongTermCount)
	}
	if stats.TaskPrompt != "Test task" {
		t.Errorf("TaskPrompt = %s, want 'Test task'", stats.TaskPrompt)
	}
}

func TestObservation_Timestamp(t *testing.T) {
	m := NewManager(&Config{})
	m.StartTask("Test")

	// Add observation without timestamp
	m.AddObservation(&Observation{URL: "test"})

	obs := m.GetRecentObservations(1)
	if len(obs) != 1 {
		t.Fatal("Expected 1 observation")
	}

	// Timestamp should be set automatically
	if obs[0].Timestamp.IsZero() {
		t.Error("Timestamp was not automatically set")
	}
	if time.Since(obs[0].Timestamp) > time.Second {
		t.Error("Timestamp is too old")
	}
}

// Additional edge case tests

func TestManager_EmptyConfig(t *testing.T) {
	// Empty config should use defaults
	m := NewManager(&Config{})
	if m == nil {
		t.Fatal("NewManager(&Config{}) returned nil")
	}
	// Should have default limit
	if m.config.ShortTermLimit != 10 {
		t.Errorf("Default ShortTermLimit = %d, want 10", m.config.ShortTermLimit)
	}
}

func TestManager_SearchLongTermMemory(t *testing.T) {
	m := NewManager(&Config{})

	// Add various entries
	m.AddLongTermMemory(&LongTermEntry{
		Key:     "login-pattern",
		Type:    "success",
		Content: "Click login button works on example.com",
		Site:    "example.com",
	})
	m.AddLongTermMemory(&LongTermEntry{
		Key:     "submit-pattern",
		Type:    "failure",
		Content: "Submit button not found",
		Site:    "other.com",
	})
	m.AddLongTermMemory(&LongTermEntry{
		Key:     "login-other",
		Type:    "success",
		Content: "Login successful on another site",
		Site:    "another.com",
	})

	// Note: containsKeywords is a placeholder that returns true if both text and query are non-empty
	// So any non-empty query matches all entries with non-empty content/key
	tests := []struct {
		name     string
		query    string
		site     string
		expected int
	}{
		// Any non-empty query matches all 3 entries (current placeholder behavior)
		{"any query matches all with content", "anything", "", 3},
		// Site filter works: only entries matching site are returned
		{"site filter with query", "anything", "example.com", 1},
		{"site filter different site", "anything", "other.com", 1},
		// Empty query returns nothing (containsKeywords returns false for empty query)
		{"empty query returns nothing", "", "", 0},
		{"empty query with site returns nothing", "", "example.com", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := m.SearchLongTermMemory(tt.query, tt.site)
			if len(results) != tt.expected {
				t.Errorf("SearchLongTermMemory(%q, %q) = %d results, want %d",
					tt.query, tt.site, len(results), tt.expected)
			}
		})
	}
}

func TestManager_LoadNonExistent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bua-memory-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(&Config{StorageDir: tmpDir})
	ctx := context.Background()

	// Load from non-existent file should not error
	err = m.Load(ctx)
	if err != nil {
		t.Errorf("Load() from non-existent file should not error, got: %v", err)
	}
}

func TestManager_SaveInvalidDir(t *testing.T) {
	m := NewManager(&Config{StorageDir: "/nonexistent/path/that/should/not/exist"})
	ctx := context.Background()

	// Save to invalid directory should error
	err := m.Save(ctx)
	if err == nil {
		t.Error("Save() to invalid directory should error")
	}
}

func TestManager_ObservationFields(t *testing.T) {
	m := NewManager(&Config{})
	m.StartTask("Test task")

	obs := &Observation{
		URL:   "https://example.com/page",
		Title: "Example Page",
		Action: &Action{
			Type:      "click",
			Target:    "42",
			Value:     "button#submit",
			Reasoning: "Need to submit the form",
		},
		Result:         "success",
		ScreenshotPath: "/tmp/screenshot.png",
		ElementCount:   100,
	}
	m.AddObservation(obs)

	retrieved := m.GetRecentObservations(1)
	if len(retrieved) != 1 {
		t.Fatal("Expected 1 observation")
	}

	r := retrieved[0]
	if r.URL != "https://example.com/page" {
		t.Errorf("URL = %q", r.URL)
	}
	if r.Title != "Example Page" {
		t.Errorf("Title = %q", r.Title)
	}
	if r.Action == nil {
		t.Error("Action should not be nil")
	} else {
		if r.Action.Type != "click" {
			t.Errorf("Action.Type = %q", r.Action.Type)
		}
		if r.Action.Target != "42" {
			t.Errorf("Action.Target = %q", r.Action.Target)
		}
		if r.Action.Value != "button#submit" {
			t.Errorf("Action.Value = %q", r.Action.Value)
		}
		if r.Action.Reasoning != "Need to submit the form" {
			t.Errorf("Action.Reasoning = %q", r.Action.Reasoning)
		}
	}
	if r.Result != "success" {
		t.Errorf("Result = %q", r.Result)
	}
	if r.ScreenshotPath != "/tmp/screenshot.png" {
		t.Errorf("ScreenshotPath = %q", r.ScreenshotPath)
	}
	if r.ElementCount != 100 {
		t.Errorf("ElementCount = %d", r.ElementCount)
	}
}

func TestManager_LongTermEntry_AccessCount(t *testing.T) {
	m := NewManager(&Config{})

	m.AddLongTermMemory(&LongTermEntry{
		Key:     "test-key",
		Content: "Test content",
	})

	// Access multiple times
	for i := 0; i < 5; i++ {
		_, _ = m.GetLongTermMemory("test-key")
	}

	got, _ := m.GetLongTermMemory("test-key")
	if got.AccessCount != 6 { // 5 + 1 from this call
		t.Errorf("AccessCount = %d, want 6", got.AccessCount)
	}
}

func TestManager_LongTermEntry_AccessedAt(t *testing.T) {
	m := NewManager(&Config{})

	m.AddLongTermMemory(&LongTermEntry{
		Key:     "test-key",
		Content: "Test content",
	})

	time.Sleep(10 * time.Millisecond)
	before := time.Now()

	m.GetLongTermMemory("test-key")

	got, _ := m.GetLongTermMemory("test-key")
	if got.AccessedAt.Before(before) {
		t.Error("AccessedAt was not updated")
	}
}

func TestManager_OverwriteLongTermEntry(t *testing.T) {
	m := NewManager(&Config{})

	// Add initial entry
	m.AddLongTermMemory(&LongTermEntry{
		Key:     "test-key",
		Content: "Original content",
	})

	// Overwrite with same key
	m.AddLongTermMemory(&LongTermEntry{
		Key:     "test-key",
		Content: "Updated content",
	})

	got, ok := m.GetLongTermMemory("test-key")
	if !ok {
		t.Fatal("Entry not found")
	}
	if got.Content != "Updated content" {
		t.Errorf("Content = %q, want 'Updated content'", got.Content)
	}
}

func TestManager_ObservationCompaction(t *testing.T) {
	m := NewManager(&Config{ShortTermLimit: 3})
	m.StartTask("Test task")

	// Add more than the limit
	for i := 0; i < 10; i++ {
		m.AddObservation(&Observation{
			URL:   "https://example.com",
			Title: "Test",
		})
	}

	obs := m.GetRecentObservations(0)
	if len(obs) > 3 {
		t.Errorf("Observations not compacted: got %d, want <= 3", len(obs))
	}
}

func TestManager_GetTaskContext_Empty(t *testing.T) {
	m := NewManager(&Config{})

	// Without starting a task
	ctx := m.GetTaskContext()
	if ctx != "" {
		t.Errorf("GetTaskContext() without StartTask should return empty, got %q", ctx)
	}
}

func TestManager_ConcurrentOperations(t *testing.T) {
	m := NewManager(&Config{ShortTermLimit: 100})
	m.StartTask("Concurrent test")

	const numGoroutines = 50
	const opsPerGoroutine = 20

	var wg sync.WaitGroup

	// Concurrent observation writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				m.AddObservation(&Observation{
					URL:   "https://example.com",
					Title: "Concurrent test",
				})
			}
		}(i)
	}

	// Concurrent long-term writes
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				key := "key-" + time.Now().String() + "-" + string(rune(id)) + "-" + string(rune(j))
				m.AddLongTermMemory(&LongTermEntry{
					Key:     key,
					Content: "Test",
				})
			}
		}(i)
	}

	// Concurrent reads
	wg.Add(numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = m.GetRecentObservations(10)
				_ = m.SearchLongTermMemory("test", "")
				_ = m.Stats()
			}
		}()
	}

	wg.Wait()

	// Should not panic and should have data
	stats := m.Stats()
	if stats.LongTermCount == 0 {
		t.Error("Expected some long-term entries after concurrent writes")
	}
}

// Benchmarks

func BenchmarkAddObservation(b *testing.B) {
	m := NewManager(&Config{ShortTermLimit: 100})
	m.StartTask("Benchmark")
	obs := &Observation{URL: "test", Title: "Test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.AddObservation(obs)
	}
}

func BenchmarkGetRecentObservations(b *testing.B) {
	m := NewManager(&Config{ShortTermLimit: 100})
	m.StartTask("Benchmark")
	for i := 0; i < 100; i++ {
		m.AddObservation(&Observation{URL: "test", Title: "Test"})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.GetRecentObservations(10)
	}
}

func BenchmarkSearchLongTermMemory(b *testing.B) {
	m := NewManager(&Config{})
	for i := 0; i < 100; i++ {
		m.AddLongTermMemory(&LongTermEntry{
			Key:     "key-" + string(rune(i)),
			Content: "Test content with various words for searching",
			Site:    "example.com",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.SearchLongTermMemory("content", "example.com")
	}
}

func BenchmarkSaveLoad(b *testing.B) {
	tmpDir, _ := os.MkdirTemp("", "bua-memory-bench")
	defer os.RemoveAll(tmpDir)

	m := NewManager(&Config{StorageDir: tmpDir})
	for i := 0; i < 50; i++ {
		m.AddLongTermMemory(&LongTermEntry{
			Key:     "key-" + string(rune(i)),
			Content: "Test content",
		})
	}

	ctx := context.Background()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_ = m.Save(ctx)
		_ = m.Load(ctx)
	}
}
