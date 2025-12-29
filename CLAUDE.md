# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build all packages
go build ./...

# Run all tests
go test ./...

# Run tests with verbose output
go test -v ./...

# Run a single test
go test -v -run TestName ./path/to/package

# Format and vet
go fmt ./...
go vet ./...

# Update dependencies
go mod tidy
```

## Architecture Overview

bua-go is a browser automation library powered by Google ADK (Agent Development Kit) and Gemini LLMs. It uses a **vision + DOM hybrid approach**.

### Core Flow
```
User calls agent.Run(prompt)
    → bua.go creates ADK runner
    → runner.Run() streams events
    → LLM sees: annotated screenshot + element map
    → LLM calls browser tools (click, type, scroll, etc.)
    → Tools execute via go-rod/rod
    → Loop continues until done() tool called
```

### ADK Integration Pattern

The project uses ADK for agent orchestration. Key pattern in `agent/agent.go`:

```go
// Tool handlers must return (Output, error), not just Output
handler := func(ctx tool.Context, input InputType) (OutputType, error) {
    return OutputType{...}, nil
}
tool, err := functiontool.New(functiontool.Config{
    Name: "tool_name",
    Description: "...",
}, handler)
```

**Import alias required** in `bua.go` to avoid conflict between local `agent` package and ADK's:
```go
adkagent "google.golang.org/adk/agent"
```

### Package Responsibilities

| Package | Role |
|---------|------|
| `bua.go` | Public API: `New()`, `Start()`, `Run()`, `Navigate()` |
| `agent/` | ADK BrowserAgent, 10 function tools, system prompts, logging |
| `browser/` | Rod wrapper, CDP operations, element interaction, downloads |
| `dom/` | ElementMap extraction, accessibility tree, bounding boxes |
| `memory/` | Short-term observations, long-term pattern storage |
| `screenshot/` | Capture, element annotation with fogleman/gg |
| `export/` | Dual-use architecture: BrowserTool, MultiBrowserTool for ADK |

### Browser Tools (defined in agent/agent.go)

10 tools available to the LLM:
- `click`, `type_text`, `navigate`, `wait`
- `scroll` - Supports both page-level scrolling and element-specific scrolling (for modals, sidebars, popups like Instagram comments)
- `extract`, `get_page_state`, `download_file`
- `request_human_takeover`, `done`

Each tool uses Input/Output struct pairs with jsonschema tags for parameter descriptions.

**Scroll tool details** (`agent/agent.go`, `browser/browser.go`):
- `direction`: up, down (required)
- `amount`: pixels to scroll (default 500)
- `element_id`: optional index of scrollable container for modal/popup scrolling
- Uses `Browser.Scroll()` for page scrolling, `Browser.ScrollInElement()` for container scrolling

### Examples (`examples/`)

| Example | Description |
|---------|-------------|
| `simple/` | Basic search example |
| `scraping/` | Hacker News data extraction |
| `download/` | File download capabilities |
| `adk_tool/` | Dual-use architecture demo |
| `content_research/` | Multi-tab Instagram research |
| `instagram_comments/` | Comment scraping with modal scrolling (element_id demo) |

### Dual-Use Architecture (export/adktool.go)

bua-go can be embedded as a tool in other ADK applications:
- `BrowserTool` - Single browser instance wrapper
- `MultiBrowserTool` - Parallel browser management (create/execute/close/list)
- `SimpleBrowserTask()` - One-off convenience function

## Environment

- `GOOGLE_API_KEY` - Required for Gemini API access

## Key Dependencies

- `google.golang.org/adk` - Agent Development Kit (agent loop, tool calling)
- `google.golang.org/genai` - Gemini API client
- `github.com/go-rod/rod` - Browser automation via CDP
- `github.com/fogleman/gg` - Screenshot annotation

## Serena MCP Integration

This project uses **Serena MCP** for semantic code understanding and project memory. Always use Serena tools when available:

### Project Memory
```
list_memories()              # See available project documentation
read_memory("memory_name")   # Load specific context (e.g., "project-overview")
write_memory("name", content) # Store insights for future sessions
```

**Current memories:**
- `project-overview` - High-level architecture, features, and package structure
- `adk-integration-details` - ADK patterns, tool signatures, dual-use architecture
- `prompt-best-practices` - Prompt engineering patterns for browser automation

### Semantic Code Operations
```
find_symbol("name_path")           # Find symbols by name (e.g., "BrowserAgent/Init")
find_referencing_symbols(...)      # Find all references to a symbol
get_symbols_overview("path")       # Get overview of symbols in a file
replace_symbol_body(...)           # Replace entire symbol definition
insert_before_symbol(...)          # Insert code before a symbol
insert_after_symbol(...)           # Insert code after a symbol
rename_symbol(...)                 # Rename with automatic reference updates
```

### Session Workflow
1. **Start**: `list_memories()` → `read_memory("project-overview")`
2. **During work**: Use `find_symbol`, `get_symbols_overview` for navigation
3. **After changes**: `write_memory()` to capture significant decisions
4. **Before ending**: Update memories with any architectural changes

### When to Use Serena vs Native Tools
| Task | Use Serena | Use Native |
|------|------------|------------|
| Symbol rename across project | ✅ `rename_symbol` | |
| Find all usages of a function | ✅ `find_referencing_symbols` | |
| Understand package structure | ✅ `get_symbols_overview` | |
| Simple text search | | ✅ Grep |
| Read entire file | | ✅ Read |
| Pattern-based edits | | ✅ Edit/MultiEdit |
