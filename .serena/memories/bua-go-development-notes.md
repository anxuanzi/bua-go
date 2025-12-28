# bua-go Development Notes

## Project Overview
Browser automation library using Google ADK (Agent Development Kit) for Go with go-rod.
- **Package**: `github.com/anxuanzi/bua-go`
- **Purpose**: Vision + DOM hybrid browser automation powered by LLMs (Gemini)

## Key Architecture

### Core Components
- `bua.go` - Main Agent orchestration, Config, Result types
- `agent/agent.go` - ADK BrowserAgent with tool definitions
- `browser/browser.go` - go-rod browser wrapper with viewport handling
- `dom/` - Element map and accessibility tree extraction
- `memory/` - Session memory management
- `screenshot/` - Screenshot capture and annotation

### Dependencies
- `google.golang.org/adk` - Google Agent Development Kit
- `google.golang.org/genai` - Gemini AI client
- `github.com/go-rod/rod` - Browser automation

## Critical Fix: Browser Viewport Responsiveness

### Problem
Content wasn't rendering at correct responsive size - appeared small or didn't fit window.

### Solution
**Window size AND viewport must match** for proper responsive behavior:

1. **Launcher flags** (bua.go:201-208):
```go
a.launcher = launcher.New().
    Set("window-size", fmt.Sprintf("%d,%d", a.config.Viewport.Width, a.config.Viewport.Height)).
    Headless(a.config.Headless)
```

2. **CDP viewport emulation** (browser/browser.go:69-78):
```go
err := b.page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
    Width:             b.config.Viewport.Width,
    Height:            b.config.Viewport.Height,
    DeviceScaleFactor: 1.0,
    Mobile:            false,
})
```

### Viewport Presets (bua.go:74-83)
```go
DesktopViewport = &Viewport{Width: 1280, Height: 800}      // Safe laptop default
LargeDesktopViewport = &Viewport{Width: 1920, Height: 1080} // Full HD
TabletViewport = &Viewport{Width: 768, Height: 1024}
MobileViewport = &Viewport{Width: 375, Height: 812}
```

## ADK Event Processing

### Data Extraction from Tool Calls
Agent returns data via `done` tool's FunctionCall args, not text parts:

```go
if part.FunctionCall != nil && part.FunctionCall.Name == "done" {
    args := part.FunctionCall.Args
    if data, exists := args["extracted_data"]; exists {
        if dataStr, ok := data.(string); ok {
            extractedData = dataStr
        }
    }
    if summary, exists := args["summary"]; exists {
        if summaryStr, ok := summary.(string); ok {
            doneSummary = summaryStr
        }
    }
    if success, exists := args["success"]; exists {
        if successBool, ok := success.(bool); ok {
            result.Success = successBool
        }
    }
}
```

### Debug Logging (bua.go:328-344)
Comprehensive event logging for troubleshooting:
- Event Author and Partial status
- Text parts (truncated to 200 chars)
- FunctionCall name and args
- FunctionResponse name and response

## Browser Tools (agent/agent.go)
- `click` - Click element by index
- `type_text` - Type into input field
- `scroll` - Scroll page up/down
- `navigate` - Go to URL
- `wait` - Wait for page stability
- `extract` - Extract element/page data
- `get_page_state` - Get URL, title, element map
- `request_human_takeover` - Request human intervention
- `done` - Signal task completion with extracted data

## Examples
- `examples/simple/main.go` - Basic Google search
- `examples/multipage/main.go` - Multi-page workflow (Wikipedia, GitHub, go.dev)

## Environment
- Requires `GOOGLE_API_KEY` environment variable
- Default model: `gemini-2.5-flash`
- Default max tokens: 128000

## Common Issues & Solutions

| Issue | Solution |
|-------|----------|
| Empty result data | Capture from `done` tool FunctionCall.Args |
| Content too small/wrong size | Match window-size AND viewport dimensions |
| 1920x1080 too large | Use 1280x800 DesktopViewport default |
| Browser detection | Launcher flags disable automation indicators |
