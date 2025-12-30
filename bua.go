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
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	adkagent "google.golang.org/adk/agent"
	"google.golang.org/adk/artifact"
	"google.golang.org/adk/memory"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"

	"github.com/anxuanzi/bua-go/agent"
	"github.com/anxuanzi/bua-go/browser"
	"github.com/anxuanzi/bua-go/dom"
	"github.com/anxuanzi/bua-go/screenshot"
)

// Config holds the configuration for creating a new Agent.
type Config struct {
	// APIKey is the API key for the LLM provider (e.g., Gemini API key).
	APIKey string

	// Model is the model ID to use (e.g., "gemini-3-flash-preview").
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

	// MaxTokens is the maximum context window size for the LLM.
	// Used for token management and conversation compaction.
	// Defaults to 1048576 if zero.
	MaxTokens int

	// Debug enables verbose logging.
	Debug bool

	// ShowAnnotations enables visual element annotations in the browser.
	// When enabled, annotations are shown before each action for debugging.
	// Also captures annotated screenshots for each step.
	ShowAnnotations bool

	// ScreenshotMode controls when screenshots are sent to the model.
	// "normal" (default): Only in get_page_state responses
	// "smart": After each action AND in get_page_state responses (higher token usage but better awareness)
	ScreenshotMode string

	// MaxElements limits elements sent to LLM (default 150, 0 = no limit).
	// Critical for staying within context limits - 500 elements can use 50K+ tokens.
	// Increase this if using a model with larger context window.
	MaxElements int

	// ScreenshotMaxWidth is the max width for LLM screenshots (default 800).
	// Smaller = fewer tokens. 800px is readable while being ~10x smaller than full size.
	// Increase this for better visual quality on larger context models.
	ScreenshotMaxWidth int

	// ScreenshotQuality is JPEG quality for LLM screenshots (default 60, range 1-100).
	// Lower = smaller file but more artifacts. 60 is good balance.
	// Increase this for better image quality on larger context models.
	ScreenshotQuality int

	// TextOnly disables all screenshot capture for faster, lower-token operation.
	// When enabled, the agent relies only on element map text data.
	// Best for: text extraction, form filling, simple navigation where visual context isn't needed.
	// Provides significant speed improvement (no screenshot capture/encoding overhead).
	TextOnly bool
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

// TokenPreset defines token management settings for different use cases.
type TokenPreset struct {
	// MaxElements limits interactive elements sent to LLM.
	MaxElements int
	// ScreenshotMaxWidth is the max width for LLM screenshots.
	ScreenshotMaxWidth int
	// ScreenshotQuality is JPEG quality (1-100).
	ScreenshotQuality int
	// TextOnly disables screenshots entirely for fastest operation.
	TextOnly bool
}

// Token management presets for different use cases.
// All Gemini 2.x/3.x models have 1M token context, so presets are based on
// use case rather than model size.
var (
	// TokenPresetEfficient minimizes token usage for cost savings.
	// Best for: Simple tasks, high-volume automation, cost-sensitive use cases.
	// ~15-25K tokens per page state.
	TokenPresetEfficient = &TokenPreset{
		MaxElements:        100,
		ScreenshotMaxWidth: 640,
		ScreenshotQuality:  50,
	}

	// TokenPresetBalanced is the default balance between quality and token usage.
	// Best for: Most automation tasks, general web browsing.
	// ~25-40K tokens per page state.
	TokenPresetBalanced = &TokenPreset{
		MaxElements:        150,
		ScreenshotMaxWidth: 800,
		ScreenshotQuality:  60,
	}

	// TokenPresetQuality provides higher quality for complex visual tasks.
	// Best for: Complex UIs, data-heavy pages, visual verification tasks.
	// ~40-60K tokens per page state.
	TokenPresetQuality = &TokenPreset{
		MaxElements:        250,
		ScreenshotMaxWidth: 1024,
		ScreenshotQuality:  75,
	}

	// TokenPresetMaximum provides full quality when token budget is not a concern.
	// Best for: Debugging, complex multi-step tasks, maximum accuracy needed.
	// ~60-100K tokens per page state.
	TokenPresetMaximum = &TokenPreset{
		MaxElements:        400,
		ScreenshotMaxWidth: 1280,
		ScreenshotQuality:  85,
	}

	// TokenPresetTextOnly disables screenshots for fastest, lowest-token operation.
	// Best for: Text extraction, form filling, simple navigation, high-speed scraping.
	// ~5-15K tokens per page state (element map only, no screenshots).
	// Significant speed improvement - no screenshot capture/encoding overhead.
	TokenPresetTextOnly = &TokenPreset{
		MaxElements:        200,
		ScreenshotMaxWidth: 0, // Not used
		ScreenshotQuality:  0, // Not used
		TextOnly:           true,
	}
)

// ApplyTokenPreset applies a token preset to the config.
func (c *Config) ApplyTokenPreset(preset *TokenPreset) {
	if preset == nil {
		return
	}
	c.MaxElements = preset.MaxElements
	c.ScreenshotMaxWidth = preset.ScreenshotMaxWidth
	c.ScreenshotQuality = preset.ScreenshotQuality
	c.TextOnly = preset.TextOnly
}

// Gemini model constants for convenience.
const (
	// ModelGemini3Pro is the latest Gemini 3 Pro model (1M context).
	ModelGemini3Pro = "gemini-3-pro-preview"
	// ModelGemini3Flash is the latest Gemini 3 Flash model (1M context, faster).
	ModelGemini3Flash = "gemini-3-flash-preview"
	// ModelGemini25Pro is Gemini 2.5 Pro (1M context, stable).
	ModelGemini25Pro = "gemini-2.5-pro"
	// ModelGemini25Flash is Gemini 2.5 Flash (1M context, fast & efficient).
	ModelGemini25Flash = "gemini-2.5-flash"
	// ModelGemini25FlashLite is Gemini 2.5 Flash Lite (1M context, most efficient).
	ModelGemini25FlashLite = "gemini-2.5-flash-lite"
	// ModelGemini20Flash is Gemini 2.0 Flash (1M context).
	ModelGemini20Flash = "gemini-2.0-flash"
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

	// TokensUsed is the total number of tokens consumed (estimated).
	TokensUsed int

	// Duration is the total time taken to complete the task.
	Duration time.Duration

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
	config          Config
	browser         *browser.Browser
	launcher        *launcher.Launcher
	browserAgent    *agent.BrowserAgent
	runner          *runner.Runner
	sessionService  session.Service
	memoryService   memory.Service
	artifactService artifact.Service

	mu     sync.Mutex
	closed bool
}

// New creates a new browser automation agent with the given configuration.
func New(cfg Config) (*Agent, error) {
	// Apply defaults
	if cfg.Model == "" {
		cfg.Model = "gemini-3-flash-preview"
	}
	if cfg.Viewport == nil {
		cfg.Viewport = DesktopViewport
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 1048576 // gemini-3-flash-preview input limit
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

	// Validate required fields
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("APIKey is required")
	}

	// Create the agent
	a := &Agent{
		config: cfg,
	}

	return a, nil
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
		// Anti-detection flags
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-infobars").
		Set("disable-dev-shm-usage").
		Set("no-first-run").
		Set("no-default-browser-check").
		// Media playback flags (for Instagram Reels, YouTube, etc.)
		Set("autoplay-policy", "no-user-gesture-required").
		Set("disable-features", "PreloadMediaEngagementData,MediaEngagementBypassAutoplayPolicies").
		Set("enable-features", "NetworkService,NetworkServiceInProcess").
		// Additional anti-detection
		Set("disable-background-networking").
		Set("disable-client-side-phishing-detection").
		Set("disable-default-apps").
		Set("disable-extensions").
		Set("disable-hang-monitor").
		Set("disable-popup-blocking").
		Set("disable-prompt-on-repost").
		Set("disable-sync").
		Set("disable-translate").
		Set("metrics-recording-only").
		Set("safebrowsing-disable-auto-update").
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

	// Determine screenshot directory for annotations
	screenshotDir := ""
	if a.config.ShowAnnotations {
		screenshotDir = filepath.Join(a.config.ProfileDir, "..", "screenshots", "steps")
	}

	// Create and initialize ADK browser agent
	screenshotMode := a.config.ScreenshotMode
	if screenshotMode == "" {
		screenshotMode = "normal" // Default to normal mode
	}

	a.browserAgent = agent.New(agent.Config{
		APIKey:             a.config.APIKey,
		Model:              a.config.Model,
		MaxIterations:      50,
		MaxTokens:          a.config.MaxTokens,
		Debug:              a.config.Debug,
		ShowAnnotations:    a.config.ShowAnnotations,
		ScreenshotDir:      screenshotDir,
		ScreenshotMode:     screenshotMode,
		MaxElements:        a.config.MaxElements,
		ScreenshotMaxWidth: a.config.ScreenshotMaxWidth,
		ScreenshotQuality:  a.config.ScreenshotQuality,
		TextOnly:           a.config.TextOnly,
	}, a.browser)

	if err := a.browserAgent.Init(ctx); err != nil {
		return fmt.Errorf("failed to initialize ADK agent: %w", err)
	}

	// Create ADK runner for executing the agent
	adkAgent := a.browserAgent.GetADKAgent()
	if adkAgent == nil {
		return fmt.Errorf("ADK agent not initialized")
	}

	// Create ADK services
	a.sessionService = session.InMemoryService()
	a.memoryService = memory.InMemoryService()
	a.artifactService = artifact.InMemoryService()

	r, err := runner.New(runner.Config{
		AppName:         "bua-browser-agent",
		Agent:           adkAgent,
		SessionService:  a.sessionService,
		MemoryService:   a.memoryService,
		ArtifactService: a.artifactService,
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

	// Get logger for token/timing tracking
	logger := a.browserAgent.GetLogger()
	if logger != nil {
		logger.StartTask()
		// Track prompt tokens
		tokens := logger.GetTokens()
		if tokens != nil {
			tokens.AddText(prompt)
		}
	}

	// Execute the agent and collect events
	result := &Result{
		Success: true,
		Steps:   []Step{},
		Data:    make(map[string]any),
	}

	var lastResponse string
	var doneSummary string
	pendingSteps := make(map[string]Step) // Track pending function calls until we see their response
	var doneToolCalled bool
	var humanTakeoverRequested bool
	for event, err := range r.Run(ctx, userID, sessionID, userMessage, adkagent.RunConfig{}) {
		if err != nil {
			// If done tool was called successfully, ignore runner errors (e.g., "empty response")
			if doneToolCalled && result.Success {
				if a.config.Debug {
					fmt.Printf("[DEBUG] Ignoring runner error after done: %v\n", err)
				}
				continue
			}
			// If human takeover was requested, set appropriate error
			if humanTakeoverRequested {
				result.Success = false
				result.Error = "human takeover requested - agent could not complete task"
				break
			}
			// Handle "empty response" error when agent finished without calling done
			if err.Error() == "empty response" && len(result.Steps) > 0 {
				// Agent did some work but didn't call done - treat as partial success
				result.Success = false
				result.Error = "agent did not complete task (no done() call)"
				break
			}
			// Check for rate limiting (429) and retry with backoff
			if delay, isRateLimit := parseRateLimitDelay(err.Error()); isRateLimit {
				if a.config.Debug {
					fmt.Printf("[DEBUG] Rate limited, waiting %v before retry...\n", delay)
				}
				// Wait for the suggested delay plus a small buffer
				select {
				case <-ctx.Done():
					result.Success = false
					result.Error = "context cancelled while waiting for rate limit"
					return result, nil
				case <-time.After(delay + 2*time.Second):
				}
				// Recursive retry - will create a new session
				if a.config.Debug {
					fmt.Printf("[DEBUG] Retrying after rate limit...\n")
				}
				return a.Run(ctx, prompt)
			}
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
						// Track tokens from content
						if logger != nil && !event.Partial {
							tokens := logger.GetTokens()
							if tokens != nil {
								if part.Text != "" {
									tokens.AddText(part.Text)
								}
								if part.FunctionCall != nil {
									// Estimate tokens for function call (name + args)
									callStr := fmt.Sprintf("%s(%v)", part.FunctionCall.Name, part.FunctionCall.Args)
									tokens.AddText(callStr)
								}
								if part.FunctionResponse != nil {
									// Estimate tokens for function response
									respStr := fmt.Sprintf("%v", part.FunctionResponse.Response)
									tokens.AddText(respStr)
								}
							}
						}
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
						// Track pending function calls
						if part.FunctionCall != nil {
							step := Step{
								Action: part.FunctionCall.Name,
							}
							args := part.FunctionCall.Args
							// Extract reasoning if available (also check "reason" as alias)
							if reasoning, exists := args["reasoning"]; exists {
								if reasoningStr, ok := reasoning.(string); ok {
									step.Reasoning = reasoningStr
								}
							} else if reason, exists := args["reason"]; exists {
								if reasonStr, ok := reason.(string); ok {
									step.Reasoning = reasonStr
								}
							}
							// Extract target info based on action type
							if idx, exists := args["element_index"]; exists {
								step.Target = fmt.Sprintf("Element #%v", idx)
							}
							if url, exists := args["url"]; exists {
								if urlStr, ok := url.(string); ok {
									step.Target = urlStr
								}
							}
							if text, exists := args["text"]; exists {
								if textStr, ok := text.(string); ok {
									if step.Target != "" {
										step.Target += " → \"" + truncateString(textStr, 30) + "\""
									} else {
										step.Target = "\"" + truncateString(textStr, 30) + "\""
									}
								}
							}
							// Store pending step - will add when we see successful response
							pendingSteps[part.FunctionCall.Name] = step

							// Handle done tool specially
							if part.FunctionCall.Name == "done" {
								doneToolCalled = true
								if data, exists := args["data"]; exists {
									if dataMap, ok := data.(map[string]any); ok {
										result.Data = dataMap
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

							// Handle human takeover request
							if part.FunctionCall.Name == "request_human_takeover" {
								humanTakeoverRequested = true
								result.Success = false
								if reason, exists := args["reason"]; exists {
									if reasonStr, ok := reason.(string); ok {
										doneSummary = "Human takeover requested: " + reasonStr
									}
								}
							}
						}
						// Track steps from successful function responses
						if part.FunctionResponse != nil {
							funcName := part.FunctionResponse.Name
							respMap := part.FunctionResponse.Response
							// Check if response indicates success
							if success, exists := respMap["success"]; exists {
								if successBool, ok := success.(bool); ok && successBool {
									// Add the pending step if it exists and not done/get_page_state
									if step, exists := pendingSteps[funcName]; exists {
										if funcName != "done" && funcName != "get_page_state" {
											result.Steps = append(result.Steps, step)
										}
									}
								}
							}
							// Clean up pending step
							delete(pendingSteps, funcName)
						}
					}
				}
			}
		}
	}

	// Build result data - add summary if we have it
	dataMap, ok := result.Data.(map[string]any)
	if !ok || dataMap == nil {
		dataMap = make(map[string]any)
	}
	if doneSummary != "" {
		dataMap["summary"] = doneSummary
	}
	if lastResponse != "" && len(dataMap) == 0 {
		dataMap["response"] = lastResponse
	}
	result.Data = dataMap

	// Add stats to result
	if logger != nil {
		result.Duration = logger.TaskDuration()
		tokens := logger.GetTokens()
		if tokens != nil {
			result.TokensUsed = tokens.Used()
		}
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

// AnnotationConfig is an alias for browser.AnnotationConfig.
type AnnotationConfig = browser.AnnotationConfig

// DefaultAnnotationConfig returns the default annotation configuration.
func DefaultAnnotationConfig() *AnnotationConfig {
	return browser.DefaultAnnotationConfig()
}

// ShowAnnotations draws visual overlays on all detected elements in the browser.
// This helps visualize what elements the agent can see and interact with.
// Pass nil for cfg to use default settings.
func (a *Agent) ShowAnnotations(ctx context.Context, cfg *AnnotationConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return fmt.Errorf("agent not started, call Start() first")
	}

	// Get current element map
	elements, err := a.browser.GetElementMap(ctx)
	if err != nil {
		return fmt.Errorf("failed to get elements: %w", err)
	}

	return a.browser.ShowAnnotations(ctx, elements, cfg)
}

// HideAnnotations removes all annotation overlays from the browser.
func (a *Agent) HideAnnotations(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return fmt.Errorf("agent not started, call Start() first")
	}

	return a.browser.HideAnnotations(ctx)
}

// ToggleAnnotations shows or hides annotations based on current state.
// Returns true if annotations are now visible, false if hidden.
func (a *Agent) ToggleAnnotations(ctx context.Context, cfg *AnnotationConfig) (bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.browser == nil {
		return false, fmt.Errorf("agent not started, call Start() first")
	}

	// Get current element map
	elements, err := a.browser.GetElementMap(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to get elements: %w", err)
	}

	return a.browser.ToggleAnnotations(ctx, elements, cfg)
}

// GetAgent returns the underlying BrowserAgent for advanced use cases.
func (a *Agent) GetAgent() *agent.BrowserAgent {
	return a.browserAgent
}

// parseRateLimitDelay extracts the retry delay from a 429 rate limit error message.
// Returns the delay duration and true if this is a rate limit error, otherwise 0 and false.
func parseRateLimitDelay(errMsg string) (time.Duration, bool) {
	// Check if this is a rate limit error
	if !strings.Contains(errMsg, "429") && !strings.Contains(errMsg, "RESOURCE_EXHAUSTED") {
		return 0, false
	}

	// Try to extract retry delay from message like "Please retry in 29.924233789s."
	re := regexp.MustCompile(`retry in (\d+(?:\.\d+)?)s`)
	matches := re.FindStringSubmatch(errMsg)
	if len(matches) >= 2 {
		if seconds, err := strconv.ParseFloat(matches[1], 64); err == nil {
			return time.Duration(seconds*1000) * time.Millisecond, true
		}
	}

	// Also try "retryDelay:XXs" format from Details
	re2 := regexp.MustCompile(`retryDelay:(\d+)s`)
	matches2 := re2.FindStringSubmatch(errMsg)
	if len(matches2) >= 2 {
		if seconds, err := strconv.Atoi(matches2[1]); err == nil {
			return time.Duration(seconds) * time.Second, true
		}
	}

	// Default to 30 seconds if we can't parse
	return 30 * time.Second, true
}
