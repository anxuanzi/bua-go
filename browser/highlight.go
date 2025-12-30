// Package browser provides the browser automation layer using go-rod.
package browser

import (
	"fmt"
	"time"

	"github.com/go-rod/rod"
)

// Highlighter provides visual feedback for browser automation actions.
// It injects CSS and HTML elements to show animated highlights on elements
// being interacted with, similar to Python browser-use.
type Highlighter struct {
	page    *rod.Page
	enabled bool
	delay   time.Duration // How long to show highlight before action
}

// NewHighlighter creates a new highlighter for the given page.
func NewHighlighter(page *rod.Page, enabled bool) *Highlighter {
	return &Highlighter{
		page:    page,
		enabled: enabled,
		delay:   300 * time.Millisecond, // Default 300ms visual feedback
	}
}

// SetDelay sets how long the highlight is shown before action execution.
func (h *Highlighter) SetDelay(d time.Duration) {
	h.delay = d
}

// SetEnabled enables or disables highlighting.
func (h *Highlighter) SetEnabled(enabled bool) {
	h.enabled = enabled
}

// injectStyles injects the CSS for highlight animations if not already present.
func (h *Highlighter) injectStyles() error {
	_, err := h.page.Eval(`(function() {
		if (document.getElementById('bua-highlight-styles')) return;

		const style = document.createElement('style');
		style.id = 'bua-highlight-styles';
		style.textContent = ` + "`" + `
			.bua-highlight-corner {
				position: fixed;
				pointer-events: none;
				z-index: 999999;
				transition: all 0.15s ease-out;
			}
			.bua-highlight-corner-tl { border-top: 3px solid #ff6b35; border-left: 3px solid #ff6b35; }
			.bua-highlight-corner-tr { border-top: 3px solid #ff6b35; border-right: 3px solid #ff6b35; }
			.bua-highlight-corner-bl { border-bottom: 3px solid #ff6b35; border-left: 3px solid #ff6b35; }
			.bua-highlight-corner-br { border-bottom: 3px solid #ff6b35; border-right: 3px solid #ff6b35; }

			.bua-highlight-crosshair {
				position: fixed;
				pointer-events: none;
				z-index: 999999;
			}
			.bua-highlight-crosshair-h {
				width: 40px;
				height: 2px;
				background: #ff6b35;
				transform: translateX(-50%);
			}
			.bua-highlight-crosshair-v {
				width: 2px;
				height: 40px;
				background: #ff6b35;
				transform: translateY(-50%);
			}
			.bua-highlight-circle {
				position: fixed;
				pointer-events: none;
				z-index: 999998;
				border: 2px solid #ff6b35;
				border-radius: 50%;
				animation: bua-pulse 0.4s ease-out;
			}
			@keyframes bua-pulse {
				0% { transform: translate(-50%, -50%) scale(0.5); opacity: 1; }
				100% { transform: translate(-50%, -50%) scale(1.5); opacity: 0; }
			}

			.bua-highlight-label {
				position: fixed;
				pointer-events: none;
				z-index: 999999;
				background: #ff6b35;
				color: white;
				padding: 2px 6px;
				font-size: 11px;
				font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
				font-weight: 500;
				border-radius: 3px;
				white-space: nowrap;
			}
		` + "`" + `;
		document.head.appendChild(style);
	})()`)
	return err
}

// HighlightElement shows animated corner brackets around an element.
// x, y are the top-left corner; width, height are the element dimensions.
func (h *Highlighter) HighlightElement(x, y, width, height float64, label string) error {
	if !h.enabled || h.page == nil {
		return nil
	}

	if err := h.injectStyles(); err != nil {
		return err
	}

	cornerSize := 20.0 // Length of corner brackets

	js := fmt.Sprintf(`(function() {
		// Remove any existing highlights
		document.querySelectorAll('.bua-highlight-corner, .bua-highlight-label').forEach(el => el.remove());

		const x = %f;
		const y = %f;
		const w = %f;
		const h = %f;
		const cornerSize = %f;
		const label = %q;
		const padding = 4; // Padding around element

		// Create corner elements
		const corners = [
			{cls: 'bua-highlight-corner-tl', left: x - padding, top: y - padding, w: cornerSize, h: cornerSize},
			{cls: 'bua-highlight-corner-tr', left: x + w + padding - cornerSize, top: y - padding, w: cornerSize, h: cornerSize},
			{cls: 'bua-highlight-corner-bl', left: x - padding, top: y + h + padding - cornerSize, w: cornerSize, h: cornerSize},
			{cls: 'bua-highlight-corner-br', left: x + w + padding - cornerSize, top: y + h + padding - cornerSize, w: cornerSize, h: cornerSize},
		];

		corners.forEach(c => {
			const el = document.createElement('div');
			el.className = 'bua-highlight-corner ' + c.cls;
			el.style.left = c.left + 'px';
			el.style.top = c.top + 'px';
			el.style.width = c.w + 'px';
			el.style.height = c.h + 'px';
			document.body.appendChild(el);
		});

		// Add label if provided
		if (label) {
			const labelEl = document.createElement('div');
			labelEl.className = 'bua-highlight-label';
			labelEl.textContent = label;
			labelEl.style.left = (x - padding) + 'px';
			labelEl.style.top = (y - padding - 22) + 'px';
			document.body.appendChild(labelEl);
		}
	})()`, x, y, width, height, cornerSize, label)

	_, err := h.page.Eval(js)
	if err != nil {
		return fmt.Errorf("failed to show element highlight: %w", err)
	}

	// Wait for visual feedback
	time.Sleep(h.delay)
	return nil
}

// HighlightCoordinates shows a crosshair and expanding circle at the click position.
func (h *Highlighter) HighlightCoordinates(x, y float64, label string) error {
	if !h.enabled || h.page == nil {
		return nil
	}

	if err := h.injectStyles(); err != nil {
		return err
	}

	js := fmt.Sprintf(`(function() {
		// Remove any existing highlights
		document.querySelectorAll('.bua-highlight-crosshair, .bua-highlight-crosshair-h, .bua-highlight-crosshair-v, .bua-highlight-circle, .bua-highlight-label').forEach(el => el.remove());

		const x = %f;
		const y = %f;
		const label = %q;

		// Horizontal crosshair
		const hLine = document.createElement('div');
		hLine.className = 'bua-highlight-crosshair bua-highlight-crosshair-h';
		hLine.style.left = x + 'px';
		hLine.style.top = y + 'px';
		document.body.appendChild(hLine);

		// Vertical crosshair
		const vLine = document.createElement('div');
		vLine.className = 'bua-highlight-crosshair bua-highlight-crosshair-v';
		vLine.style.left = x + 'px';
		vLine.style.top = y + 'px';
		document.body.appendChild(vLine);

		// Expanding circle
		const circle = document.createElement('div');
		circle.className = 'bua-highlight-circle';
		circle.style.left = x + 'px';
		circle.style.top = y + 'px';
		circle.style.width = '30px';
		circle.style.height = '30px';
		document.body.appendChild(circle);

		// Add label if provided
		if (label) {
			const labelEl = document.createElement('div');
			labelEl.className = 'bua-highlight-label';
			labelEl.textContent = label;
			labelEl.style.left = (x + 15) + 'px';
			labelEl.style.top = (y - 25) + 'px';
			document.body.appendChild(labelEl);
		}
	})()`, x, y, label)

	_, err := h.page.Eval(js)
	if err != nil {
		return fmt.Errorf("failed to show coordinate highlight: %w", err)
	}

	// Wait for visual feedback
	time.Sleep(h.delay)
	return nil
}

// HighlightScroll shows a scroll indicator on the page or element.
func (h *Highlighter) HighlightScroll(x, y float64, direction string) error {
	if !h.enabled || h.page == nil {
		return nil
	}

	if err := h.injectStyles(); err != nil {
		return err
	}

	arrow := "↓"
	switch direction {
	case "up":
		arrow = "↑"
	case "left":
		arrow = "←"
	case "right":
		arrow = "→"
	}

	js := fmt.Sprintf(`(function() {
		// Remove any existing scroll indicators
		document.querySelectorAll('.bua-highlight-label').forEach(el => el.remove());

		const x = %f;
		const y = %f;
		const arrow = %q;

		const labelEl = document.createElement('div');
		labelEl.className = 'bua-highlight-label';
		labelEl.textContent = 'Scroll ' + arrow;
		labelEl.style.left = x + 'px';
		labelEl.style.top = y + 'px';
		labelEl.style.fontSize = '14px';
		document.body.appendChild(labelEl);
	})()`, x, y, arrow)

	_, err := h.page.Eval(js)
	if err != nil {
		return fmt.Errorf("failed to show scroll highlight: %w", err)
	}

	// Shorter delay for scroll
	time.Sleep(h.delay / 2)
	return nil
}

// HighlightType shows a typing indicator on an input element.
func (h *Highlighter) HighlightType(x, y, width, height float64, text string) error {
	if !h.enabled || h.page == nil {
		return nil
	}

	// Show corner brackets with "typing..." label
	label := "typing..."
	if len(text) > 20 {
		label = fmt.Sprintf("typing: %s...", text[:20])
	} else if len(text) > 0 {
		label = fmt.Sprintf("typing: %s", text)
	}

	return h.HighlightElement(x, y, width, height, label)
}

// RemoveHighlights removes all highlight elements from the page.
func (h *Highlighter) RemoveHighlights() error {
	if h.page == nil {
		return nil
	}

	_, err := h.page.Eval(`(function() {
		document.querySelectorAll('.bua-highlight-corner, .bua-highlight-crosshair, .bua-highlight-crosshair-h, .bua-highlight-crosshair-v, .bua-highlight-circle, .bua-highlight-label').forEach(el => el.remove());
	})()`)
	return err
}
