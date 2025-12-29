# BrowserUse Agent - Golang

> Browser automation powered by AI — just tell it what to do in plain English.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## What is bua-go?

bua-go is a Go library that lets you automate web browsers using natural language. Instead of writing complex selectors
and scripts, just describe what you want:

```go
agent.Run(ctx, "Search for 'golang' and click the first result")
```

The AI sees the page (screenshots + DOM), decides what to click/type/scroll, and executes the actions for you.

## Features

| Feature                      | Description                                        |
|------------------------------|----------------------------------------------------|
| **Natural Language**         | Describe tasks in plain English                    |
| **Vision + DOM**             | AI sees both screenshots and page structure        |
| **Google ADK**               | Powered by Gemini via Agent Development Kit        |
| **Token Presets**            | Optimize for speed, cost, or quality               |
| **TextOnly Mode**            | Skip screenshots for fastest operation             |
| **Session Memory**           | Remembers cookies, logins, and patterns            |
| **Headless Mode**            | Run invisibly in the background                    |
| **Viewport Presets**         | Desktop, tablet, and mobile sizes                  |
| **Visual Annotations**       | See what the AI sees with colored element overlays |
| **File Downloads**           | Download files with authentication support         |
| **Dual-Use Architecture**    | Use as library OR as tool in other ADK agents      |

## Installation

```bash
go get github.com/anxuanzi/bua-go
```

**Requirements:**

- Go 1.25+
- [Google API Key](https://aistudio.google.com/) (for Gemini)
- Chrome/Chromium (auto-managed)

## Quick Start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/anxuanzi/bua-go"
)

func main() {
	cfg := bua.Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Model:    bua.ModelGemini3Flash, // Use model constants
		Headless: false,
	}
	// Optional: Apply token preset for faster operation
	// cfg.ApplyTokenPreset(bua.TokenPresetTextOnly)

	agent, err := bua.New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	agent.Start(ctx)
	agent.Navigate(ctx, "https://www.google.com")

	result, _ := agent.Run(ctx, "Search for 'Go programming' and click the first result")
	fmt.Printf("Done! Steps: %d, Tokens: %d\n", len(result.Steps), result.TokensUsed)
}
```

## Web Scraping Example

```go
// Use TextOnly mode for fast text extraction
cfg := bua.Config{
	APIKey: os.Getenv("GOOGLE_API_KEY"),
	Model:  bua.ModelGemini25Flash,
}
cfg.ApplyTokenPreset(bua.TokenPresetTextOnly) // No screenshots, fastest

result, _ := agent.Run(ctx, `
    Go to news.ycombinator.com and extract the top 5 story titles and URLs.
    Return as JSON array with 'title' and 'url' fields.
`)

data, _ := json.MarshalIndent(result.Data, "", "  ")
fmt.Println(string(data))
```

## Configuration

### Basic Configuration

```go
bua.Config{
    APIKey:          "your-api-key",        // Required: Google API key
    Model:           bua.ModelGemini3Flash, // Model constant (see below)
    ProfileName:     "my-session",          // Persist cookies/logins (optional)
    Headless:        true,                  // Run without browser window
    Viewport:        bua.DesktopViewport,   // Or TabletViewport, MobileViewport
    Debug:           true,                  // Verbose logging
    ShowAnnotations: true,                  // Visual element overlays
}
```

### Model Constants

All Gemini 2.x/3.x models have 1M token context window.

| Constant | Model ID | Description |
|----------|----------|-------------|
| `bua.ModelGemini3Pro` | gemini-3-pro-preview | Latest, best multimodal |
| `bua.ModelGemini3Flash` | gemini-3-flash-preview | Fast, 1M context |
| `bua.ModelGemini25Pro` | gemini-2.5-pro | Stable, production-ready |
| `bua.ModelGemini25Flash` | gemini-2.5-flash | Fast & efficient |
| `bua.ModelGemini25FlashLite` | gemini-2.5-flash-lite | Most cost-effective |
| `bua.ModelGemini20Flash` | gemini-2.0-flash | Previous gen, stable |

### Token Management & Presets

Control token usage and speed with presets:

```go
cfg := bua.Config{
    APIKey: apiKey,
    Model:  bua.ModelGemini25Flash,
}

// Apply a preset
cfg.ApplyTokenPreset(bua.TokenPresetTextOnly) // Fastest, no screenshots
```

| Preset | Screenshots | Tokens/Page | Speed | Best For |
|--------|-------------|-------------|-------|----------|
| `TokenPresetTextOnly` | None | ~5-15K | Fastest | Text extraction, form filling, scraping |
| `TokenPresetEfficient` | 640px, q50 | ~15-25K | Fast | High-volume automation, cost savings |
| `TokenPresetBalanced` | 800px, q60 | ~25-40K | Normal | Most tasks (default behavior) |
| `TokenPresetQuality` | 1024px, q75 | ~40-60K | Slower | Complex UIs, visual verification |
| `TokenPresetMaximum` | 1280px, q85 | ~60-100K | Slowest | Debugging, maximum accuracy |

### Manual Token Configuration

For fine-grained control:

```go
bua.Config{
    // Element map limits
    MaxElements: 150,        // Max interactive elements sent to LLM (default: 150)

    // Screenshot compression
    ScreenshotMaxWidth: 800, // Resize width in pixels (default: 800)
    ScreenshotQuality:  60,  // JPEG quality 1-100 (default: 60)

    // Disable screenshots entirely
    TextOnly: true,          // Fastest mode, element map only
}
```

### TextOnly Mode

For maximum speed when visual context isn't needed:

```go
cfg := bua.Config{
    APIKey:   apiKey,
    Model:    bua.ModelGemini3Flash,
    TextOnly: true, // No screenshots, relies on element map text
}
```

**Benefits:**
- Eliminates screenshot capture/encoding overhead (~5-10s per step)
- Reduces tokens by 50-80%
- Ideal for: text extraction, form filling, simple navigation, high-speed scraping

**Trade-offs:**
- No visual context for the AI
- May struggle with complex visual layouts
- Best for well-structured pages with clear text labels

### Viewport Sizes

| Preset | Size |
|--------|------|
| `bua.DesktopViewport` | 1280×800 |
| `bua.LargeDesktopViewport` | 1920×1080 |
| `bua.TabletViewport` | 768×1024 |
| `bua.MobileViewport` | 375×812 |

## Available Actions

The AI can perform these actions (10 tools):

| Action | What it does |
|--------|--------------|
| `click` | Click on elements |
| `type_text` | Type into inputs |
| `scroll` | Scroll page or specific container (modals, sidebars) |
| `navigate` | Go to a URL |
| `wait` | Wait for page to load |
| `extract` | Pull data from page |
| `get_page_state` | Get current URL, title, elements (optional screenshot) |
| `download_file` | Download files (with auth support) |
| `request_human_takeover` | Ask for human help (CAPTCHA, etc.) |
| `done` | Complete the task |

## How It Works

```
You: "Click the login button"
         ↓
    📸 Screenshot + 🌳 DOM Tree (or TextOnly: DOM only)
         ↓
    🤖 AI analyzes the page
         ↓
    🎯 Finds "Login" button at index [3]
         ↓
    🖱️ Clicks element [3]
         ↓
    ✅ Reports success
```

The AI uses a **hybrid approach**:

- **Vision**: Sees the page layout via compressed screenshots (unless TextOnly)
- **DOM**: Understands element structure and semantics via element map

## Browser Profiles

Keep your sessions alive across runs:

```go
cfg := bua.Config{
    ProfileName: "my-account", // Saves to ~/.bua/profiles/my-account/
}
```

Persists: cookies, localStorage, auth tokens, IndexedDB

## Debugging

```go
cfg := bua.Config{
    Debug:           true, // See what the AI is thinking
    ShowAnnotations: true, // Visual element overlays (requires Headless: false)
}
```

Screenshots are saved to `~/.bua/screenshots/steps/` with annotations showing element indices.

## Dual-Use Architecture

bua-go can be used as a **tool within other ADK agents**:

```go
import "github.com/anxuanzi/bua-go/export"

// Create browser tool
browserTool := export.NewBrowserTool(&export.BrowserToolConfig{
    APIKey: apiKey,
    Model:  "gemini-3-flash-preview",
})
defer browserTool.Close()

// Get ADK tool and add to your agent
adkTool, _ := browserTool.Tool()

myAgent, _ := llmagent.New(llmagent.Config{
    Name:  "my_agent",
    Model: model,
    Tools: []tool.Tool{adkTool, otherTools...},
})
```

### Multi-Browser Support

For parallel browser operations:

```go
multiBrowser := export.NewMultiBrowserTool(&export.MultiBrowserToolConfig{
    BrowserToolConfig:     export.DefaultBrowserToolConfig(),
    MaxConcurrentBrowsers: 3,
})
// Actions: create, execute, close, list
```

## Downloads

Files are downloaded to `~/.bua/downloads/`:

```go
result, _ := agent.Run(ctx, `
    Go to example.com/files and download the PDF report.
    Use the download_file tool with the file URL.
`)
```

## Rate Limiting

The agent automatically handles Gemini API rate limits (429 errors):
- Parses retry delay from error response
- Waits and retries automatically
- Add delays between tasks for high-volume operations

## API Reference

```go
// Create agent
agent, err := bua.New(cfg)

// Start browser
agent.Start(ctx)

// Navigate
agent.Navigate(ctx, "https://example.com")

// Run natural language task
result, err := agent.Run(ctx, "fill out the contact form")

// Result contains:
// - result.Success     bool
// - result.Data        any (extracted data)
// - result.Steps       []Step (action history)
// - result.TokensUsed  int
// - result.Duration    time.Duration
// - result.Confidence  *TaskConfidence

// Take screenshot
screenshot, err := agent.Screenshot(ctx)

// Get element map
elements, err := agent.GetElementMap(ctx)

// Show/hide annotations
agent.ShowAnnotations(ctx, nil)
agent.HideAnnotations(ctx)

// Cleanup
agent.Close()
```

## Testing

```bash
go test ./...        # Run tests
go test -v ./...     # Verbose output
```

## Contributing

1. Fork the repo
2. Make your changes
3. Run `go fmt ./...` and `go vet ./...`
4. Submit a PR

## License

MIT License — see [LICENSE](LICENSE)

## Credits

- [go-rod/rod](https://github.com/go-rod/rod) — Browser automation
- [Google ADK](https://google.golang.org/adk) — Agent Development Kit
- [fogleman/gg](https://github.com/fogleman/gg) — Screenshot annotation
