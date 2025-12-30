// Package export provides tools for using bua-go within other ADK applications.
package export

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"

	"github.com/anxuanzi/bua-go"
)

// BrowserToolConfig holds configuration for the browser automation tool.
type BrowserToolConfig struct {
	// APIKey is the Gemini API key. If empty, uses GOOGLE_API_KEY env var.
	APIKey string

	// Model is the Gemini model to use (default: gemini-3-flash-preview).
	Model string

	// Headless runs the browser without a visible window.
	Headless bool

	// Viewport defines the browser window size.
	Viewport *bua.Viewport

	// ShowAnnotations enables visual element annotations.
	ShowAnnotations bool

	// Debug enables verbose logging.
	Debug bool
}

// DefaultBrowserToolConfig returns the default configuration.
func DefaultBrowserToolConfig() *BrowserToolConfig {
	return &BrowserToolConfig{
		Model:           "gemini-3-flash-preview",
		Headless:        true,
		Viewport:        bua.DesktopViewport,
		ShowAnnotations: false,
		Debug:           false,
	}
}

// BrowserToolInput is the input for the browser automation tool.
type BrowserToolInput struct {
	Task        string `json:"task" jsonschema:"The browser automation task to perform (e.g., 'Go to example.com and extract the main heading')"`
	StartURL    string `json:"start_url,omitempty" jsonschema:"Optional: URL to navigate to before starting the task"`
	MaxSteps    int    `json:"max_steps,omitempty" jsonschema:"Optional: Maximum number of steps to take (default: 30)"`
	KeepBrowser bool   `json:"keep_browser,omitempty" jsonschema:"Optional: Keep browser open after task completion for follow-up tasks"`
}

// BrowserToolOutput is the output from the browser automation tool.
type BrowserToolOutput struct {
	Success   bool             `json:"success"`
	Message   string           `json:"message"`
	Data      map[string]any   `json:"data,omitempty"`
	Findings  []map[string]any `json:"findings,omitempty"`
	FinalURL  string           `json:"final_url,omitempty"`
	FinalHTML string           `json:"final_html,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// BrowserTool wraps a bua-go agent for use as an ADK tool.
type BrowserTool struct {
	config *BrowserToolConfig
	agent  *bua.Agent
	mu     sync.Mutex
}

// NewBrowserTool creates a new browser automation tool.
func NewBrowserTool(cfg *BrowserToolConfig) *BrowserTool {
	if cfg == nil {
		cfg = DefaultBrowserToolConfig()
	}
	return &BrowserTool{
		config: cfg,
	}
}

// Tool returns the ADK tool that can be added to other agents.
func (bt *BrowserTool) Tool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input BrowserToolInput) (BrowserToolOutput, error) {
		return bt.execute(ctx, input)
	}

	return functiontool.New(
		functiontool.Config{
			Name:        "browser_automation",
			Description: "Automate browser tasks like navigating websites, clicking elements, filling forms, and extracting data. Useful for web scraping, data collection, form submission, and any task requiring browser interaction.",
		},
		handler,
	)
}

// execute runs the browser automation task.
func (bt *BrowserTool) execute(ctx tool.Context, input BrowserToolInput) (BrowserToolOutput, error) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Create or reuse agent
	if bt.agent == nil || !input.KeepBrowser {
		// Close existing agent if any
		if bt.agent != nil {
			bt.agent.Close()
		}

		// Create new agent
		cfg := bua.Config{
			APIKey:          bt.config.APIKey,
			Model:           bt.config.Model,
			Headless:        bt.config.Headless,
			Viewport:        bt.config.Viewport,
			ShowAnnotations: bt.config.ShowAnnotations,
			Debug:           bt.config.Debug,
		}

		agent, err := bua.New(cfg)
		if err != nil {
			return BrowserToolOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to create browser agent: %v", err),
			}, nil
		}
		bt.agent = agent

		// Start browser
		bgCtx := context.Background()
		if err := bt.agent.Start(bgCtx); err != nil {
			bt.agent.Close()
			bt.agent = nil
			return BrowserToolOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to start browser: %v", err),
			}, nil
		}
	}

	// Navigate to start URL if provided
	bgCtx := context.Background()
	if input.StartURL != "" {
		if err := bt.agent.Navigate(bgCtx, input.StartURL); err != nil {
			return BrowserToolOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to navigate to start URL: %v", err),
			}, nil
		}
	}

	// Run the task
	result, err := bt.agent.Run(bgCtx, input.Task)
	if err != nil {
		return BrowserToolOutput{
			Success: false,
			Error:   fmt.Sprintf("task execution failed: %v", err),
		}, nil
	}

	// Build output
	output := BrowserToolOutput{
		Success: result.Success,
		Message: "Task completed",
	}

	// Convert Data to map[string]any if possible
	if result.Data != nil {
		if dataMap, ok := result.Data.(map[string]any); ok {
			output.Data = dataMap
		} else {
			output.Data = map[string]any{"raw": result.Data}
		}
	}

	if result.Error != "" {
		output.Error = result.Error
	}

	// Get current page state
	if bt.agent != nil {
		browserAgent := bt.agent.GetAgent()
		if browserAgent != nil {
			b := browserAgent.GetBrowser()
			if b != nil {
				output.FinalURL = b.GetURL()
			}
		}
	}

	// Close browser if not keeping it
	if !input.KeepBrowser {
		bt.agent.Close()
		bt.agent = nil
	}

	return output, nil
}

// Close closes the browser tool and releases resources.
func (bt *BrowserTool) Close() error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if bt.agent != nil {
		bt.agent.Close()
		bt.agent = nil
	}
	return nil
}

// MultiBrowserToolConfig holds configuration for the multi-task browser tool.
type MultiBrowserToolConfig struct {
	*BrowserToolConfig
	// MaxConcurrentBrowsers limits concurrent browser instances.
	MaxConcurrentBrowsers int
}

// MultiBrowserTool manages multiple browser instances for parallel tasks.
type MultiBrowserTool struct {
	config    *MultiBrowserToolConfig
	instances map[string]*bua.Agent
	mu        sync.Mutex
}

// NewMultiBrowserTool creates a new multi-browser tool.
func NewMultiBrowserTool(cfg *MultiBrowserToolConfig) *MultiBrowserTool {
	if cfg == nil {
		cfg = &MultiBrowserToolConfig{
			BrowserToolConfig:     DefaultBrowserToolConfig(),
			MaxConcurrentBrowsers: 3,
		}
	}
	return &MultiBrowserTool{
		config:    cfg,
		instances: make(map[string]*bua.Agent),
	}
}

// MultiBrowserInput is the input for multi-browser tool operations.
type MultiBrowserInput struct {
	Action      string `json:"action" jsonschema:"Action to perform: 'create', 'execute', 'close', or 'list'"`
	BrowserID   string `json:"browser_id,omitempty" jsonschema:"Browser instance ID (returned from 'create' action)"`
	Task        string `json:"task,omitempty" jsonschema:"Task to execute (for 'execute' action)"`
	StartURL    string `json:"start_url,omitempty" jsonschema:"URL to navigate to (for 'create' action)"`
	ProfileName string `json:"profile_name,omitempty" jsonschema:"Profile name for the browser instance"`
}

// MultiBrowserOutput is the output from multi-browser operations.
type MultiBrowserOutput struct {
	Success   bool             `json:"success"`
	Message   string           `json:"message"`
	BrowserID string           `json:"browser_id,omitempty"`
	Data      map[string]any   `json:"data,omitempty"`
	Findings  []map[string]any `json:"findings,omitempty"`
	Browsers  []string         `json:"browsers,omitempty"`
	Error     string           `json:"error,omitempty"`
}

// Tool returns the ADK tool for multi-browser operations.
func (mbt *MultiBrowserTool) Tool() (tool.Tool, error) {
	handler := func(ctx tool.Context, input MultiBrowserInput) (MultiBrowserOutput, error) {
		return mbt.execute(ctx, input)
	}

	return functiontool.New(
		functiontool.Config{
			Name:        "multi_browser",
			Description: "Manage multiple browser instances for parallel web automation tasks. Use 'create' to start a new browser, 'execute' to run a task, 'close' to close a browser, or 'list' to see all browsers.",
		},
		handler,
	)
}

func (mbt *MultiBrowserTool) execute(ctx tool.Context, input MultiBrowserInput) (MultiBrowserOutput, error) {
	mbt.mu.Lock()
	defer mbt.mu.Unlock()

	switch input.Action {
	case "create":
		return mbt.createBrowser(input)
	case "execute":
		return mbt.executeTasks(input)
	case "close":
		return mbt.closeBrowser(input)
	case "list":
		return mbt.listBrowsers()
	default:
		return MultiBrowserOutput{
			Success: false,
			Error:   fmt.Sprintf("unknown action: %s", input.Action),
		}, nil
	}
}

func (mbt *MultiBrowserTool) createBrowser(input MultiBrowserInput) (MultiBrowserOutput, error) {
	if len(mbt.instances) >= mbt.config.MaxConcurrentBrowsers {
		return MultiBrowserOutput{
			Success: false,
			Error:   fmt.Sprintf("maximum concurrent browsers reached (%d)", mbt.config.MaxConcurrentBrowsers),
		}, nil
	}

	profileName := input.ProfileName
	if profileName == "" {
		profileName = fmt.Sprintf("browser_%d", len(mbt.instances)+1)
	}

	cfg := bua.Config{
		APIKey:          mbt.config.APIKey,
		Model:           mbt.config.Model,
		ProfileName:     profileName,
		Headless:        mbt.config.Headless,
		Viewport:        mbt.config.Viewport,
		ShowAnnotations: mbt.config.ShowAnnotations,
		Debug:           mbt.config.Debug,
	}

	agent, err := bua.New(cfg)
	if err != nil {
		return MultiBrowserOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to create browser: %v", err),
		}, nil
	}

	bgCtx := context.Background()
	if err := agent.Start(bgCtx); err != nil {
		agent.Close()
		return MultiBrowserOutput{
			Success: false,
			Error:   fmt.Sprintf("failed to start browser: %v", err),
		}, nil
	}

	if input.StartURL != "" {
		if err := agent.Navigate(bgCtx, input.StartURL); err != nil {
			agent.Close()
			return MultiBrowserOutput{
				Success: false,
				Error:   fmt.Sprintf("failed to navigate: %v", err),
			}, nil
		}
	}

	browserID := profileName
	mbt.instances[browserID] = agent

	return MultiBrowserOutput{
		Success:   true,
		Message:   "Browser created",
		BrowserID: browserID,
	}, nil
}

func (mbt *MultiBrowserTool) executeTasks(input MultiBrowserInput) (MultiBrowserOutput, error) {
	agent, ok := mbt.instances[input.BrowserID]
	if !ok {
		return MultiBrowserOutput{
			Success: false,
			Error:   fmt.Sprintf("browser not found: %s", input.BrowserID),
		}, nil
	}

	bgCtx := context.Background()
	result, err := agent.Run(bgCtx, input.Task)
	if err != nil {
		return MultiBrowserOutput{
			Success: false,
			Error:   fmt.Sprintf("task failed: %v", err),
		}, nil
	}

	output := MultiBrowserOutput{
		Success:   result.Success,
		Message:   "Task completed",
		BrowserID: input.BrowserID,
	}

	// Convert Data to map[string]any if possible
	if result.Data != nil {
		if dataMap, ok := result.Data.(map[string]any); ok {
			output.Data = dataMap
		} else {
			output.Data = map[string]any{"raw": result.Data}
		}
	}

	return output, nil
}

func (mbt *MultiBrowserTool) closeBrowser(input MultiBrowserInput) (MultiBrowserOutput, error) {
	agent, ok := mbt.instances[input.BrowserID]
	if !ok {
		return MultiBrowserOutput{
			Success: false,
			Error:   fmt.Sprintf("browser not found: %s", input.BrowserID),
		}, nil
	}

	agent.Close()
	delete(mbt.instances, input.BrowserID)

	return MultiBrowserOutput{
		Success:   true,
		Message:   "Browser closed",
		BrowserID: input.BrowserID,
	}, nil
}

func (mbt *MultiBrowserTool) listBrowsers() (MultiBrowserOutput, error) {
	browsers := make([]string, 0, len(mbt.instances))
	for id := range mbt.instances {
		browsers = append(browsers, id)
	}

	return MultiBrowserOutput{
		Success:  true,
		Message:  fmt.Sprintf("Found %d browsers", len(browsers)),
		Browsers: browsers,
	}, nil
}

// Close closes all browser instances.
func (mbt *MultiBrowserTool) Close() error {
	mbt.mu.Lock()
	defer mbt.mu.Unlock()

	for id, agent := range mbt.instances {
		agent.Close()
		delete(mbt.instances, id)
	}
	return nil
}

// SimpleBrowserTask provides a simple function for one-off browser tasks.
// This is useful for quick automation without needing to manage tool instances.
func SimpleBrowserTask(task string, startURL string, cfg *BrowserToolConfig) (*BrowserToolOutput, error) {
	bt := NewBrowserTool(cfg)
	defer bt.Close()

	tool, err := bt.Tool()
	if err != nil {
		return nil, fmt.Errorf("failed to create tool: %w", err)
	}

	// Create input
	input := BrowserToolInput{
		Task:     task,
		StartURL: startURL,
	}

	// Serialize input to JSON for the tool call
	inputBytes, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input: %w", err)
	}

	// Call the tool directly using its Run method
	_ = tool
	_ = inputBytes

	// Actually execute using our execute method directly
	output, err := bt.execute(nil, input)
	if err != nil {
		return nil, err
	}

	return &output, nil
}
