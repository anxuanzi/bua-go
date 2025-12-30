// Package browser provides the browser automation layer using go-rod.
package browser

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/google/uuid"
	"golang.org/x/image/draw"

	"github.com/anxuanzi/bua-go/dom"
	"github.com/anxuanzi/bua-go/screenshot"
)

// Viewport defines browser viewport dimensions.
type Viewport struct {
	Width  int
	Height int
}

// Config holds browser configuration.
type Config struct {
	Viewport         *Viewport
	ScreenshotConfig *screenshot.Config
}

// TabInfo contains information about a browser tab.
type TabInfo struct {
	ID    string
	URL   string
	Title string
}

// Browser wraps a rod browser for controlled automation.
// Supports multi-tab management.
type Browser struct {
	rod      *rod.Browser
	config   Config
	screener *screenshot.Manager

	// Multi-tab support
	pages       map[string]*rod.Page // tabID -> page
	activeTabID string               // currently active tab

	// Action highlighting
	highlighter      *Highlighter
	highlightEnabled bool
	highlightDelay   time.Duration

	// Deprecated: use pages map instead
	page *rod.Page

	mu sync.RWMutex
}

// New creates a new browser wrapper.
func New(rodBrowser *rod.Browser, cfg Config) *Browser {
	b := &Browser{
		rod:              rodBrowser,
		config:           cfg,
		pages:            make(map[string]*rod.Page),
		highlightEnabled: true,                   // Enable by default
		highlightDelay:   300 * time.Millisecond, // Default 300ms visual feedback
	}

	if cfg.ScreenshotConfig != nil {
		b.screener = screenshot.NewManager(cfg.ScreenshotConfig)
	}

	return b
}

// SetHighlightEnabled enables or disables action highlighting.
func (b *Browser) SetHighlightEnabled(enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.highlightEnabled = enabled
	if b.highlighter != nil {
		b.highlighter.SetEnabled(enabled)
	}
}

// SetHighlightDelay sets how long highlights are shown before action execution.
func (b *Browser) SetHighlightDelay(d time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.highlightDelay = d
	if b.highlighter != nil {
		b.highlighter.SetDelay(d)
	}
}

// getHighlighter returns a highlighter for the active page.
func (b *Browser) getHighlighter() *Highlighter {
	page := b.getActivePageLocked()
	if page == nil {
		return nil
	}
	if b.highlighter == nil || b.highlighter.page != page {
		b.highlighter = NewHighlighter(page, b.highlightEnabled)
		b.highlighter.SetDelay(b.highlightDelay)
	}
	return b.highlighter
}

// waitForStableWithTimeout waits for the page to stabilize with an overall timeout.
// This prevents indefinite blocking on pages with continuous animations/video (e.g., Instagram Reels).
// stabilityDuration: how long the page must be stable (no DOM changes) to be considered "ready"
// maxWait: maximum total time to wait for stability before giving up
func waitForStableWithTimeout(page *rod.Page, stabilityDuration, maxWait time.Duration) {
	if page == nil {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		// WaitStable waits until page hasn't changed for stabilityDuration
		_ = page.WaitStable(stabilityDuration)
	}()

	select {
	case <-done:
		// Page became stable within timeout
	case <-time.After(maxWait):
		// Timeout reached - page may still be loading/animating but we continue anyway
	}
}

// Navigate navigates to the specified URL.
// If no tab exists, creates a new one.
func (b *Browser) Navigate(ctx context.Context, url string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Get current page (create if needed)
	page := b.getActivePageLocked()
	if page == nil {
		// Create first tab
		tabID, err := b.createTabLocked(url)
		if err != nil {
			return err
		}
		page = b.pages[tabID]
	} else {
		// Navigate existing page
		err := page.Navigate(url)
		if err != nil {
			return fmt.Errorf("failed to navigate: %w", err)
		}
	}

	// Wait for page to be ready
	err := page.WaitLoad()
	if err != nil {
		return fmt.Errorf("failed to wait for page load: %w", err)
	}

	// Wait for page to stabilize with timeout to avoid blocking on animated/video pages
	// Use 300ms stability requirement, max 5 seconds total wait
	waitForStableWithTimeout(page, 300*time.Millisecond, 5*time.Second)

	return nil
}

// createTabLocked creates a new tab (must hold lock).
func (b *Browser) createTabLocked(url string) (string, error) {
	// Create a new page
	page, err := b.rod.Page(proto.TargetCreateTarget{URL: url})
	if err != nil {
		return "", fmt.Errorf("failed to create page: %w", err)
	}

	// Set viewport
	if b.config.Viewport != nil {
		err := page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
			Width:             b.config.Viewport.Width,
			Height:            b.config.Viewport.Height,
			DeviceScaleFactor: 1.0,
			Mobile:            false,
		})
		if err != nil {
			return "", fmt.Errorf("failed to set viewport: %w", err)
		}
	}

	// Generate tab ID
	tabID := uuid.New().String()[:8]

	// Store tab
	b.pages[tabID] = page
	b.activeTabID = tabID

	// Also maintain backward compatibility
	b.page = page

	return tabID, nil
}

// getActivePageLocked returns the active page (must hold lock).
func (b *Browser) getActivePageLocked() *rod.Page {
	if b.activeTabID != "" {
		if page, ok := b.pages[b.activeTabID]; ok {
			return page
		}
	}
	// Fallback to legacy single page
	return b.page
}

// Screenshot takes a screenshot of the current page.
func (b *Browser) Screenshot(ctx context.Context) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return nil, fmt.Errorf("no active page")
	}

	// Use viewport screenshot (false) instead of full-page (true) to avoid
	// fixed overlay elements being captured multiple times during page stitching
	data, err := page.Screenshot(false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return data, nil
}

// ScreenshotWithAnnotations takes an annotated screenshot with element indices.
func (b *Browser) ScreenshotWithAnnotations(ctx context.Context, elements *dom.ElementMap) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return nil, fmt.Errorf("no active page")
	}

	// Take viewport screenshot (false) - full-page (true) causes fixed overlays to repeat
	data, err := page.Screenshot(false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Annotate if we have elements and a screener
	if elements != nil && b.screener != nil {
		annotated, err := b.screener.Annotate(data, elements)
		if err != nil {
			return nil, fmt.Errorf("failed to annotate screenshot: %w", err)
		}
		return annotated, nil
	}

	return data, nil
}

// SaveScreenshot saves a screenshot to storage and returns the path.
func (b *Browser) SaveScreenshot(ctx context.Context, data []byte, name string) (string, error) {
	if b.screener == nil {
		return "", fmt.Errorf("screenshot manager not configured")
	}

	return b.screener.Save(data, name)
}

// ScreenshotForLLM takes a compressed screenshot optimized for LLM context.
// It resizes to maxWidth and uses JPEG compression to minimize token usage.
// A 1280x800 PNG (~500KB) becomes a 640x400 JPEG (~30-50KB) - 10-15x smaller.
func (b *Browser) ScreenshotForLLM(ctx context.Context, maxWidth int, quality int) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return nil, fmt.Errorf("no active page")
	}

	// Default values for LLM optimization
	if maxWidth <= 0 {
		maxWidth = 800 // Reduced from 1280 - still readable, much smaller
	}
	if quality <= 0 {
		quality = 60 // JPEG quality 60 is good balance of size/quality
	}

	// Take viewport screenshot
	data, err := page.Screenshot(false, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	// Decode PNG
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode screenshot: %w", err)
	}

	// Calculate new dimensions maintaining aspect ratio
	bounds := img.Bounds()
	origWidth := bounds.Dx()
	origHeight := bounds.Dy()

	if origWidth <= maxWidth {
		// Image is already small enough, just convert to JPEG
		return compressToJPEG(img, quality)
	}

	// Calculate new height maintaining aspect ratio
	newWidth := maxWidth
	newHeight := (origHeight * maxWidth) / origWidth

	// Resize image
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.BiLinear.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	// Compress to JPEG
	return compressToJPEG(resized, quality)
}

// compressToJPEG converts an image to JPEG with specified quality.
func compressToJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality})
	if err != nil {
		return nil, fmt.Errorf("failed to encode JPEG: %w", err)
	}
	return buf.Bytes(), nil
}

// GetElementMap extracts the element map from the current page.
func (b *Browser) GetElementMap(ctx context.Context) (*dom.ElementMap, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return nil, fmt.Errorf("no active page")
	}

	return dom.ExtractElementMap(ctx, page)
}

// GetAccessibilityTree extracts the accessibility tree from the current page.
func (b *Browser) GetAccessibilityTree(ctx context.Context) (*dom.AccessibilityTree, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return nil, fmt.Errorf("no active page")
	}

	return dom.ExtractAccessibilityTree(ctx, page)
}

// Click clicks on an element by its index in the element map.
func (b *Browser) Click(ctx context.Context, elementIndex int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	// Get the element map to find the element
	elements, err := dom.ExtractElementMap(ctx, page)
	if err != nil {
		return fmt.Errorf("failed to get element map: %w", err)
	}

	el, ok := elements.ByIndex(elementIndex)
	if !ok {
		return fmt.Errorf("element with index %d not found", elementIndex)
	}

	// Show highlight before click
	if highlighter := b.getHighlighter(); highlighter != nil {
		label := fmt.Sprintf("click [%d]", elementIndex)
		_ = highlighter.HighlightElement(
			el.BoundingBox.X,
			el.BoundingBox.Y,
			el.BoundingBox.Width,
			el.BoundingBox.Height,
			label,
		)
		defer highlighter.RemoveHighlights()
	}

	// Click at the center of the element using JavaScript
	centerX := el.BoundingBox.X + el.BoundingBox.Width/2
	centerY := el.BoundingBox.Y + el.BoundingBox.Height/2

	// Use CDP to click at coordinates
	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseMoved,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 0,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to move mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMousePressed,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to press mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseReleased,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to release mouse: %w", err)
	}

	return nil
}

// ClickElement clicks on an element directly.
func (b *Browser) ClickElement(ctx context.Context, el *dom.Element) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	// Show highlight before click
	if highlighter := b.getHighlighter(); highlighter != nil {
		label := fmt.Sprintf("click [%d]", el.Index)
		_ = highlighter.HighlightElement(
			el.BoundingBox.X,
			el.BoundingBox.Y,
			el.BoundingBox.Width,
			el.BoundingBox.Height,
			label,
		)
		defer highlighter.RemoveHighlights()
	}

	// Click at the center of the element using CDP
	centerX := el.BoundingBox.X + el.BoundingBox.Width/2
	centerY := el.BoundingBox.Y + el.BoundingBox.Height/2

	err := proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseMoved,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 0,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to move mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMousePressed,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to press mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseReleased,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to release mouse: %w", err)
	}

	return nil
}

// ClickAt clicks at specific coordinates on the page.
// This is useful as a fallback when element detection fails.
func (b *Browser) ClickAt(ctx context.Context, x, y float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	// Show coordinate highlight before click
	if highlighter := b.getHighlighter(); highlighter != nil {
		label := fmt.Sprintf("click (%d,%d)", int(x), int(y))
		_ = highlighter.HighlightCoordinates(x, y, label)
		defer highlighter.RemoveHighlights()
	}

	// Use CDP to click at coordinates
	err := proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseMoved,
		X:          x,
		Y:          y,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 0,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to move mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMousePressed,
		X:          x,
		Y:          y,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to press mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseReleased,
		X:          x,
		Y:          y,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(page)
	if err != nil {
		return fmt.Errorf("failed to release mouse: %w", err)
	}

	return nil
}

// Type types text into the currently focused element.
func (b *Browser) Type(ctx context.Context, text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	// Use InsertText for text input
	return page.InsertText(text)
}

// TypeInElement clicks on an element and types text into it.
func (b *Browser) TypeInElement(ctx context.Context, elementIndex int, text string) error {
	b.mu.Lock()

	page := b.getActivePageLocked()
	if page == nil {
		b.mu.Unlock()
		return fmt.Errorf("no active page")
	}

	// Get the element map to find the element for highlighting
	elements, err := dom.ExtractElementMap(ctx, page)
	if err != nil {
		b.mu.Unlock()
		return fmt.Errorf("failed to get element map: %w", err)
	}

	el, ok := elements.ByIndex(elementIndex)
	if !ok {
		b.mu.Unlock()
		return fmt.Errorf("element with index %d not found", elementIndex)
	}

	// Show type highlight
	if highlighter := b.getHighlighter(); highlighter != nil {
		_ = highlighter.HighlightType(
			el.BoundingBox.X,
			el.BoundingBox.Y,
			el.BoundingBox.Width,
			el.BoundingBox.Height,
			text,
		)
		defer highlighter.RemoveHighlights()
	}
	b.mu.Unlock()

	// First click to focus
	if err := b.Click(ctx, elementIndex); err != nil {
		return err
	}

	// Small delay to ensure focus
	b.mu.RLock()
	page = b.getActivePageLocked()
	b.mu.RUnlock()
	if page != nil {
		// Wait for stability after click, but don't block on animated pages
		waitForStableWithTimeout(page, 200*time.Millisecond, 2*time.Second)
	}

	// Type the text
	return b.Type(ctx, text)
}

// Scroll scrolls the page by the specified amount.
func (b *Browser) Scroll(ctx context.Context, deltaX, deltaY float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	// Determine scroll direction for highlight
	direction := "down"
	if deltaY < 0 {
		direction = "up"
	} else if deltaX > 0 {
		direction = "right"
	} else if deltaX < 0 {
		direction = "left"
	}

	// Show scroll highlight in center of viewport
	if highlighter := b.getHighlighter(); highlighter != nil {
		// Get viewport dimensions
		viewportWidth := 1280.0
		viewportHeight := 800.0
		if b.config.Viewport != nil {
			viewportWidth = float64(b.config.Viewport.Width)
			viewportHeight = float64(b.config.Viewport.Height)
		}
		_ = highlighter.HighlightScroll(viewportWidth/2, viewportHeight/2, direction)
		defer highlighter.RemoveHighlights()
	}

	return page.Mouse.Scroll(deltaX, deltaY, 1)
}

// ScrollToElement scrolls an element into view.
func (b *Browser) ScrollToElement(ctx context.Context, elementIndex int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	elements, err := dom.ExtractElementMap(ctx, page)
	if err != nil {
		return fmt.Errorf("failed to get element map: %w", err)
	}

	el, ok := elements.ByIndex(elementIndex)
	if !ok {
		return fmt.Errorf("element with index %d not found", elementIndex)
	}

	// Scroll the element into view using JavaScript
	_, err = page.Eval(fmt.Sprintf(
		`document.querySelector('[data-bua-index="%d"]')?.scrollIntoView({behavior: 'smooth', block: 'center'})`,
		el.Index,
	))
	if err != nil {
		// Fall back to coordinate-based scroll
		return page.Mouse.Scroll(0, el.BoundingBox.Y-300, 1)
	}

	return nil
}

// ScrollInElement scrolls within a specific scrollable element (e.g., modal, sidebar).
// This is useful for scrolling within containers that have their own scroll bars,
// like Instagram comment modals, chat windows, or dropdown lists.
func (b *Browser) ScrollInElement(ctx context.Context, elementIndex int, deltaX, deltaY float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	elements, err := dom.ExtractElementMap(ctx, page)
	if err != nil {
		return fmt.Errorf("failed to get element map: %w", err)
	}

	el, ok := elements.ByIndex(elementIndex)
	if !ok {
		return fmt.Errorf("element with index %d not found", elementIndex)
	}

	// Use JavaScript to scroll within the element
	// scrollBy is the most reliable way to scroll within a specific container
	_, err = page.Eval(fmt.Sprintf(
		`(function() {
			const el = document.querySelector('[data-bua-index="%d"]');
			if (!el) return false;
			el.scrollBy({top: %f, left: %f, behavior: 'smooth'});
			return true;
		})()`,
		el.Index, deltaY, deltaX,
	))
	if err != nil {
		return fmt.Errorf("failed to scroll in element: %w", err)
	}

	return nil
}

// FindScrollableModal attempts to auto-detect a scrollable modal/overlay container on the page.
// Returns the element index if found, or -1 if no modal detected.
// This is useful as a fallback when the agent can't identify the scrollable container.
func (b *Browser) FindScrollableModal(ctx context.Context) (int, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return -1, fmt.Errorf("no active page")
	}

	// JavaScript to find the most likely scrollable modal container
	// Checks for: role=dialog, overflow scrollable, fixed/absolute positioned overlays
	result, err := page.Eval(`(function() {
		// Priority 1: Elements with role="dialog" that are scrollable
		const dialogs = document.querySelectorAll('[role="dialog"]');
		for (const el of dialogs) {
			const style = window.getComputedStyle(el);
			if (style.display !== 'none' && style.visibility !== 'hidden') {
				// Check if it or a child is scrollable
				const scrollable = el.querySelector('*');
				if (el.scrollHeight > el.clientHeight || (scrollable && scrollable.scrollHeight > scrollable.clientHeight)) {
					const idx = el.getAttribute('data-bua-index');
					if (idx) return parseInt(idx);
				}
			}
		}

		// Priority 2: Fixed/absolute positioned elements with overflow scroll/auto
		const candidates = [];
		const allElements = document.querySelectorAll('[data-bua-index]');
		for (const el of allElements) {
			const style = window.getComputedStyle(el);
			const position = style.position;
			const overflow = style.overflowY;
			const isScrollable = (overflow === 'auto' || overflow === 'scroll') && el.scrollHeight > el.clientHeight;
			const isOverlay = position === 'fixed' || position === 'absolute';

			if (isScrollable && isOverlay && style.display !== 'none') {
				const idx = parseInt(el.getAttribute('data-bua-index'));
				const rect = el.getBoundingClientRect();
				// Score based on size and visibility
				const score = rect.width * rect.height;
				if (score > 10000) { // Minimum size threshold
					candidates.push({idx, score});
				}
			}
		}

		// Priority 3: Any scrollable container with significant scroll height
		for (const el of allElements) {
			const style = window.getComputedStyle(el);
			const overflow = style.overflowY;
			const isScrollable = (overflow === 'auto' || overflow === 'scroll') && el.scrollHeight > el.clientHeight + 100;

			if (isScrollable && style.display !== 'none') {
				const idx = parseInt(el.getAttribute('data-bua-index'));
				const rect = el.getBoundingClientRect();
				// Must be reasonably sized and visible
				if (rect.width > 200 && rect.height > 200 && rect.top >= 0 && rect.left >= 0) {
					const score = (el.scrollHeight - el.clientHeight) * rect.width; // Prefer more scrollable content
					candidates.push({idx, score: score * 0.5}); // Lower priority than overlays
				}
			}
		}

		// Return the highest scoring candidate
		if (candidates.length > 0) {
			candidates.sort((a, b) => b.score - a.score);
			return candidates[0].idx;
		}

		return -1;
	})()`)

	if err != nil {
		return -1, fmt.Errorf("failed to detect scrollable modal: %w", err)
	}

	// result.Value is a gjson.Result - use Int() which returns int64
	idx := int(result.Value.Int())
	return idx, nil
}

// ScrollInModalAuto attempts to scroll in an auto-detected modal container.
// If no modal is found, falls back to page scroll.
// Returns the element index used for scrolling (-1 if page scroll).
func (b *Browser) ScrollInModalAuto(ctx context.Context, deltaX, deltaY float64) (int, error) {
	modalIdx, err := b.FindScrollableModal(ctx)
	if err != nil {
		// Fall back to page scroll
		return -1, b.Scroll(ctx, deltaX, deltaY)
	}

	if modalIdx >= 0 {
		// Found a modal, scroll within it
		err = b.ScrollInElement(ctx, modalIdx, deltaX, deltaY)
		return modalIdx, err
	}

	// No modal found, scroll the page
	return -1, b.Scroll(ctx, deltaX, deltaY)
}

// WaitForNavigation waits for a navigation to complete.
func (b *Browser) WaitForNavigation(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	return page.WaitLoad()
}

// WaitForStable waits for the page to become stable (no more changes).
func (b *Browser) WaitForStable(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return fmt.Errorf("no active page")
	}

	return page.WaitStable(300)
}

// GetURL returns the current page URL.
func (b *Browser) GetURL() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return ""
	}

	info, err := page.Info()
	if err != nil {
		return ""
	}
	return info.URL
}

// GetTitle returns the current page title.
func (b *Browser) GetTitle() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	page := b.getActivePageLocked()
	if page == nil {
		return ""
	}

	info, err := page.Info()
	if err != nil {
		return ""
	}
	return info.Title
}

// Page returns the underlying rod.Page for advanced operations.
// Deprecated: Use GetActiveTabID() and multi-tab methods instead.
func (b *Browser) Page() *rod.Page {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.getActivePageLocked()
}

// GetActiveTabID returns the ID of the currently active tab.
func (b *Browser) GetActiveTabID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.activeTabID
}

// intPtr returns a pointer to an int value.
func intPtr(v int) *int {
	return &v
}

// Close closes the browser and all tabs.
func (b *Browser) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Close all tabs
	for tabID, page := range b.pages {
		if page != nil {
			page.Close()
		}
		delete(b.pages, tabID)
	}
	b.activeTabID = ""

	// Legacy cleanup
	if b.page != nil {
		b.page.Close()
		b.page = nil
	}

	if b.rod != nil {
		err := b.rod.Close()
		b.rod = nil
		return err
	}

	return nil
}

// ========================================
// Multi-Tab Management Methods
// ========================================

// NewTab opens a new browser tab with the specified URL.
// Returns the tab ID for later reference.
func (b *Browser) NewTab(ctx context.Context, url string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	tabID, err := b.createTabLocked(url)
	if err != nil {
		return "", err
	}

	// Wait for page to load
	page := b.pages[tabID]
	if err := page.WaitLoad(); err != nil {
		return tabID, fmt.Errorf("page load failed: %w", err)
	}
	// Wait for stability with timeout to avoid blocking on animated/video pages
	waitForStableWithTimeout(page, 300*time.Millisecond, 5*time.Second)

	return tabID, nil
}

// SwitchTab switches to a different browser tab by its ID.
func (b *Browser) SwitchTab(ctx context.Context, tabID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page, ok := b.pages[tabID]
	if !ok {
		return fmt.Errorf("tab %s not found", tabID)
	}

	b.activeTabID = tabID
	b.page = page // maintain backward compatibility

	// Bring the tab to front
	page.MustActivate()

	return nil
}

// CloseTab closes a browser tab by its ID.
// Cannot close the last remaining tab.
func (b *Browser) CloseTab(ctx context.Context, tabID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	page, ok := b.pages[tabID]
	if !ok {
		return fmt.Errorf("tab %s not found", tabID)
	}

	// Don't allow closing the last tab
	if len(b.pages) <= 1 {
		return fmt.Errorf("cannot close the last tab")
	}

	// Close the page
	page.Close()
	delete(b.pages, tabID)

	// If we closed the active tab, switch to another
	if b.activeTabID == tabID {
		for newTabID, newPage := range b.pages {
			b.activeTabID = newTabID
			b.page = newPage
			newPage.MustActivate()
			break
		}
	}

	return nil
}

// ListTabs returns information about all open tabs.
func (b *Browser) ListTabs(ctx context.Context) []TabInfo {
	b.mu.RLock()
	defer b.mu.RUnlock()

	var tabs []TabInfo
	for tabID, page := range b.pages {
		info, err := page.Info()
		if err != nil {
			continue
		}
		tabs = append(tabs, TabInfo{
			ID:    tabID,
			URL:   info.URL,
			Title: info.Title,
		})
	}
	return tabs
}
