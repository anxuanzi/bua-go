// Package browser provides the browser automation layer using go-rod.
package browser

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"

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

// Browser wraps a rod browser for controlled automation.
type Browser struct {
	rod      *rod.Browser
	page     *rod.Page
	config   Config
	screener *screenshot.Manager

	mu sync.RWMutex
}

// New creates a new browser wrapper.
func New(rodBrowser *rod.Browser, cfg Config) *Browser {
	b := &Browser{
		rod:    rodBrowser,
		config: cfg,
	}

	if cfg.ScreenshotConfig != nil {
		b.screener = screenshot.NewManager(cfg.ScreenshotConfig)
	}

	return b
}

// Navigate navigates to the specified URL.
func (b *Browser) Navigate(ctx context.Context, url string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Create page if needed
	if b.page == nil {
		// Create a blank page first
		page, err := b.rod.Page(proto.TargetCreateTarget{URL: "about:blank"})
		if err != nil {
			return fmt.Errorf("failed to create page: %w", err)
		}
		b.page = page

		// Set viewport to match window size for proper responsive behavior
		// Key: Window size (set in launcher) and viewport must match
		if b.config.Viewport != nil {
			err := b.page.SetViewport(&proto.EmulationSetDeviceMetricsOverride{
				Width:             b.config.Viewport.Width,
				Height:            b.config.Viewport.Height,
				DeviceScaleFactor: 1.0,
				Mobile:            false,
			})
			if err != nil {
				return fmt.Errorf("failed to set viewport: %w", err)
			}
		}

		// Navigate to the URL
		err = b.page.Navigate(url)
		if err != nil {
			return fmt.Errorf("failed to navigate: %w", err)
		}
	} else {
		// Navigate existing page
		err := b.page.Navigate(url)
		if err != nil {
			return fmt.Errorf("failed to navigate: %w", err)
		}
	}

	// Wait for page to be ready
	err := b.page.WaitLoad()
	if err != nil {
		return fmt.Errorf("failed to wait for page load: %w", err)
	}

	// Wait for page to stabilize (animations, lazy loading, etc.)
	b.page.MustWaitStable()

	return nil
}

// Screenshot takes a screenshot of the current page.
func (b *Browser) Screenshot(ctx context.Context) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return nil, fmt.Errorf("no active page")
	}

	data, err := b.page.Screenshot(true, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to take screenshot: %w", err)
	}

	return data, nil
}

// ScreenshotWithAnnotations takes an annotated screenshot with element indices.
func (b *Browser) ScreenshotWithAnnotations(ctx context.Context, elements *dom.ElementMap) ([]byte, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return nil, fmt.Errorf("no active page")
	}

	// Take raw screenshot
	data, err := b.page.Screenshot(true, nil)
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

// GetElementMap extracts the element map from the current page.
func (b *Browser) GetElementMap(ctx context.Context) (*dom.ElementMap, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return nil, fmt.Errorf("no active page")
	}

	return dom.ExtractElementMap(ctx, b.page)
}

// GetAccessibilityTree extracts the accessibility tree from the current page.
func (b *Browser) GetAccessibilityTree(ctx context.Context) (*dom.AccessibilityTree, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return nil, fmt.Errorf("no active page")
	}

	return dom.ExtractAccessibilityTree(ctx, b.page)
}

// Click clicks on an element by its index in the element map.
func (b *Browser) Click(ctx context.Context, elementIndex int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	// Get the element map to find the element
	elements, err := dom.ExtractElementMap(ctx, b.page)
	if err != nil {
		return fmt.Errorf("failed to get element map: %w", err)
	}

	el, ok := elements.ByIndex(elementIndex)
	if !ok {
		return fmt.Errorf("element with index %d not found", elementIndex)
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
	}.Call(b.page)
	if err != nil {
		return fmt.Errorf("failed to move mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMousePressed,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(b.page)
	if err != nil {
		return fmt.Errorf("failed to press mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseReleased,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(b.page)
	if err != nil {
		return fmt.Errorf("failed to release mouse: %w", err)
	}

	return nil
}

// ClickElement clicks on an element directly.
func (b *Browser) ClickElement(ctx context.Context, el *dom.Element) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
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
	}.Call(b.page)
	if err != nil {
		return fmt.Errorf("failed to move mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMousePressed,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(b.page)
	if err != nil {
		return fmt.Errorf("failed to press mouse: %w", err)
	}

	err = proto.InputDispatchMouseEvent{
		Type:       proto.InputDispatchMouseEventTypeMouseReleased,
		X:          centerX,
		Y:          centerY,
		Button:     proto.InputMouseButtonLeft,
		ClickCount: 1,
	}.Call(b.page)
	if err != nil {
		return fmt.Errorf("failed to release mouse: %w", err)
	}

	return nil
}

// Type types text into the currently focused element.
func (b *Browser) Type(ctx context.Context, text string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	// Use InsertText for text input
	return b.page.InsertText(text)
}

// TypeInElement clicks on an element and types text into it.
func (b *Browser) TypeInElement(ctx context.Context, elementIndex int, text string) error {
	// First click to focus
	if err := b.Click(ctx, elementIndex); err != nil {
		return err
	}

	// Small delay to ensure focus
	b.page.MustWaitStable()

	// Type the text
	return b.Type(ctx, text)
}

// Scroll scrolls the page by the specified amount.
func (b *Browser) Scroll(ctx context.Context, deltaX, deltaY float64) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	return b.page.Mouse.Scroll(deltaX, deltaY, 1)
}

// ScrollToElement scrolls an element into view.
func (b *Browser) ScrollToElement(ctx context.Context, elementIndex int) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	elements, err := dom.ExtractElementMap(ctx, b.page)
	if err != nil {
		return fmt.Errorf("failed to get element map: %w", err)
	}

	el, ok := elements.ByIndex(elementIndex)
	if !ok {
		return fmt.Errorf("element with index %d not found", elementIndex)
	}

	// Scroll the element into view using JavaScript
	_, err = b.page.Eval(fmt.Sprintf(
		`document.querySelector('[data-bua-index="%d"]')?.scrollIntoView({behavior: 'smooth', block: 'center'})`,
		el.Index,
	))
	if err != nil {
		// Fall back to coordinate-based scroll
		return b.page.Mouse.Scroll(0, el.BoundingBox.Y-300, 1)
	}

	return nil
}

// WaitForNavigation waits for a navigation to complete.
func (b *Browser) WaitForNavigation(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	return b.page.WaitLoad()
}

// WaitForStable waits for the page to become stable (no more changes).
func (b *Browser) WaitForStable(ctx context.Context) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	return b.page.WaitStable(300)
}

// GetURL returns the current page URL.
func (b *Browser) GetURL() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return ""
	}

	info, err := b.page.Info()
	if err != nil {
		return ""
	}
	return info.URL
}

// GetTitle returns the current page title.
func (b *Browser) GetTitle() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.page == nil {
		return ""
	}

	info, err := b.page.Info()
	if err != nil {
		return ""
	}
	return info.Title
}

// Page returns the underlying rod.Page for advanced operations.
func (b *Browser) Page() *rod.Page {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.page
}

// intPtr returns a pointer to an int value.
func intPtr(v int) *int {
	return &v
}

// Close closes the browser.
func (b *Browser) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

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
