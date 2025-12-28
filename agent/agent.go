// Package agent provides the ADK-based browser automation agent.
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"

	"github.com/anxuanzi/bua-go/browser"
	"github.com/anxuanzi/bua-go/dom"
)

// Config holds agent configuration.
type Config struct {
	// APIKey is the Gemini API key.
	APIKey string

	// Model is the model ID to use.
	Model string

	// MaxIterations is the maximum number of agent loop iterations.
	MaxIterations int

	// MaxTokens is the maximum context window size.
	MaxTokens int

	// Debug enables verbose logging.
	Debug bool

	// ShowAnnotations enables visual element annotations before actions.
	ShowAnnotations bool

	// ScreenshotDir is the directory to save annotated screenshots.
	ScreenshotDir string
}

// BrowserAgent wraps an ADK agent with browser automation capabilities.
type BrowserAgent struct {
	config   Config
	browser  *browser.Browser
	adkAgent agent.Agent
	logger   *Logger
	tools    []tool.Tool

	// In-memory state for findings (since tool.Context doesn't provide state access)
	findings   []map[string]any
	findingsMu sync.RWMutex
}

// New creates a new browser agent.
func New(cfg Config, b *browser.Browser) *BrowserAgent {
	if cfg.MaxIterations == 0 {
		cfg.MaxIterations = 50
	}
	if cfg.MaxTokens == 0 {
		cfg.MaxTokens = 1048576 // gemini-3-flash-preview input limit
	}
	if cfg.Model == "" {
		cfg.Model = "gemini-3-flash-preview"
	}

	return &BrowserAgent{
		config:   cfg,
		browser:  b,
		logger:   NewLogger(cfg.Debug),
		findings: []map[string]any{},
	}
}

// Init initializes the ADK agent with browser tools.
func (a *BrowserAgent) Init(ctx context.Context) error {
	// Get API key
	apiKey := a.config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}

	// Create Gemini model
	model, err := gemini.NewModel(ctx, a.config.Model, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return fmt.Errorf("failed to create Gemini model: %w", err)
	}

	// Create browser tools
	tools, err := a.createBrowserTools()
	if err != nil {
		return fmt.Errorf("failed to create browser tools: %w", err)
	}
	a.tools = tools

	// Create ADK agent
	adkAgent, err := llmagent.New(llmagent.Config{
		Name:        "browser_automation_agent",
		Model:       model,
		Description: "A browser automation agent that can navigate websites, interact with elements, and extract data.",
		Instruction: SystemPrompt(),
		Tools:       tools,
		GenerateContentConfig: &genai.GenerateContentConfig{
			Temperature:     genai.Ptr[float32](0.2),
			MaxOutputTokens: 16384, // Conservative output limit (model supports 65536)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create ADK agent: %w", err)
	}
	a.adkAgent = adkAgent

	return nil
}

// preAction is called before browser actions to show annotations and capture state.
func (a *BrowserAgent) preAction() {
	if a.browser == nil || !a.config.ShowAnnotations {
		return
	}

	bgCtx := context.Background()

	// Get element map
	elements, err := a.browser.GetElementMap(bgCtx)
	if err != nil {
		a.logger.Error("preAction/GetElementMap", err)
		return
	}

	// Show annotations in browser
	err = a.browser.ShowAnnotations(bgCtx, elements, nil)
	if err != nil {
		a.logger.Error("preAction/ShowAnnotations", err)
	} else {
		a.logger.Annotation(elements.Count())
	}

	// Take annotated screenshot
	if a.config.ScreenshotDir != "" {
		screenshot, err := a.browser.ScreenshotWithAnnotations(bgCtx, elements)
		if err != nil {
			a.logger.Error("preAction/Screenshot", err)
			return
		}

		filename := fmt.Sprintf("step_%03d_%s.png",
			a.logger.GetStep()+1,
			time.Now().Format("150405"))
		a.saveScreenshotToFile(screenshot, filename)
	}
}

// postAction is called after browser actions to clean up annotations.
func (a *BrowserAgent) postAction() {
	if a.browser == nil || !a.config.ShowAnnotations {
		return
	}

	bgCtx := context.Background()

	// Hide annotations after action
	if err := a.browser.HideAnnotations(bgCtx); err != nil {
		a.logger.Error("postAction/HideAnnotations", err)
	}

	// Wait for page to stabilize
	a.browser.WaitForStable(bgCtx)
}

// saveScreenshotToFile saves screenshot to disk as fallback.
func (a *BrowserAgent) saveScreenshotToFile(data []byte, filename string) {
	path := filepath.Join(a.config.ScreenshotDir, filename)
	if err := os.MkdirAll(a.config.ScreenshotDir, 0755); err != nil {
		a.logger.Error("saveScreenshotToFile/MkdirAll", err)
		return
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		a.logger.Error("saveScreenshotToFile/WriteFile", err)
		return
	}
	a.logger.Screenshot(path, true)
}

// createBrowserTools creates the function tools for browser automation.
func (a *BrowserAgent) createBrowserTools() ([]tool.Tool, error) {
	var tools []tool.Tool

	// Click tool
	clickHandler := func(ctx tool.Context, input ClickInput) (ClickOutput, error) {
		if a.browser == nil {
			return ClickOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.preAction()
		defer a.postAction()

		a.logger.Click(input.ElementIndex, input.Reasoning)

		err := a.browser.Click(context.Background(), input.ElementIndex)
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return ClickOutput{Success: false, Message: err.Error()}, nil
		}

		msg := fmt.Sprintf("Clicked element %d", input.ElementIndex)
		a.logger.ActionResult(true, msg)
		return ClickOutput{Success: true, Message: msg}, nil
	}
	clickTool, err := functiontool.New(
		functiontool.Config{
			Name:        "click",
			Description: "Click on an element by its index number shown in the annotated screenshot and element map.",
		},
		clickHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create click tool: %w", err)
	}
	tools = append(tools, clickTool)

	// Type tool
	typeHandler := func(ctx tool.Context, input TypeInput) (TypeOutput, error) {
		if a.browser == nil {
			return TypeOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.preAction()
		defer a.postAction()

		a.logger.Type(input.ElementIndex, input.Text, input.Reasoning)

		err := a.browser.TypeInElement(context.Background(), input.ElementIndex, input.Text)
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return TypeOutput{Success: false, Message: err.Error()}, nil
		}

		msg := fmt.Sprintf("Typed '%s' into element %d", input.Text, input.ElementIndex)
		a.logger.ActionResult(true, msg)
		return TypeOutput{Success: true, Message: msg}, nil
	}
	typeTool, err := functiontool.New(
		functiontool.Config{
			Name:        "type_text",
			Description: "Type text into an input field. First clicks the element to focus it, then types the text.",
		},
		typeHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create type tool: %w", err)
	}
	tools = append(tools, typeTool)

	// Scroll tool
	scrollHandler := func(ctx tool.Context, input ScrollInput) (ScrollOutput, error) {
		if a.browser == nil {
			return ScrollOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.preAction()
		defer a.postAction()

		amount := input.Amount
		if amount == 0 {
			amount = 500
		}

		a.logger.Scroll(input.Direction, amount, input.Reasoning)

		var deltaY float64
		switch input.Direction {
		case "up":
			deltaY = -float64(amount)
		case "down":
			deltaY = float64(amount)
		default:
			a.logger.ActionResult(false, "Invalid direction")
			return ScrollOutput{Success: false, Message: "Invalid direction. Use: up or down"}, nil
		}

		err := a.browser.Scroll(context.Background(), 0, deltaY)
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return ScrollOutput{Success: false, Message: err.Error()}, nil
		}

		msg := fmt.Sprintf("Scrolled %s by %d pixels", input.Direction, amount)
		a.logger.ActionResult(true, msg)
		return ScrollOutput{Success: true, Message: msg}, nil
	}
	scrollTool, err := functiontool.New(
		functiontool.Config{
			Name:        "scroll",
			Description: "Scroll the page in a direction (up or down) to reveal more content.",
		},
		scrollHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create scroll tool: %w", err)
	}
	tools = append(tools, scrollTool)

	// Navigate tool
	navigateHandler := func(ctx tool.Context, input NavigateInput) (NavigateOutput, error) {
		if a.browser == nil {
			return NavigateOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.preAction()
		defer a.postAction()

		a.logger.Navigate(input.URL)

		err := a.browser.Navigate(context.Background(), input.URL)
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return NavigateOutput{Success: false, Message: err.Error()}, nil
		}

		url := a.browser.GetURL()
		title := a.browser.GetTitle()
		a.logger.ActionResult(true, fmt.Sprintf("Loaded: %s", title))

		return NavigateOutput{
			Success: true,
			Message: fmt.Sprintf("Navigated to %s", input.URL),
			URL:     url,
			Title:   title,
		}, nil
	}
	navigateTool, err := functiontool.New(
		functiontool.Config{
			Name:        "navigate",
			Description: "Navigate to a specific URL.",
		},
		navigateHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create navigate tool: %w", err)
	}
	tools = append(tools, navigateTool)

	// Wait tool
	waitHandler := func(ctx tool.Context, input WaitInput) (WaitOutput, error) {
		if a.browser == nil {
			return WaitOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.logger.Wait(input.Reason)

		err := a.browser.WaitForStable(context.Background())
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return WaitOutput{Success: false, Message: err.Error()}, nil
		}

		msg := fmt.Sprintf("Waited for page to stabilize: %s", input.Reason)
		a.logger.ActionResult(true, "Page stable")
		return WaitOutput{Success: true, Message: msg}, nil
	}
	waitTool, err := functiontool.New(
		functiontool.Config{
			Name:        "wait",
			Description: "Wait for the page to stabilize after an action or for dynamic content to load.",
		},
		waitHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create wait tool: %w", err)
	}
	tools = append(tools, waitTool)

	// Extract tool
	extractHandler := func(ctx tool.Context, input ExtractInput) (ExtractOutput, error) {
		if a.browser == nil {
			return ExtractOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.logger.Extract(input.WhatToExtract)

		data := make(map[string]any)
		if input.ElementIndex < 0 {
			data["url"] = a.browser.GetURL()
			data["title"] = a.browser.GetTitle()
			elements, err := a.browser.GetElementMap(context.Background())
			if err == nil {
				data["element_count"] = elements.Count()
			}
			a.logger.ActionResult(true, "Extracted page info")
		} else {
			elements, err := a.browser.GetElementMap(context.Background())
			if err != nil {
				a.logger.ActionResult(false, err.Error())
				return ExtractOutput{Success: false, Message: err.Error()}, nil
			}
			el, ok := elements.ByIndex(input.ElementIndex)
			if !ok {
				msg := fmt.Sprintf("Element %d not found", input.ElementIndex)
				a.logger.ActionResult(false, msg)
				return ExtractOutput{Success: false, Message: msg}, nil
			}
			data["tag"] = el.TagName
			data["text"] = el.Text
			if el.Href != "" {
				data["href"] = el.Href
			}
			if el.Value != "" {
				data["value"] = el.Value
			}
			a.logger.ActionResult(true, fmt.Sprintf("Extracted from element %d", input.ElementIndex))
		}

		return ExtractOutput{
			Success: true,
			Message: "Data extracted successfully",
			Data:    data,
		}, nil
	}
	extractTool, err := functiontool.New(
		functiontool.Config{
			Name:        "extract",
			Description: "Extract data from an element or the page. Use element_index=-1 to extract general page information.",
		},
		extractHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create extract tool: %w", err)
	}
	tools = append(tools, extractTool)

	// Get page state tool
	getPageStateHandler := func(ctx tool.Context, input GetPageStateInput) (GetPageStateOutput, error) {
		if a.browser == nil {
			return GetPageStateOutput{Success: false, Error: "Browser not initialized"}, nil
		}

		bgCtx := context.Background()
		output := GetPageStateOutput{
			Success: true,
			URL:     a.browser.GetURL(),
			Title:   a.browser.GetTitle(),
		}

		elements, err := a.browser.GetElementMap(bgCtx)
		if err != nil {
			output.Error = fmt.Sprintf("Failed to get element map: %v", err)
			a.logger.Error("get_page_state", err)
			return output, nil
		}

		output.ElementMap = elements.ToTokenString()
		a.logger.PageState(output.URL, output.Title, elements.Count())

		return output, nil
	}
	pageStateTool, err := functiontool.New(
		functiontool.Config{
			Name:        "get_page_state",
			Description: "Get the current page state including URL, title, and interactive elements. Call this to see what's on the page.",
		},
		getPageStateHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create page state tool: %w", err)
	}
	tools = append(tools, pageStateTool)

	// Save finding tool - for persisting important discoveries
	saveFindingHandler := func(ctx tool.Context, input SaveFindingInput) (SaveFindingOutput, error) {
		a.logger.Info("save_finding: Saving: %s - %s", input.Category, input.Title)

		// Create a structured finding to save
		finding := map[string]any{
			"category":   input.Category,
			"title":      input.Title,
			"details":    input.Details,
			"source_url": a.browser.GetURL(),
			"timestamp":  time.Now().Format(time.RFC3339),
		}

		// Save to agent's internal findings store
		a.findingsMu.Lock()
		a.findings = append(a.findings, finding)
		totalSaved := len(a.findings)
		a.findingsMu.Unlock()

		// Generate finding ID
		findingID := fmt.Sprintf("finding_%s_%s_%d",
			input.Category,
			sanitizeFilename(input.Title),
			time.Now().Unix())

		return SaveFindingOutput{
			Success:    true,
			Message:    fmt.Sprintf("Saved finding: %s", input.Title),
			FindingID:  findingID,
			TotalSaved: totalSaved,
		}, nil
	}
	saveFindingTool, err := functiontool.New(
		functiontool.Config{
			Name:        "save_finding",
			Description: "Save an important finding or discovery to memory. Use this to remember leads, contacts, important data, or any information you need to recall later. Findings persist across the entire session.",
		},
		saveFindingHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create save_finding tool: %w", err)
	}
	tools = append(tools, saveFindingTool)

	// Search findings tool - for retrieving saved findings
	searchFindingsHandler := func(ctx tool.Context, input SearchFindingsInput) (SearchFindingsOutput, error) {
		a.logger.Info("search_findings: Searching: %s (category: %s)", input.Query, input.Category)

		a.findingsMu.RLock()
		allFindings := make([]map[string]any, len(a.findings))
		copy(allFindings, a.findings)
		a.findingsMu.RUnlock()

		var results []map[string]any

		// Filter by category if provided
		if input.Category != "" {
			for _, finding := range allFindings {
				cat, _ := finding["category"].(string)
				if cat == input.Category {
					results = append(results, finding)
				}
			}
		} else {
			results = allFindings
		}

		// Filter by query if provided
		if input.Query != "" && len(results) > 0 {
			filtered := []map[string]any{}
			queryLower := strings.ToLower(input.Query)
			for _, finding := range results {
				title, _ := finding["title"].(string)
				details, _ := finding["details"].(string)
				if strings.Contains(strings.ToLower(title), queryLower) ||
					strings.Contains(strings.ToLower(details), queryLower) {
					filtered = append(filtered, finding)
				}
			}
			results = filtered
		}

		return SearchFindingsOutput{
			Success:  true,
			Findings: results,
			Count:    len(results),
		}, nil
	}
	searchFindingsTool, err := functiontool.New(
		functiontool.Config{
			Name:        "search_findings",
			Description: "Search through previously saved findings. Use this to recall leads, contacts, or any data you saved earlier. You can filter by category (e.g., 'lead', 'contact', 'product') or search by keyword.",
		},
		searchFindingsHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create search_findings tool: %w", err)
	}
	tools = append(tools, searchFindingsTool)

	// Multi-tab tools
	newTabHandler := func(ctx tool.Context, input NewTabInput) (NewTabOutput, error) {
		if a.browser == nil {
			return NewTabOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.preAction()
		defer a.postAction()

		a.logger.Info("new_tab: Opening: %s", input.URL)

		tabID, err := a.browser.NewTab(context.Background(), input.URL)
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return NewTabOutput{Success: false, Message: err.Error()}, nil
		}

		return NewTabOutput{
			Success: true,
			Message: fmt.Sprintf("Opened new tab: %s", tabID),
			TabID:   tabID,
			URL:     input.URL,
		}, nil
	}
	newTabTool, err := functiontool.New(
		functiontool.Config{
			Name:        "new_tab",
			Description: "Open a new browser tab with the specified URL. Returns the tab ID for later reference.",
		},
		newTabHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create new_tab tool: %w", err)
	}
	tools = append(tools, newTabTool)

	switchTabHandler := func(ctx tool.Context, input SwitchTabInput) (SwitchTabOutput, error) {
		if a.browser == nil {
			return SwitchTabOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.preAction()
		defer a.postAction()

		a.logger.Info("switch_tab: Switching to: %s", input.TabID)

		err := a.browser.SwitchTab(context.Background(), input.TabID)
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return SwitchTabOutput{Success: false, Message: err.Error()}, nil
		}

		return SwitchTabOutput{
			Success: true,
			Message: fmt.Sprintf("Switched to tab: %s", input.TabID),
			URL:     a.browser.GetURL(),
			Title:   a.browser.GetTitle(),
		}, nil
	}
	switchTabTool, err := functiontool.New(
		functiontool.Config{
			Name:        "switch_tab",
			Description: "Switch to a different browser tab by its ID. Use list_tabs to see available tabs.",
		},
		switchTabHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create switch_tab tool: %w", err)
	}
	tools = append(tools, switchTabTool)

	closeTabHandler := func(ctx tool.Context, input CloseTabInput) (CloseTabOutput, error) {
		if a.browser == nil {
			return CloseTabOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.logger.Info("close_tab: Closing: %s", input.TabID)

		err := a.browser.CloseTab(context.Background(), input.TabID)
		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return CloseTabOutput{Success: false, Message: err.Error()}, nil
		}

		return CloseTabOutput{
			Success: true,
			Message: fmt.Sprintf("Closed tab: %s", input.TabID),
		}, nil
	}
	closeTabTool, err := functiontool.New(
		functiontool.Config{
			Name:        "close_tab",
			Description: "Close a browser tab by its ID.",
		},
		closeTabHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create close_tab tool: %w", err)
	}
	tools = append(tools, closeTabTool)

	listTabsHandler := func(ctx tool.Context, input ListTabsInput) (ListTabsOutput, error) {
		if a.browser == nil {
			return ListTabsOutput{Success: false, Error: "Browser not initialized"}, nil
		}

		tabs := a.browser.ListTabs(context.Background())
		activeTab := a.browser.GetActiveTabID()

		var tabInfos []TabInfo
		for _, tab := range tabs {
			tabInfos = append(tabInfos, TabInfo{
				TabID:  tab.ID,
				URL:    tab.URL,
				Title:  tab.Title,
				Active: tab.ID == activeTab,
			})
		}

		return ListTabsOutput{
			Success:   true,
			Tabs:      tabInfos,
			ActiveTab: activeTab,
		}, nil
	}
	listTabsTool, err := functiontool.New(
		functiontool.Config{
			Name:        "list_tabs",
			Description: "List all open browser tabs with their IDs, URLs, and titles.",
		},
		listTabsHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create list_tabs tool: %w", err)
	}
	tools = append(tools, listTabsTool)

	// Download file tool
	downloadHandler := func(ctx tool.Context, input DownloadFileInput) (DownloadFileOutput, error) {
		if a.browser == nil {
			return DownloadFileOutput{Success: false, Message: "Browser not initialized"}, nil
		}

		a.logger.Info("download_file: Downloading from URL: %s (use_page_auth: %v)", input.URL, input.UsePageAuth)

		cfg := browser.DefaultDownloadConfig()
		// DefaultDownloadConfig already sets ~/.bua/downloads/

		var downloadInfo *browser.DownloadInfo
		var err error

		if input.UsePageAuth {
			// Use browser context with cookies/auth
			downloadInfo, err = a.browser.DownloadResource(context.Background(), input.URL, cfg)
		} else {
			// Use direct HTTP download
			downloadInfo, err = a.browser.DownloadFile(context.Background(), input.URL, cfg)
		}

		if err != nil {
			a.logger.ActionResult(false, err.Error())
			return DownloadFileOutput{Success: false, Message: err.Error()}, nil
		}

		msg := fmt.Sprintf("Downloaded: %s (%d bytes)", downloadInfo.Filename, downloadInfo.Size)
		a.logger.ActionResult(true, msg)

		return DownloadFileOutput{
			Success:  true,
			Message:  msg,
			Filename: downloadInfo.Filename,
			FilePath: downloadInfo.FilePath,
			Size:     downloadInfo.Size,
			MimeType: downloadInfo.MimeType,
		}, nil
	}
	downloadTool, err := functiontool.New(
		functiontool.Config{
			Name:        "download_file",
			Description: "Download a file from a URL. Use use_page_auth=true to use the browser's cookies and authentication context for authenticated downloads.",
		},
		downloadHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create download_file tool: %w", err)
	}
	tools = append(tools, downloadTool)

	// Request human takeover tool
	humanTakeoverHandler := func(ctx tool.Context, input HumanTakeoverInput) (HumanTakeoverOutput, error) {
		a.logger.HumanTakeover(input.Reason)

		return HumanTakeoverOutput{
			Success:   true,
			Message:   fmt.Sprintf("Human takeover requested: %s. Please complete the action and confirm.", input.Reason),
			Completed: false,
		}, nil
	}
	humanTool, err := functiontool.New(
		functiontool.Config{
			Name:        "request_human_takeover",
			Description: "Request a human to take over for tasks like login, CAPTCHA, or other actions requiring human intervention.",
		},
		humanTakeoverHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create human takeover tool: %w", err)
	}
	tools = append(tools, humanTool)

	// Done tool
	doneHandler := func(ctx tool.Context, input DoneInput) (DoneOutput, error) {
		a.logger.Done(input.Success, input.Summary)

		// Get all findings count from agent's internal store
		a.findingsMu.RLock()
		totalFindings := len(a.findings)
		a.findingsMu.RUnlock()

		return DoneOutput{
			Success:       input.Success,
			Summary:       input.Summary,
			TotalFindings: totalFindings,
		}, nil
	}
	doneTool, err := functiontool.New(
		functiontool.Config{
			Name:        "done",
			Description: "Indicate that the task is complete. Set success=true if the task was accomplished, false otherwise.",
		},
		doneHandler,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create done tool: %w", err)
	}
	tools = append(tools, doneTool)

	return tools, nil
}

// Helper functions

func sanitizeFilename(s string) string {
	// Simple sanitization - replace non-alphanumeric with underscore
	result := ""
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			result += string(c)
		} else if len(result) > 0 && result[len(result)-1] != '_' {
			result += "_"
		}
	}
	if len(result) > 50 {
		result = result[:50]
	}
	return result
}

// Tool input/output types

type ClickInput struct {
	ElementIndex int    `json:"element_index" jsonschema:"The index number of the element to click (shown in the element map)"`
	Reasoning    string `json:"reasoning" jsonschema:"Brief explanation of why you're clicking this element"`
}

type ClickOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type TypeInput struct {
	ElementIndex int    `json:"element_index" jsonschema:"The index number of the input element"`
	Text         string `json:"text" jsonschema:"The text to type into the element"`
	Reasoning    string `json:"reasoning" jsonschema:"Brief explanation of why you're typing this text"`
}

type TypeOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ScrollInput struct {
	Direction string `json:"direction" jsonschema:"Direction to scroll: up or down"`
	Amount    int    `json:"amount" jsonschema:"Amount to scroll in pixels (default 500)"`
	Reasoning string `json:"reasoning" jsonschema:"Brief explanation of why you're scrolling"`
}

type ScrollOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type NavigateInput struct {
	URL       string `json:"url" jsonschema:"The URL to navigate to"`
	Reasoning string `json:"reasoning" jsonschema:"Brief explanation of why you're navigating to this URL"`
}

type NavigateOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	URL     string `json:"url,omitempty"`
	Title   string `json:"title,omitempty"`
}

type WaitInput struct {
	Reason string `json:"reason" jsonschema:"What you're waiting for"`
}

type WaitOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ExtractInput struct {
	ElementIndex  int    `json:"element_index" jsonschema:"The index of the element to extract from (-1 for entire page)"`
	WhatToExtract string `json:"what_to_extract" jsonschema:"Description of what data to extract"`
}

type ExtractOutput struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

type GetPageStateInput struct {
	IncludeScreenshot bool `json:"include_screenshot" jsonschema:"Whether to include the annotated screenshot (default true)"`
}

type GetPageStateOutput struct {
	Success    bool   `json:"success"`
	URL        string `json:"url"`
	Title      string `json:"title"`
	ElementMap string `json:"element_map"`
	Screenshot string `json:"screenshot,omitempty"`
	Error      string `json:"error,omitempty"`
}

// Memory tools input/output types

type SaveFindingInput struct {
	Category string `json:"category" jsonschema:"Category of the finding (e.g., 'lead', 'contact', 'product', 'link', 'data')"`
	Title    string `json:"title" jsonschema:"Short title or identifier for this finding"`
	Details  string `json:"details" jsonschema:"Detailed information about this finding (include all relevant data)"`
}

type SaveFindingOutput struct {
	Success    bool   `json:"success"`
	Message    string `json:"message"`
	FindingID  string `json:"finding_id"`
	TotalSaved int    `json:"total_saved"`
}

type SearchFindingsInput struct {
	Query    string `json:"query" jsonschema:"Search query to find relevant findings"`
	Category string `json:"category,omitempty" jsonschema:"Optional: filter by category (e.g., 'lead', 'contact')"`
}

type SearchFindingsOutput struct {
	Success  bool             `json:"success"`
	Findings []map[string]any `json:"findings"`
	Count    int              `json:"count"`
}

// Multi-tab input/output types

type NewTabInput struct {
	URL string `json:"url" jsonschema:"The URL to open in the new tab"`
}

type NewTabOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	TabID   string `json:"tab_id"`
	URL     string `json:"url"`
}

type SwitchTabInput struct {
	TabID string `json:"tab_id" jsonschema:"The ID of the tab to switch to"`
}

type SwitchTabOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	URL     string `json:"url"`
	Title   string `json:"title"`
}

type CloseTabInput struct {
	TabID string `json:"tab_id" jsonschema:"The ID of the tab to close"`
}

type CloseTabOutput struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type ListTabsInput struct{}

type TabInfo struct {
	TabID  string `json:"tab_id"`
	URL    string `json:"url"`
	Title  string `json:"title"`
	Active bool   `json:"active"`
}

type ListTabsOutput struct {
	Success   bool      `json:"success"`
	Tabs      []TabInfo `json:"tabs"`
	ActiveTab string    `json:"active_tab"`
	Error     string    `json:"error,omitempty"`
}

type HumanTakeoverInput struct {
	Reason string `json:"reason" jsonschema:"Why human intervention is needed"`
}

type HumanTakeoverOutput struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Completed bool   `json:"completed"`
}

type DoneInput struct {
	Success       bool   `json:"success" jsonschema:"Whether the task was completed successfully"`
	Summary       string `json:"summary" jsonschema:"Summary of what was accomplished"`
	ExtractedData string `json:"extracted_data,omitempty" jsonschema:"Any data that was extracted during the task (as JSON)"`
}

type DoneOutput struct {
	Success       bool   `json:"success"`
	Summary       string `json:"summary"`
	TotalFindings int    `json:"total_findings"`
}

// Download tool input/output types

type DownloadFileInput struct {
	URL         string `json:"url" jsonschema:"The URL of the file to download"`
	Filename    string `json:"filename,omitempty" jsonschema:"Optional: custom filename for the downloaded file"`
	UsePageAuth bool   `json:"use_page_auth,omitempty" jsonschema:"If true, use the page's cookies and auth context for the download"`
	Reasoning   string `json:"reasoning" jsonschema:"Brief explanation of why you're downloading this file"`
}

type DownloadFileOutput struct {
	Success  bool   `json:"success"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	FilePath string `json:"file_path,omitempty"`
	Size     int64  `json:"size,omitempty"`
	MimeType string `json:"mime_type,omitempty"`
}

// GetADKAgent returns the underlying ADK agent for advanced use cases.
func (a *BrowserAgent) GetADKAgent() agent.Agent {
	return a.adkAgent
}

// GetBrowser returns the browser instance.
func (a *BrowserAgent) GetBrowser() *browser.Browser {
	return a.browser
}

// Tools returns the browser tools for use in other agents.
func (a *BrowserAgent) Tools() []tool.Tool {
	return a.tools
}

// GetFindings returns all findings collected during task execution.
func (a *BrowserAgent) GetFindings() []map[string]any {
	a.findingsMu.RLock()
	defer a.findingsMu.RUnlock()

	// Return a copy to prevent mutation
	result := make([]map[string]any, len(a.findings))
	for i, finding := range a.findings {
		copied := make(map[string]any)
		for k, v := range finding {
			copied[k] = v
		}
		result[i] = copied
	}
	return result
}

// GetLogger returns the logger for external token/timing updates.
func (a *BrowserAgent) GetLogger() *Logger {
	return a.logger
}

// Result represents the result of a task execution.
type Result struct {
	Success         bool
	Data            map[string]any
	Error           string
	Steps           []Step
	TokensUsed      int
	ScreenshotPaths []string
}

// Step represents a single step in the execution.
type Step struct {
	Action         string
	Target         string
	Reasoning      string
	URL            string
	Title          string
	ScreenshotPath string
}

// PageState represents the current state of the page.
type PageState struct {
	URL           string
	Title         string
	Elements      *dom.ElementMap
	Screenshot    []byte
	ScreenshotB64 string
}
