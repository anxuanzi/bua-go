// Package bua provides E2E/integration tests for the browser automation agent.
package bua

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"

	"github.com/anxuanzi/bua-go/dom"
	"github.com/anxuanzi/bua-go/screenshot"
)

// Test helpers

func skipIfShort(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}
}

func skipIfCI(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skip("Skipping browser test in CI environment")
	}
}

func skipIfNoAPIKey(t *testing.T) {
	if os.Getenv("GOOGLE_API_KEY") == "" {
		t.Skip("Skipping test: GOOGLE_API_KEY not set")
	}
}

// Unit tests - these don't require a browser or API key

func TestViewportPresets(t *testing.T) {
	t.Run("DesktopViewport", func(t *testing.T) {
		if DesktopViewport.Width != 1280 {
			t.Errorf("DesktopViewport.Width = %d, want 1280", DesktopViewport.Width)
		}
		if DesktopViewport.Height != 800 {
			t.Errorf("DesktopViewport.Height = %d, want 800", DesktopViewport.Height)
		}
	})

	t.Run("LargeDesktopViewport", func(t *testing.T) {
		if LargeDesktopViewport.Width != 1920 {
			t.Errorf("LargeDesktopViewport.Width = %d, want 1920", LargeDesktopViewport.Width)
		}
		if LargeDesktopViewport.Height != 1080 {
			t.Errorf("LargeDesktopViewport.Height = %d, want 1080", LargeDesktopViewport.Height)
		}
	})

	t.Run("TabletViewport", func(t *testing.T) {
		if TabletViewport.Width != 768 {
			t.Errorf("TabletViewport.Width = %d, want 768", TabletViewport.Width)
		}
		if TabletViewport.Height != 1024 {
			t.Errorf("TabletViewport.Height = %d, want 1024", TabletViewport.Height)
		}
	})

	t.Run("MobileViewport", func(t *testing.T) {
		if MobileViewport.Width != 375 {
			t.Errorf("MobileViewport.Width = %d, want 375", MobileViewport.Width)
		}
		if MobileViewport.Height != 812 {
			t.Errorf("MobileViewport.Height = %d, want 812", MobileViewport.Height)
		}
	})
}

func TestConfig(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		cfg := Config{}
		if cfg.APIKey != "" {
			t.Error("APIKey should be empty")
		}
		if cfg.Model != "" {
			t.Error("Model should be empty")
		}
		if cfg.Headless != false {
			t.Error("Headless should be false by default")
		}
		if cfg.Viewport != nil {
			t.Error("Viewport should be nil by default")
		}
	})

	t.Run("full config", func(t *testing.T) {
		cfg := Config{
			APIKey:           "test-key",
			Model:            "gemini-pro",
			ProfileName:      "test-profile",
			ProfileDir:       "/tmp/profiles",
			Headless:         true,
			Viewport:         &Viewport{Width: 1024, Height: 768},
			ScreenshotConfig: &screenshot.Config{StorageDir: "/tmp/screenshots"},
			MaxTokens:        500000,
			Debug:            true,
			ShowAnnotations:  true,
		}

		if cfg.APIKey != "test-key" {
			t.Errorf("APIKey = %q", cfg.APIKey)
		}
		if cfg.Model != "gemini-pro" {
			t.Errorf("Model = %q", cfg.Model)
		}
		if cfg.ProfileName != "test-profile" {
			t.Errorf("ProfileName = %q", cfg.ProfileName)
		}
		if cfg.ProfileDir != "/tmp/profiles" {
			t.Errorf("ProfileDir = %q", cfg.ProfileDir)
		}
		if !cfg.Headless {
			t.Error("Headless should be true")
		}
		if cfg.Viewport.Width != 1024 {
			t.Errorf("Viewport.Width = %d", cfg.Viewport.Width)
		}
		if cfg.MaxTokens != 500000 {
			t.Errorf("MaxTokens = %d", cfg.MaxTokens)
		}
		if !cfg.Debug {
			t.Error("Debug should be true")
		}
		if !cfg.ShowAnnotations {
			t.Error("ShowAnnotations should be true")
		}
	})
}

func TestResult(t *testing.T) {
	t.Run("success result", func(t *testing.T) {
		result := Result{
			Success: true,
			Data:    map[string]string{"key": "value"},
			Error:   "",
		}
		if !result.Success {
			t.Error("Success should be true")
		}
		if result.Error != "" {
			t.Errorf("Error should be empty, got %q", result.Error)
		}
		data, ok := result.Data.(map[string]string)
		if !ok {
			t.Error("Data should be map[string]string")
		}
		if data["key"] != "value" {
			t.Errorf("Data[key] = %q", data["key"])
		}
	})

	t.Run("error result", func(t *testing.T) {
		result := Result{
			Success: false,
			Data:    nil,
			Error:   "Task failed",
		}
		if result.Success {
			t.Error("Success should be false")
		}
		if result.Error != "Task failed" {
			t.Errorf("Error = %q", result.Error)
		}
	})
}

func TestNewWithEmptyAPIKey(t *testing.T) {
	_, err := New(Config{})
	if err == nil {
		t.Error("New() with empty API key should error")
	}
}

// Integration tests - require browser but not API key

func TestAgentWithBrowser(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	// Create a headless browser directly for testing browser operations
	l := launcher.New().Headless(true)
	url, err := l.Launch()
	if err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer l.Kill()

	rodBrowser := rod.New().ControlURL(url)
	if err := rodBrowser.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer rodBrowser.MustClose()

	page := rodBrowser.MustPage("https://example.com")
	defer page.MustClose()

	// Test page operations
	t.Run("page title", func(t *testing.T) {
		title := page.MustInfo().Title
		if title != "Example Domain" {
			t.Errorf("Title = %q, want 'Example Domain'", title)
		}
	})

	t.Run("page url", func(t *testing.T) {
		pageURL := page.MustInfo().URL
		if pageURL != "https://example.com/" {
			t.Errorf("URL = %q, want 'https://example.com/'", pageURL)
		}
	})
}

// E2E tests - require both browser and API key

func TestAgentE2E(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)
	skipIfNoAPIKey(t)

	cfg := Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Headless: true,
		Viewport: DesktopViewport,
		Debug:    testing.Verbose(),
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	t.Run("navigate", func(t *testing.T) {
		if err := agent.Navigate(ctx, "https://example.com"); err != nil {
			t.Fatalf("Navigate() error: %v", err)
		}
	})

	t.Run("screenshot", func(t *testing.T) {
		screenshot, err := agent.Screenshot(ctx)
		if err != nil {
			t.Fatalf("Screenshot() error: %v", err)
		}
		if len(screenshot) == 0 {
			t.Error("Screenshot should not be empty")
		}
		// Verify PNG header
		if len(screenshot) < 8 {
			t.Fatal("Screenshot too small")
		}
		pngHeader := []byte{0x89, 0x50, 0x4E, 0x47}
		for i, b := range pngHeader {
			if screenshot[i] != b {
				t.Error("Screenshot is not a valid PNG")
				break
			}
		}
	})

	t.Run("get element map", func(t *testing.T) {
		elements, err := agent.GetElementMap(ctx)
		if err != nil {
			t.Fatalf("GetElementMap() error: %v", err)
		}
		if elements == nil {
			t.Fatal("ElementMap should not be nil")
		}
		if elements.Count() == 0 {
			t.Error("ElementMap should have elements")
		}
	})

	t.Run("get accessibility tree", func(t *testing.T) {
		tree, err := agent.GetAccessibilityTree(ctx)
		if err != nil {
			t.Fatalf("GetAccessibilityTree() error: %v", err)
		}
		if tree == nil {
			t.Fatal("AccessibilityTree should not be nil")
		}
	})

	t.Run("page accessor", func(t *testing.T) {
		page := agent.Page()
		if page == nil {
			t.Fatal("Page() should not return nil after Start()")
		}
		title := page.MustInfo().Title
		if title != "Example Domain" {
			t.Errorf("Page title = %q, want 'Example Domain'", title)
		}
	})

	t.Run("get agent", func(t *testing.T) {
		browserAgent := agent.GetAgent()
		if browserAgent == nil {
			t.Error("GetAgent() should not return nil")
		}
	})

	t.Run("get findings", func(t *testing.T) {
		findings := agent.GetFindings()
		if findings == nil {
			t.Error("GetFindings() should not return nil")
		}
		// Findings should be empty initially
		if len(findings) != 0 {
			t.Errorf("Initial findings should be empty, got %d", len(findings))
		}
	})
}

func TestAgentRunSimpleTask(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)
	skipIfNoAPIKey(t)

	cfg := Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Headless: true,
		Viewport: DesktopViewport,
		Debug:    testing.Verbose(),
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Run a simple extraction task
	result, err := agent.Run(ctx, "Go to https://example.com and extract the main heading text. Return the text as a JSON object with key 'heading'.")
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if result == nil {
		t.Fatal("Result should not be nil")
	}

	t.Logf("Result: success=%v, error=%q, data=%v", result.Success, result.Error, result.Data)

	if !result.Success && result.Error != "" {
		t.Logf("Task completed with error: %s", result.Error)
	}
}

func TestAgentMultipleNavigations(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)
	skipIfNoAPIKey(t)

	cfg := Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Headless: true,
		Viewport: DesktopViewport,
		Debug:    testing.Verbose(),
	}

	agent, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := agent.Start(ctx); err != nil {
		t.Fatalf("Start() error: %v", err)
	}

	// Navigate to multiple URLs
	urls := []string{
		"https://example.com",
		"https://httpbin.org/html",
	}

	for _, url := range urls {
		t.Run(url, func(t *testing.T) {
			if err := agent.Navigate(ctx, url); err != nil {
				t.Errorf("Navigate(%s) error: %v", url, err)
			}

			// Verify navigation by checking URL
			page := agent.Page()
			if page == nil {
				t.Fatal("Page is nil")
			}
			currentURL := page.MustInfo().URL
			if currentURL == "" {
				t.Error("Current URL should not be empty")
			}
			t.Logf("Navigated to: %s", currentURL)
		})
	}
}

// Annotation tests

func TestAnnotationConfig(t *testing.T) {
	t.Run("default config", func(t *testing.T) {
		cfg := AnnotationConfig{}
		// Default values should be zero values
		if cfg.ShowIndex != false {
			t.Error("ShowIndex should be false by default")
		}
	})

	t.Run("custom config", func(t *testing.T) {
		cfg := AnnotationConfig{
			ShowIndex: true,
		}
		if !cfg.ShowIndex {
			t.Error("ShowIndex should be true")
		}
	})
}

// DOM integration tests

func TestElementMapIntegration(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	// Test that ElementMap works correctly with browser
	l := launcher.New().Headless(true)
	url, err := l.Launch()
	if err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}
	defer l.Kill()

	rodBrowser := rod.New().ControlURL(url)
	if err := rodBrowser.Connect(); err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer rodBrowser.MustClose()

	page := rodBrowser.MustPage("https://example.com")
	defer page.MustClose()

	// Wait for page to load
	page.MustWaitLoad()

	// Get elements using the DOM package
	elements := dom.NewElementMap()
	elements.PageTitle = page.MustInfo().Title
	elements.PageURL = page.MustInfo().URL

	if elements.PageTitle != "Example Domain" {
		t.Errorf("PageTitle = %q, want 'Example Domain'", elements.PageTitle)
	}

	if elements.PageURL != "https://example.com/" {
		t.Errorf("PageURL = %q, want 'https://example.com/'", elements.PageURL)
	}
}

// Benchmarks

func BenchmarkNavigate(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}
	if os.Getenv("CI") == "true" {
		b.Skip("Skipping browser benchmark in CI")
	}
	if os.Getenv("GOOGLE_API_KEY") == "" {
		b.Skip("Skipping benchmark: GOOGLE_API_KEY not set")
	}

	cfg := Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Headless: true,
		Viewport: DesktopViewport,
	}

	agent, err := New(cfg)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer agent.Close()

	ctx := context.Background()
	if err := agent.Start(ctx); err != nil {
		b.Fatalf("Start() error: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := agent.Navigate(ctx, "https://example.com"); err != nil {
			b.Fatalf("Navigate() error: %v", err)
		}
	}
}

func BenchmarkScreenshot(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}
	if os.Getenv("CI") == "true" {
		b.Skip("Skipping browser benchmark in CI")
	}
	if os.Getenv("GOOGLE_API_KEY") == "" {
		b.Skip("Skipping benchmark: GOOGLE_API_KEY not set")
	}

	cfg := Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Headless: true,
		Viewport: DesktopViewport,
	}

	agent, err := New(cfg)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer agent.Close()

	ctx := context.Background()
	if err := agent.Start(ctx); err != nil {
		b.Fatalf("Start() error: %v", err)
	}
	if err := agent.Navigate(ctx, "https://example.com"); err != nil {
		b.Fatalf("Navigate() error: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := agent.Screenshot(ctx)
		if err != nil {
			b.Fatalf("Screenshot() error: %v", err)
		}
	}
}

func BenchmarkGetElementMap(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}
	if os.Getenv("CI") == "true" {
		b.Skip("Skipping browser benchmark in CI")
	}
	if os.Getenv("GOOGLE_API_KEY") == "" {
		b.Skip("Skipping benchmark: GOOGLE_API_KEY not set")
	}

	cfg := Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Headless: true,
		Viewport: DesktopViewport,
	}

	agent, err := New(cfg)
	if err != nil {
		b.Fatalf("New() error: %v", err)
	}
	defer agent.Close()

	ctx := context.Background()
	if err := agent.Start(ctx); err != nil {
		b.Fatalf("Start() error: %v", err)
	}
	if err := agent.Navigate(ctx, "https://example.com"); err != nil {
		b.Fatalf("Navigate() error: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := agent.GetElementMap(ctx)
		if err != nil {
			b.Fatalf("GetElementMap() error: %v", err)
		}
	}
}
