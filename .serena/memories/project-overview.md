# bua-go Project Overview

## Repository
- GitHub: https://github.com/anxuanzi/bua-go
- Module: `github.com/anxuanzi/bua-go`
- Go Version: 1.25

## Description
Go library for browser automation powered by LLMs via **Google ADK (Agent Development Kit)**. Uses vision + DOM hybrid approach combining screenshots with parsed HTML and accessibility trees.

## Key Features
- **Natural Language Tasks**: Describe what you want in plain English
- **Vision + DOM Hybrid**: AI sees annotated screenshots AND element structure
- **10 Browser Tools**: click, type_text, scroll, navigate, wait, extract, get_page_state, download_file, request_human_takeover, done
- **Visual Annotations**: Colored element overlays showing what the AI sees
- **File Downloads**: Direct HTTP and authenticated CDP downloads to `~/.bua/downloads/`
- **Dual-Use Architecture**: Use as library OR embed as tool in other ADK agents
- **Session Persistence**: Browser profiles save cookies, auth, localStorage

## Package Structure
```
bua.go              # Main public API (Agent, Config, Run)
agent/
  agent.go          # ADK BrowserAgent with 10 tools
  prompts.go        # System prompts
  logger.go         # Structured emoji logging
browser/
  browser.go        # Rod wrapper, CDP operations
  annotation.go     # Visual element overlays
  download.go       # File download capability
dom/                # Element map, accessibility tree
memory/             # Short-term + long-term memory
screenshot/         # Capture + annotation (fogleman/gg)
export/
  adktool.go        # BrowserTool, MultiBrowserTool for dual-use
examples/
  simple/           # Basic search example
  scraping/         # Data extraction
  download/         # File downloads
  adk_tool/         # Dual-use architecture demo
  content_research/ # Multi-page research
```

## Key Dependencies
- `google.golang.org/adk v0.3.0` - Agent Development Kit
- `google.golang.org/genai` - Gemini API client
- `github.com/go-rod/rod` - Browser automation
- `github.com/fogleman/gg` - Image annotation

## Configuration
```go
bua.Config{
    APIKey:          "key",                    // Required
    Model:           "gemini-3-flash-preview", // Default model
    ProfileName:     "session",                // Persist browser state
    Headless:        false,                    // Show browser
    Viewport:        bua.DesktopViewport,      // 1280x800
    Debug:           true,                     // Verbose logging
    ShowAnnotations: true,                     // Visual overlays
}
```

## Viewport Presets
- DesktopViewport: 1280x800 (default)
- LargeDesktopViewport: 1920x1080
- TabletViewport: 768x1024
- MobileViewport: 375x812

## File Locations
- Browser profiles: `~/.bua/profiles/{name}/`
- Screenshots: `~/.bua/screenshots/steps/`
- Downloads: `~/.bua/downloads/`

## Environment
- `GOOGLE_API_KEY` - Required for Gemini API access

## ADK Tool Pattern
```go
handler := func(ctx tool.Context, input InputType) (OutputType, error) {
    return OutputType{...}, nil
}
tool, err := functiontool.New(functiontool.Config{
    Name: "tool_name",
    Description: "...",
}, handler)
```

## Import Alias
Local `agent` package conflicts with ADK's agent package:
```go
adkagent "google.golang.org/adk/agent"
```

## Testing
```bash
go build ./...  # Build all
go test ./...   # Run tests
```
