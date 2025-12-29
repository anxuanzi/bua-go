// Package browser provides tests for the browser automation layer.
package browser

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// TestViewport tests Viewport struct.
func TestViewport(t *testing.T) {
	v := Viewport{Width: 1920, Height: 1080}
	if v.Width != 1920 {
		t.Errorf("Width = %d, want 1920", v.Width)
	}
	if v.Height != 1080 {
		t.Errorf("Height = %d, want 1080", v.Height)
	}
}

// TestConfig tests Config struct.
func TestConfig(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		cfg := Config{}
		if cfg.Viewport != nil {
			t.Error("Viewport should be nil by default")
		}
		if cfg.ScreenshotConfig != nil {
			t.Error("ScreenshotConfig should be nil by default")
		}
	})

	t.Run("with viewport", func(t *testing.T) {
		cfg := Config{
			Viewport: &Viewport{Width: 1024, Height: 768},
		}
		if cfg.Viewport.Width != 1024 {
			t.Errorf("Viewport.Width = %d, want 1024", cfg.Viewport.Width)
		}
	})
}

// TestTabInfo tests TabInfo struct.
func TestTabInfo(t *testing.T) {
	tab := TabInfo{
		ID:    "abc123",
		URL:   "https://example.com",
		Title: "Example Domain",
	}

	if tab.ID != "abc123" {
		t.Errorf("ID = %q, want %q", tab.ID, "abc123")
	}
	if tab.URL != "https://example.com" {
		t.Errorf("URL = %q, want %q", tab.URL, "https://example.com")
	}
	if tab.Title != "Example Domain" {
		t.Errorf("Title = %q, want %q", tab.Title, "Example Domain")
	}
}

// TestNewBrowser tests browser creation without starting.
func TestNewBrowser(t *testing.T) {
	t.Run("with nil rod browser", func(t *testing.T) {
		b := New(nil, Config{})
		if b == nil {
			t.Fatal("New() returned nil")
		}
		if b.pages == nil {
			t.Error("pages map should be initialized")
		}
		if len(b.pages) != 0 {
			t.Errorf("pages should be empty, got %d", len(b.pages))
		}
	})

	t.Run("with viewport config", func(t *testing.T) {
		cfg := Config{
			Viewport: &Viewport{Width: 1920, Height: 1080},
		}
		b := New(nil, cfg)
		if b.config.Viewport == nil {
			t.Error("Viewport should be set")
		}
		if b.config.Viewport.Width != 1920 {
			t.Errorf("Viewport.Width = %d, want 1920", b.config.Viewport.Width)
		}
	})
}

// TestGetActiveTabID tests tab ID accessor.
func TestGetActiveTabID(t *testing.T) {
	b := New(nil, Config{})
	if b.GetActiveTabID() != "" {
		t.Errorf("initial activeTabID should be empty, got %q", b.GetActiveTabID())
	}
}

// Integration tests - require a real browser
// These are skipped in short mode

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

// setupBrowser creates a browser for integration tests.
func setupBrowser(t *testing.T) (*Browser, func()) {
	t.Helper()

	// Launch headless browser
	l := launcher.New().Headless(true)
	url, err := l.Launch()
	if err != nil {
		t.Fatalf("Failed to launch browser: %v", err)
	}

	rodBrowser := rod.New().ControlURL(url)
	err = rodBrowser.Connect()
	if err != nil {
		l.Kill()
		t.Fatalf("Failed to connect to browser: %v", err)
	}

	b := New(rodBrowser, Config{
		Viewport: &Viewport{Width: 1280, Height: 720},
	})

	cleanup := func() {
		rodBrowser.MustClose()
		l.Kill()
	}

	return b, cleanup
}

func TestBrowserIntegration_Navigate(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := b.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	url := b.GetURL()
	if url != "https://example.com/" {
		t.Errorf("URL = %q, want %q", url, "https://example.com/")
	}

	title := b.GetTitle()
	if title != "Example Domain" {
		t.Errorf("Title = %q, want %q", title, "Example Domain")
	}
}

func TestBrowserIntegration_MultiTab(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Navigate first tab
	err := b.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	firstTabID := b.GetActiveTabID()
	if firstTabID == "" {
		t.Fatal("First tab ID should not be empty")
	}

	// Open new tab
	secondTabID, err := b.NewTab(ctx, "https://httpbin.org/html")
	if err != nil {
		t.Fatalf("NewTab failed: %v", err)
	}

	if secondTabID == firstTabID {
		t.Error("New tab should have different ID")
	}

	if b.GetActiveTabID() != secondTabID {
		t.Error("Active tab should be the new tab")
	}

	// List tabs
	tabs := b.ListTabs(ctx)
	if len(tabs) != 2 {
		t.Errorf("Should have 2 tabs, got %d", len(tabs))
	}

	// Switch back to first tab
	err = b.SwitchTab(ctx, firstTabID)
	if err != nil {
		t.Fatalf("SwitchTab failed: %v", err)
	}

	if b.GetActiveTabID() != firstTabID {
		t.Error("Should be on first tab after switch")
	}

	// Close second tab
	err = b.CloseTab(ctx, secondTabID)
	if err != nil {
		t.Fatalf("CloseTab failed: %v", err)
	}

	tabs = b.ListTabs(ctx)
	if len(tabs) != 1 {
		t.Errorf("Should have 1 tab after close, got %d", len(tabs))
	}
}

func TestBrowserIntegration_Screenshot(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := b.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	screenshot, err := b.Screenshot(ctx)
	if err != nil {
		t.Fatalf("Screenshot failed: %v", err)
	}

	if len(screenshot) == 0 {
		t.Error("Screenshot should not be empty")
	}

	// Check it's a PNG (starts with PNG header)
	if len(screenshot) < 8 {
		t.Fatal("Screenshot too small to be valid PNG")
	}
	pngHeader := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngHeader {
		if screenshot[i] != b {
			t.Error("Screenshot is not a valid PNG")
			break
		}
	}
}

func TestBrowserIntegration_GetElementMap(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := b.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	elements, err := b.GetElementMap(ctx)
	if err != nil {
		t.Fatalf("GetElementMap failed: %v", err)
	}

	if elements == nil {
		t.Fatal("ElementMap should not be nil")
	}

	// Example.com should have at least a link and some text
	if elements.Count() == 0 {
		t.Error("ElementMap should have some elements")
	}
}

func TestBrowserIntegration_Click(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := b.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Get element map to find a clickable element
	elements, err := b.GetElementMap(ctx)
	if err != nil {
		t.Fatalf("GetElementMap failed: %v", err)
	}

	// Find first interactive element
	interactive := elements.InteractiveElements()
	if len(interactive) == 0 {
		t.Skip("No interactive elements found")
	}

	// Click on the first interactive element
	err = b.Click(ctx, interactive[0].Index)
	// This might fail depending on page layout, but shouldn't panic
	if err != nil {
		t.Logf("Click warning: %v", err)
	}
}

func TestBrowserIntegration_Type(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Navigate to a page with input
	err := b.Navigate(ctx, "https://httpbin.org/forms/post")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Get element map to find input
	elements, err := b.GetElementMap(ctx)
	if err != nil {
		t.Fatalf("GetElementMap failed: %v", err)
	}

	// Find first input element
	inputs := elements.InteractiveElements()
	var inputIndex int = -1
	for _, input := range inputs {
		if input.TagName == "input" || input.TagName == "textarea" {
			inputIndex = input.Index
			break
		}
	}

	if inputIndex == -1 {
		t.Skip("No input element found on page")
	}

	// Type into input
	err = b.TypeInElement(ctx, inputIndex, "test input")
	if err != nil {
		t.Errorf("TypeInElement failed: %v", err)
	}
}

func TestBrowserIntegration_Scroll(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Navigate to a page with content to scroll
	err := b.Navigate(ctx, "https://httpbin.org/html")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Scroll down (deltaX=0, deltaY=200)
	err = b.Scroll(ctx, 0, 200)
	if err != nil {
		t.Errorf("Scroll down failed: %v", err)
	}

	// Scroll up (deltaX=0, deltaY=-100)
	err = b.Scroll(ctx, 0, -100)
	if err != nil {
		t.Errorf("Scroll up failed: %v", err)
	}
}

func TestBrowserIntegration_WaitForStable(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := b.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	err = b.WaitForStable(ctx)
	if err != nil {
		t.Errorf("WaitForStable failed: %v", err)
	}
}

func TestBrowserIntegration_Close(t *testing.T) {
	skipIfShort(t)
	skipIfCI(t)

	b, cleanup := setupBrowser(t)
	defer cleanup()

	ctx := context.Background()
	err := b.Navigate(ctx, "https://example.com")
	if err != nil {
		t.Fatalf("Navigate failed: %v", err)
	}

	// Close should not panic
	b.Close()
}

// Benchmarks

func BenchmarkScreenshot(b *testing.B) {
	if testing.Short() {
		b.Skip("Skipping benchmark in short mode")
	}
	if os.Getenv("CI") == "true" {
		b.Skip("Skipping browser benchmark in CI")
	}

	// Setup browser
	l := launcher.New().Headless(true)
	url, err := l.Launch()
	if err != nil {
		b.Fatalf("Failed to launch browser: %v", err)
	}
	defer l.Kill()

	rodBrowser := rod.New().ControlURL(url)
	if err := rodBrowser.Connect(); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer rodBrowser.MustClose()

	browser := New(rodBrowser, Config{
		Viewport: &Viewport{Width: 1280, Height: 720},
	})

	ctx := context.Background()
	if err := browser.Navigate(ctx, "https://example.com"); err != nil {
		b.Fatalf("Navigate failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := browser.Screenshot(ctx)
		if err != nil {
			b.Fatalf("Screenshot failed: %v", err)
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

	// Setup browser
	l := launcher.New().Headless(true)
	url, err := l.Launch()
	if err != nil {
		b.Fatalf("Failed to launch browser: %v", err)
	}
	defer l.Kill()

	rodBrowser := rod.New().ControlURL(url)
	if err := rodBrowser.Connect(); err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer rodBrowser.MustClose()

	browser := New(rodBrowser, Config{
		Viewport: &Viewport{Width: 1280, Height: 720},
	})

	ctx := context.Background()
	if err := browser.Navigate(ctx, "https://example.com"); err != nil {
		b.Fatalf("Navigate failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := browser.GetElementMap(ctx)
		if err != nil {
			b.Fatalf("GetElementMap failed: %v", err)
		}
	}
}
