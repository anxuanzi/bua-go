# bua-go Development Notes

## Project Overview
Browser automation library using Google ADK (Agent Development Kit) for Go with go-rod.
- **Package**: `github.com/anxuanzi/bua-go`
- **Purpose**: Vision + DOM hybrid browser automation powered by LLMs (Gemini)

## Key Architecture

### Core Components
- `bua.go` - Main Agent orchestration, Config, Result types
- `agent/agent.go` - ADK BrowserAgent with tool definitions
- `agent/logger.go` - Structured logging with emojis
- `browser/browser.go` - go-rod browser wrapper with viewport handling
- `browser/annotation.go` - Visual element annotation overlays
- `dom/` - Element map and accessibility tree extraction
- `memory/` - Session memory management
- `screenshot/` - Screenshot capture and annotation

### Dependencies
- `google.golang.org/adk` - Google Agent Development Kit
- `google.golang.org/genai` - Gemini AI client
- `github.com/go-rod/rod` - Browser automation

## Configuration

### bua.Config Fields
```go
type Config struct {
    APIKey           string              // Gemini API key (required)
    Model            string              // Model ID (default: gemini-3-flash-preview)
    ProfileName      string              // Browser profile name for persistence
    ProfileDir       string              // Profile storage directory
    Headless         bool                // Run headless (default: false)
    Viewport         *Viewport           // Browser size (default: 1280x800)
    ScreenshotConfig *screenshot.Config  // Screenshot settings
    MemoryConfig     *memory.Config      // Memory settings
    MaxTokens        int                 // Token limit (default: 1048576)
    Debug            bool                // Enable debug logging
    ShowAnnotations  bool                // Show visual annotations before actions
}
```

### ShowAnnotations Feature
When `ShowAnnotations: true`:
1. Before each action (click, type, scroll), annotations are drawn on all detected elements
2. Annotated screenshots are saved to `~/.bua/screenshots/steps/`
3. Screenshots named: `step_001_click_150405.png`
4. Annotations are hidden after each action

## Debug Logging System

### Logger Features (agent/logger.go)
- Structured output with box-drawing characters
- Emoji indicators for each action type
- Step numbering
- Timestamps

### Log Output Example
```
┌─────────────────────────────────────────────────────────────────
│ 🎯 STEP 1 │ 15:04:05
├─────────────────────────────────────────────────────────────────
│ 🔧 Action:    CLICK
│ 🎪 Target:    Element #5
│ 💭 Reasoning: Click the search button to submit query
└─────────────────────────────────────────────────────────────────
   🏷️  Showing annotations for 42 elements
   📸 Screenshot (annotated): ~/.bua/screenshots/steps/step_001_click_150405.png
   ✅ Clicked element 5
```

### Emoji Legend
| Emoji | Meaning |
|-------|---------|
| 🎯 | Step header |
| 🔧 | Action type |
| 🎪 | Target element |
| 💭 | Reasoning |
| 🌐 | Navigate |
| ⏳ | Wait |
| 📄 | Page info |
| 🔗 | URL |
| 🧩 | Element count |
| 📸 | Screenshot |
| 🏷️ | Annotations |
| ✅ | Success |
| ❌ | Failure |
| ⚠️ | Error |
| 🙋 | Human takeover |

## Browser Annotations

### Color Coding
| Color  | Element Type |
|--------|--------------|
| Red    | Buttons      |
| Blue   | Links (a)    |
| Green  | Inputs       |
| Purple | Selects      |
| Teal   | Textareas    |
| Orange | Images       |
| Gray   | Other        |

### Usage
```go
cfg := bua.Config{
    APIKey:          apiKey,
    Debug:           true,           // Enable logging
    ShowAnnotations: true,           // Enable visual annotations
    Headless:        false,          // Must be visible to see annotations
}
```

## Critical Fix: Browser Viewport Responsiveness

**Window size AND viewport must match** for proper responsive behavior:

1. **Launcher flags** (bua.go):
```go
a.launcher = launcher.New().
    Set("window-size", fmt.Sprintf("%d,%d", a.config.Viewport.Width, a.config.Viewport.Height)).
    Headless(a.config.Headless)
```

2. **CDP viewport emulation** (browser/browser.go):
```go
err := b.page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
    Width:             b.config.Viewport.Width,
    Height:            b.config.Viewport.Height,
    DeviceScaleFactor: 1.0,
    Mobile:            false,
})
```

### Viewport Presets
```go
DesktopViewport = &Viewport{Width: 1280, Height: 800}      // Safe laptop default
LargeDesktopViewport = &Viewport{Width: 1920, Height: 1080} // Full HD
TabletViewport = &Viewport{Width: 768, Height: 1024}
MobileViewport = &Viewport{Width: 375, Height: 812}
```

## ADK Event Processing

Agent returns data via `done` tool's FunctionCall args:
```go
if part.FunctionCall != nil && part.FunctionCall.Name == "done" {
    args := part.FunctionCall.Args
    extractedData = args["extracted_data"].(string)
    doneSummary = args["summary"].(string)
    result.Success = args["success"].(bool)
}
```

## Browser Tools
- `click` - Click element by index
- `type_text` - Type into input field
- `scroll` - Scroll page up/down
- `navigate` - Go to URL
- `wait` - Wait for page stability
- `extract` - Extract element/page data
- `get_page_state` - Get URL, title, element map
- `request_human_takeover` - Request human intervention
- `done` - Signal task completion

## Examples
- `examples/simple/main.go` - Basic Google search
- `examples/multipage/main.go` - Multi-page workflow
- `examples/annotations/main.go` - Interactive annotation demo

## Environment
- Requires `GOOGLE_API_KEY` environment variable
- Default model: `gemini-3-flash-preview`
- Input token limit: 1,048,576
- Output token limit: 65,536 (configured as 16,384 conservative)

## Common Issues & Solutions

| Issue | Solution |
|-------|----------|
| Empty result data | Capture from `done` tool FunctionCall.Args |
| Content too small | Match window-size AND viewport dimensions |
| 1920x1080 too large | Use 1280x800 DesktopViewport default |
| Browser detection | Launcher flags disable automation indicators |
| Annotations not showing | Set ShowAnnotations: true, Headless: false |
