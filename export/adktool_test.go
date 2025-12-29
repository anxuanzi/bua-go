// Package export provides tests for ADK tool wrappers.
package export

import (
	"testing"

	"github.com/anxuanzi/bua-go"
)

// TestDefaultBrowserToolConfig tests default config values.
func TestDefaultBrowserToolConfig(t *testing.T) {
	cfg := DefaultBrowserToolConfig()

	if cfg == nil {
		t.Fatal("DefaultBrowserToolConfig() returned nil")
	}

	if cfg.Model != "gemini-3-flash-preview" {
		t.Errorf("Model = %q, want 'gemini-3-flash-preview'", cfg.Model)
	}
	if !cfg.Headless {
		t.Error("Headless should be true by default")
	}
	if cfg.Viewport == nil {
		t.Error("Viewport should not be nil")
	}
	if cfg.ShowAnnotations {
		t.Error("ShowAnnotations should be false by default")
	}
	if cfg.Debug {
		t.Error("Debug should be false by default")
	}
}

// TestBrowserToolConfig tests custom config.
func TestBrowserToolConfig(t *testing.T) {
	cfg := &BrowserToolConfig{
		APIKey:          "test-key",
		Model:           "custom-model",
		Headless:        false,
		Viewport:        &bua.Viewport{Width: 1920, Height: 1080},
		ShowAnnotations: true,
		Debug:           true,
	}

	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want 'test-key'", cfg.APIKey)
	}
	if cfg.Model != "custom-model" {
		t.Errorf("Model = %q, want 'custom-model'", cfg.Model)
	}
	if cfg.Headless {
		t.Error("Headless should be false")
	}
	if cfg.Viewport.Width != 1920 {
		t.Errorf("Viewport.Width = %d, want 1920", cfg.Viewport.Width)
	}
	if !cfg.ShowAnnotations {
		t.Error("ShowAnnotations should be true")
	}
	if !cfg.Debug {
		t.Error("Debug should be true")
	}
}

// TestNewBrowserTool tests browser tool creation.
func TestNewBrowserTool(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		bt := NewBrowserTool(nil)
		if bt == nil {
			t.Fatal("NewBrowserTool(nil) returned nil")
		}
		if bt.config == nil {
			t.Error("config should not be nil")
		}
		if bt.config.Model != "gemini-3-flash-preview" {
			t.Errorf("config.Model = %q, want 'gemini-3-flash-preview'", bt.config.Model)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &BrowserToolConfig{
			Model:    "custom-model",
			Headless: false,
		}
		bt := NewBrowserTool(cfg)
		if bt.config.Model != "custom-model" {
			t.Errorf("config.Model = %q, want 'custom-model'", bt.config.Model)
		}
	})
}

// TestBrowserToolInput tests input struct.
func TestBrowserToolInput(t *testing.T) {
	input := BrowserToolInput{
		Task:        "Extract data from example.com",
		StartURL:    "https://example.com",
		MaxSteps:    50,
		KeepBrowser: true,
	}

	if input.Task != "Extract data from example.com" {
		t.Errorf("Task = %q", input.Task)
	}
	if input.StartURL != "https://example.com" {
		t.Errorf("StartURL = %q", input.StartURL)
	}
	if input.MaxSteps != 50 {
		t.Errorf("MaxSteps = %d, want 50", input.MaxSteps)
	}
	if !input.KeepBrowser {
		t.Error("KeepBrowser should be true")
	}
}

// TestBrowserToolOutput tests output struct.
func TestBrowserToolOutput(t *testing.T) {
	output := BrowserToolOutput{
		Success:  true,
		Message:  "Task completed",
		Data:     map[string]any{"key": "value"},
		Findings: []map[string]any{{"category": "test"}},
		FinalURL: "https://example.com/result",
		Error:    "",
	}

	if !output.Success {
		t.Error("Success should be true")
	}
	if output.Message != "Task completed" {
		t.Errorf("Message = %q", output.Message)
	}
	if output.Data["key"] != "value" {
		t.Errorf("Data[key] = %v", output.Data["key"])
	}
	if len(output.Findings) != 1 {
		t.Errorf("Findings length = %d, want 1", len(output.Findings))
	}
	if output.FinalURL != "https://example.com/result" {
		t.Errorf("FinalURL = %q", output.FinalURL)
	}
}

// TestBrowserToolClose tests close method.
func TestBrowserToolClose(t *testing.T) {
	bt := NewBrowserTool(nil)
	// Should not panic when closing without agent
	err := bt.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestNewMultiBrowserTool tests multi-browser tool creation.
func TestNewMultiBrowserTool(t *testing.T) {
	t.Run("with nil config", func(t *testing.T) {
		mbt := NewMultiBrowserTool(nil)
		if mbt == nil {
			t.Fatal("NewMultiBrowserTool(nil) returned nil")
		}
		if mbt.instances == nil {
			t.Error("instances map should be initialized")
		}
		if mbt.config.MaxConcurrentBrowsers != 3 {
			t.Errorf("MaxConcurrentBrowsers = %d, want 3", mbt.config.MaxConcurrentBrowsers)
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &MultiBrowserToolConfig{
			BrowserToolConfig:     DefaultBrowserToolConfig(),
			MaxConcurrentBrowsers: 5,
		}
		mbt := NewMultiBrowserTool(cfg)
		if mbt.config.MaxConcurrentBrowsers != 5 {
			t.Errorf("MaxConcurrentBrowsers = %d, want 5", mbt.config.MaxConcurrentBrowsers)
		}
	})
}

// TestMultiBrowserInput tests multi-browser input struct.
func TestMultiBrowserInput(t *testing.T) {
	tests := []struct {
		name   string
		input  MultiBrowserInput
		action string
	}{
		{
			name:   "create action",
			input:  MultiBrowserInput{Action: "create", StartURL: "https://example.com", ProfileName: "test"},
			action: "create",
		},
		{
			name:   "execute action",
			input:  MultiBrowserInput{Action: "execute", BrowserID: "browser_1", Task: "do something"},
			action: "execute",
		},
		{
			name:   "close action",
			input:  MultiBrowserInput{Action: "close", BrowserID: "browser_1"},
			action: "close",
		},
		{
			name:   "list action",
			input:  MultiBrowserInput{Action: "list"},
			action: "list",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input.Action != tt.action {
				t.Errorf("Action = %q, want %q", tt.input.Action, tt.action)
			}
		})
	}
}

// TestMultiBrowserOutput tests multi-browser output struct.
func TestMultiBrowserOutput(t *testing.T) {
	output := MultiBrowserOutput{
		Success:   true,
		Message:   "Browser created",
		BrowserID: "browser_1",
		Data:      map[string]any{"result": "ok"},
		Findings:  []map[string]any{},
		Browsers:  []string{"browser_1", "browser_2"},
		Error:     "",
	}

	if !output.Success {
		t.Error("Success should be true")
	}
	if output.BrowserID != "browser_1" {
		t.Errorf("BrowserID = %q", output.BrowserID)
	}
	if len(output.Browsers) != 2 {
		t.Errorf("Browsers length = %d, want 2", len(output.Browsers))
	}
}

// TestMultiBrowserToolClose tests close method.
func TestMultiBrowserToolClose(t *testing.T) {
	mbt := NewMultiBrowserTool(nil)
	// Should not panic when closing without browsers
	err := mbt.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestMultiBrowserToolListEmpty tests listing empty browsers.
func TestMultiBrowserToolListEmpty(t *testing.T) {
	mbt := NewMultiBrowserTool(nil)

	output, err := mbt.listBrowsers()
	if err != nil {
		t.Fatalf("listBrowsers() error = %v", err)
	}

	if !output.Success {
		t.Error("Success should be true")
	}
	if len(output.Browsers) != 0 {
		t.Errorf("Browsers length = %d, want 0", len(output.Browsers))
	}
	if output.Message != "Found 0 browsers" {
		t.Errorf("Message = %q", output.Message)
	}
}

// TestMultiBrowserToolExecuteNotFound tests execute with invalid browser ID.
func TestMultiBrowserToolExecuteNotFound(t *testing.T) {
	mbt := NewMultiBrowserTool(nil)

	input := MultiBrowserInput{
		Action:    "execute",
		BrowserID: "nonexistent",
		Task:      "test",
	}

	output, err := mbt.executeTasks(input)
	if err != nil {
		t.Fatalf("executeTasks() error = %v", err)
	}

	if output.Success {
		t.Error("Success should be false for nonexistent browser")
	}
	if output.Error == "" {
		t.Error("Error should not be empty")
	}
}

// TestMultiBrowserToolCloseNotFound tests close with invalid browser ID.
func TestMultiBrowserToolCloseNotFound(t *testing.T) {
	mbt := NewMultiBrowserTool(nil)

	input := MultiBrowserInput{
		Action:    "close",
		BrowserID: "nonexistent",
	}

	output, err := mbt.closeBrowser(input)
	if err != nil {
		t.Fatalf("closeBrowser() error = %v", err)
	}

	if output.Success {
		t.Error("Success should be false for nonexistent browser")
	}
}

// TestMultiBrowserToolUnknownAction tests unknown action handling.
func TestMultiBrowserToolUnknownAction(t *testing.T) {
	mbt := NewMultiBrowserTool(nil)

	input := MultiBrowserInput{
		Action: "invalid",
	}

	output, err := mbt.execute(nil, input)
	if err != nil {
		t.Fatalf("execute() error = %v", err)
	}

	if output.Success {
		t.Error("Success should be false for unknown action")
	}
	if output.Error == "" {
		t.Error("Error should not be empty")
	}
}
