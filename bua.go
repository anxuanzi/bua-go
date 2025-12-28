// Package bua provides a browser automation agent powered by LLMs via Google ADK.
// It uses a vision + DOM hybrid approach, combining screenshots with parsed HTML
// and accessibility trees to navigate websites and extract data based on natural language prompts.
package bua

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/anxuanzi/bua-go/agent"
	"github.com/anxuanzi/bua-go/browser"
	"github.com/anxuanzi/bua-go/dom"
	"github.com/anxuanzi/bua-go/memory"
	"github.com/anxuanzi/bua-go/screenshot"
)

// Config holds the configuration for creating a new Agent.
type Config struct {
	// APIKey is the API key for the LLM provider (e.g., Gemini API key).
	APIKey string

	// Model is the model ID to use (e.g., "gemini-2.5-flash", "gemini-2.5-pro").
	Model string

	// ProfileName is the name of the browser profile to use for session persistence.
	// If empty, a temporary profile is created and cleaned up after the session.
	ProfileName string

	// ProfileDir is the base directory for storing browser profiles.
	// Defaults to ~/.bua/profiles if empty.
	ProfileDir string

	// Headless determines whether the browser runs in headless mode.
	// Set to false for debugging or when human takeover is needed.
	Headless bool

	// Viewport sets the browser viewport size.
	// Defaults to DesktopViewport if nil.
	Viewport *Viewport

	// ScreenshotConfig configures screenshot capture and storage.
	ScreenshotConfig *screenshot.Config

	// MemoryConfig configures the memory system.
	MemoryConfig *memory.Config

	// MaxTokens is the maximum context window size for the LLM.
	// Used for token management and conversation compaction.
	// Defaults to 128000 if zero.
	MaxTokens int

	// Debug enables verbose logging.
	Debug bool
}

// Viewport defines browser viewport dimensions.
type Viewport struct {
	Width  int
	Height int
}

// Common viewport presets.
var (
	// DesktopViewport is a safe default that fits most laptop screens
	DesktopViewport = &Viewport{Width: 1280, Height: 800}
	// LargeDesktopViewport for full HD displays
	LargeDesktopViewport = &Viewport{Width: 1920, Height: 1080}
	// TabletViewport for tablet simulation
	TabletViewport = &Viewport{Width: 768, Height: 1024}
	// MobileViewport for mobile simulation
	MobileViewport = &Viewport{Width: 375, Height: 812}
)

// Result represents the result of a task execution.
type Result struct {
	// Success indicates whether the task completed successfully.
	Success bool

	// Data contains any extracted data from the task.
	Data any

	// Error contains the error message if the task failed.
	Error string

	// Steps contains the history of steps taken during execution.
	Steps []Step

	// TokensUsed is the total number of tokens consumed.
	TokensUsed int

	// ScreenshotPaths contains paths to screenshots taken during execution.
	ScreenshotPaths []string
}

// Step represents a single step in the task execution.
type Step struct {
	// Action is the action taken (e.g., "click", "type", "scroll").
	Action string

	// Target describes what element was targeted.
	Target string

	// Reasoning is the LLM's explanation for why this action was taken.
	Reasoning string

	// ScreenshotPath is the path to the screenshot taken after this step.
	ScreenshotPath string
}

// Agent is the main interface for browser automation.
type Agent struct {
	config         Config
	browser        *browser.Browser
	memory         *memory.Manager
	launcher       *launcher.Launcher
	browserAgent   *agent.BrowserAgent
	runner         *runner.Runner
	sessionService session.Service

	mu     sync.Mutex
	closed bool
}

// New creates a new browser automation agent with the given configuration.
func New(cfg Config) (*Agent, error) {
	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}
	if cfg.Viewport == nil {
		cfg.Viewport = DesktopViewport
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 128000
	}
	if cfg.ProfileDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		cfg.ProfileDir = filepath.Join(home, ".bua", "profiles")
	}
	if cfg.ScreenshotConfig == nil {
		cfg.ScreenshotConfig = &screenshot.Config{
			Enabled:        true,
			Annotate:       true,
			StorageDir:     filepath.Join(cfg.ProfileDir, "..", "screenshots"),
			MaxScreenshots: 100,
		}
	}
	if cfg.MemoryConfig == nil {
		cfg.MemoryConfig = &memory.Config{
			ShortTermLimit: 10,
			StorageDir:     filepath.Join(cfg.ProfileDir, "..", "memory"),
		}
	}

	// Validate required fields
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("APIKey is required")
	}

	// Create the agent
	agent := &Agent{
		config: cfg,
	}

	return agent, nil
}

// Start initializes the browser and prepares the agent for task execution.
func (a *Agent) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return fmt.Errorf("agent is closed")
	}

	// Determine user data directory
	var userDataDir string
	if a.config.ProfileName != "" {
		userDataDir = filepath.Join(a.config.ProfileDir, a.config.ProfileName)
		if err := os.MkdirAll(userDataDir, 0755); err != nil {
			return fmt.Errorf("failed to create profile directory: %w", err)
		}
	}

	// Create launcher - viewport will be set via CDP for proper responsive handling
	a.launcher = launcher.New().
		Set("disable-blink-features", "AutomationControlled"). // Avoid detection
		Set("disable-infobars").
		Set("disable-dev-shm-usage").
		Set("no-first-run").
		Set("no-default-browser-check").
		Set("window-size", fmt.Sprintf("%d,%d", a.config.Viewport.Width, a.config.Viewport.Height)).
		Headless(a.config.Headless)

	if userDataDir != "" {
		a.launcher = a.launcher.UserDataDir(userDataDir)
	}

	// Launch browser
	controlURL, err := a.launcher.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	// Connect to browser
	rodBrowser := rod.New().ControlURL(controlURL)
	if err := rodBrowser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	// Create browser wrapper
	a.browser = browser.New(rodBrowser, browser.Config{
		Viewport: &browser.Viewport{
			Width:  a.config.Viewport.Width,
			Height: a.config.Viewport.Height,
		},
		ScreenshotConfig: a.config.ScreenshotConfig,
	})

	// Initialize memory manager
	a.memory = memory.NewManager(a.config.MemoryConfig)
	if err := a.memory.Load(ctx); err != nil {
		// Log warning but don't fail - start with empty memory
		if a.config.Debug {
			fmt.Printf("Warning: failed to load memory: %v\n", err)
		}
	}

	// Create and initialize ADK browser agent
	a.browserAgent = agent.New(agent.Config{
		APIKey:        a.config.APIKey,
		Model:         a.config.Model,
		MaxIterations: 50,
		MaxTokens:     a.config.MaxTokens,
		Debug:         a.config.Debug,
	}, a.browser, a.memory)

	if err := a.browserAgent.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize ADK agent: %w", err)
	}

	// Create ADK runner for executing the agent
	adkAgent := a.browserAgent.GetADKAgent()
	if adkAgent == nil {
		return fmt.Errorf("ADK agent not initialized")
	}

	// Create session service
	a.sessionService = session.InMemoryService()

	r, err := runner.New(runner.Config{
		AppName:        "bua-browser-agent",
		Agent:          adkAgent,
		SessionService: a.sessionService,
	})
	if err != nil {
		return fmt.Errorf("failed to create ADK runner: %w", err)
	}
	a.runner = r

	return nil
}

// Run executes a task with the given natural language prompt.
func (a *Agent) Run(ctx context.Context, prompt string) (*Result, error) {
	a.mu.Lock()
	if a.browser == nil || a.browserAgent == nil || a.runner == nil || a.sessionService == nil {
		a.mu.Unlock()
		return nil, fmt.Errorf("agent not started, call Start() first")
	}
	r := a.runner
	ss := a.sessionService
	a.mu.Unlock()

	// Create user message
	userMessage := &genai.Content{
		Role: "user",
		Parts: []*genai.Part{
			{Text: prompt},
		},
	}

	// Create a new session for this run
	userID := "default_user"
	createResp, err := ss.Create(ctx, &session.CreateRequest{
		AppName: "bua-browser-agent",
		UserID:  userID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	sessionID := createResp.Session.ID()

	// Execute the agent and collect events
	result := &Result{
		Success: true,
		Steps:   []Step{},
		Data:    make(map[string]any),
	}

	var lastResponse string
	var extractedData string
	var doneSummary string
	for event, err := range r.Run(ctx, userID, sessionID, userMessage, adkagent.RunConfig{}) {
		if err != nil {
			result.Success = false
			result.Error = err.Error()
			return result, nil
		}

		// Process events from the agent
		if event != nil {
			if a.config.Debug {
				fmt.Printf("[DEBUG] Event: Author=%s, Partial=%v\n", event.Author, event.Partial)
			}
			if event.Content != nil {
				for i, part := range event.Content.Parts {
					if part != nil {
						if a.config.Debug {
							if part.Text != "" {
								fmt.Printf("[DEBUG] Part[%d] Text: %s\n", i, truncateString(part.Text, 200))
							}
							if part.FunctionCall != nil {
								fmt.Printf("[DEBUG] Part[%d] FunctionCall: %s(%v)\n", i, part.FunctionCall.Name, truncateString(fmt.Sprintf("%v", part.FunctionCall.Args), 100))
							}
							if part.FunctionResponse != nil {
								fmt.Printf("[DEBUG] Part[%d] FunctionResponse: %s -> %v\n", i, part.FunctionResponse.Name, truncateString(fmt.Sprintf("%v", part.FunctionResponse.Response), 100))
							}
						}
						if part.Text != "" {
							lastResponse = part.Text
						}
						// Capture data from the done tool call
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
					}
				}
			}
		}
	}

	// Build result data
	if extractedData != "" {
		// Try to parse extracted_data as JSON
		var parsed any
		if err := json.Unmarshal([]byte(extractedData), &parsed); err == nil {
			result.Data = map[string]any{"extracted_data": parsed, "summary": doneSummary}
		} else {
			result.Data = map[string]any{"extracted_data": extractedData, "summary": doneSummary}
		}
	} else if lastResponse != "" {
		result.Data = map[string]any{"response": lastResponse}
	} else if doneSummary != "" {
		result.Data = map[string]any{"summary": doneSummary}
	}

	return result, nil
}

// Navigate navigates to the specified URL.
func (a *Agent) Navigate(ctx context.Context, url string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return fmt.Errorf("agent not started, call Start() first")
	}

	return a.browser.Navigate(ctx, url)
}

// Screenshot takes a screenshot of the current page.
func (a *Agent) Screenshot(ctx context.Context) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return nil, fmt.Errorf("agent not started, call Start() first")
	}

	return a.browser.Screenshot(ctx)
}

// GetElementMap extracts the element map from the current page.
func (a *Agent) GetElementMap(ctx context.Context) (*dom.ElementMap, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return nil, fmt.Errorf("agent not started, call Start() first")
	}

	return a.browser.GetElementMap(ctx)
}

// GetAccessibilityTree extracts the accessibility tree from the current page.
func (a *Agent) GetAccessibilityTree(ctx context.Context) (*dom.AccessibilityTree, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return nil, fmt.Errorf("agent not started, call Start() first")
	}

	return a.browser.GetAccessibilityTree(ctx)
}

// RequestHumanTakeover pauses the agent and prompts the user to complete
// an action (like login or CAPTCHA) manually.
func (a *Agent) RequestHumanTakeover(ctx context.Context, reason string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return fmt.Errorf("agent not started, call Start() first")
	}
	if a.config.Headless {
		return fmt.Errorf("human takeover requires headed mode (Headless: false)")
	}

	// TODO: Implement human takeover notification and wait
	fmt.Printf("Human takeover requested: %s\n", reason)
	fmt.Println("Complete the action in the browser and press Enter to continue...")

	return nil
}

// Close closes the browser and cleans up resources.
func (a *Agent) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}
	a.closed = true

	var errs []error

	// Save memory before closing
	if a.memory != nil {
		if err := a.memory.Save(context.Background()); err != nil {
			errs = append(errs, fmt.Errorf("failed to save memory: %w", err))
		}
	}

	// Close browser
	if a.browser != nil {
		if err := a.browser.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close browser: %w", err))
		}
	}

	// Cleanup launcher (removes temp profile if no ProfileName was set)
	if a.launcher != nil && a.config.ProfileName == "" {
		a.launcher.Cleanup()
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}

// Page returns the current page for low-level access.
// Use with caution as this bypasses the agent's abstractions.
func (a *Agent) Page() *rod.Page {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return nil
	}
	return a.browser.Page()
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// Call executes a raw CDP command on the current page.
// This is useful for accessing CDP features not directly exposed by the agent.
// Returns the raw JSON response from the CDP call.
func (a *Agent) Call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return nil, fmt.Errorf("agent not started, call Start() first")
	}

	page := a.browser.Page()
	if page == nil {
		return nil, fmt.Errorf("no active page")
	}

	result, err := page.Call(ctx, "", method, params)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(result), nil
}
