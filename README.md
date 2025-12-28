# 🤖 BrowserUse Agent - Golang

> Browser automation powered by AI — just tell it what to do in plain English.

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

## ✨ What is bua-go?

bua-go is a Go library that lets you automate web browsers using natural language. Instead of writing complex selectors
and scripts, just describe what you want:

```go
agent.Run(ctx, "Search for 'golang' and click the first result")
```

The AI sees the page (screenshots + DOM), decides what to click/type/scroll, and executes the actions for you.

## 🎯 Features

| Feature                      | Description                                        |
|------------------------------|----------------------------------------------------|
| 🗣️ **Natural Language**     | Describe tasks in plain English                    |
| 👁️ **Vision + DOM**         | AI sees both screenshots and page structure        |
| 🧠 **Google ADK**            | Powered by Gemini via Agent Development Kit        |
| 💾 **Session Memory**        | Remembers cookies, logins, and patterns            |
| 👻 **Headless Mode**         | Run invisibly in the background                    |
| 📱 **Viewport Presets**      | Desktop, tablet, and mobile sizes                  |
| 🏷️ **Visual Annotations**   | See what the AI sees with colored element overlays |
| 📥 **File Downloads**        | Download files with authentication support         |
| 🔧 **Dual-Use Architecture** | Use as library OR as tool in other ADK agents      |

## 📦 Installation

```bash
go get github.com/anxuanzi/bua-go
```

**Requirements:**

- Go 1.25+
- [Google API Key](https://aistudio.google.com/) (for Gemini)
- Chrome/Chromium (auto-managed)

## 🚀 Quick Start

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
	agent, err := bua.New(bua.Config{
		APIKey:   os.Getenv("GOOGLE_API_KEY"),
		Model:    "gemini-3-flash-preview",
		Headless: false,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	agent.Start(ctx)
	agent.Navigate(ctx, "https://www.google.com")

	result, _ := agent.Run(ctx, "Search for 'Go programming' and click the first result")
	fmt.Printf("✅ Done! Steps taken: %d\n", len(result.Steps))
}
```

## 🕷️ Web Scraping Example

```go
result, _ := agent.Run(ctx, `
    Go to news.ycombinator.com and extract the top 5 story titles and URLs.
    Return as JSON array with 'title' and 'url' fields.
`)

data, _ := json.MarshalIndent(result.Data, "", "  ")
fmt.Println(string(data))
```

## ⚙️ Configuration

```go
bua.Config{
APIKey:          "your-api-key",           // Required: Google API key
Model:           "gemini-3-flash-preview", // Latest model
ProfileName:     "my-session", // Persist cookies/logins (optional)
Headless:        true,         // Run without browser window
Viewport:        bua.DesktopViewport, // Or TabletViewport, MobileViewport
Debug:           true,                // Verbose logging
ShowAnnotations: true, // Visual element overlays
}
```

### 📐 Viewport Sizes

| Preset                | Size     |
|-----------------------|----------|
| `bua.DesktopViewport` | 1280×800 |
| `bua.TabletViewport`  | 768×1024 |
| `bua.MobileViewport`  | 375×812  |

## 🛠️ Available Actions

The AI can perform these actions (10 tools):

| Action                   | What it does                       |
|--------------------------|------------------------------------|
| `click`                  | Click on elements                  |
| `type_text`              | Type into inputs                   |
| `scroll`                 | Scroll up/down                     |
| `navigate`               | Go to a URL                        |
| `wait`                   | Wait for page to load              |
| `extract`                | Pull data from page                |
| `get_page_state`         | Get current URL, title, elements   |
| `download_file`          | Download files (with auth support) |
| `request_human_takeover` | Ask for human help (CAPTCHA, etc.) |
| `done`                   | Complete the task                  |

## 🔍 How It Works

```
You: "Click the login button"
         ↓
    📸 Screenshot + 🌳 DOM Tree
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

- **Vision**: Sees the page layout via annotated screenshots
- **DOM**: Understands element structure and semantics

## 💾 Browser Profiles

Keep your sessions alive across runs:

```go
cfg := bua.Config{
ProfileName: "my-account", // Saves to ~/.bua/profiles/my-account/
}
```

Persists: cookies, localStorage, auth tokens, IndexedDB

## 🐛 Debugging

```go
cfg := bua.Config{
Debug:           true, // See what the AI is thinking
ShowAnnotations: true, // Visual element overlays (requires Headless: false)
}
```

Screenshots are saved to `~/.bua/screenshots/steps/` with annotations showing element indices.

## 🔧 Dual-Use Architecture

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

## 📥 Downloads

Files are downloaded to `~/.bua/downloads/`:

```go
result, _ := agent.Run(ctx, `
    Go to example.com/files and download the PDF report.
    Use the download_file tool with the file URL.
`)
```

## 📖 API Reference

```go
// Create agent
agent, err := bua.New(cfg)

// Start browser
agent.Start(ctx)

// Navigate
agent.Navigate(ctx, "https://example.com")

// Run natural language task
result, err := agent.Run(ctx, "fill out the contact form")

// Take screenshot
screenshot, err := agent.Screenshot(ctx)

// Cleanup
agent.Close()
```

## 🧪 Testing

```bash
go test ./...        # Run tests
go test -v ./...     # Verbose output
```

## 🤝 Contributing

1. Fork the repo
2. Make your changes
3. Run `go fmt ./...` and `go vet ./...`
4. Submit a PR

## 📄 License

MIT License — see [LICENSE](LICENSE)

## 🙏 Credits

- [go-rod/rod](https://github.com/go-rod/rod) — Browser automation
- [Google ADK](https://google.golang.org/adk) — Agent Development Kit
- [fogleman/gg](https://github.com/fogleman/gg) — Screenshot annotation
