// Package browser provides the browser automation layer using go-rod.
package browser

import (
	"context"
	"fmt"

	"github.com/anxuanzi/bua-go/dom"
)

// AnnotationConfig holds configuration for element annotations.
type AnnotationConfig struct {
	// ShowIndex displays the element index number
	ShowIndex bool
	// ShowType displays the element type (button, input, link, etc.)
	ShowType bool
	// ShowBoundingBox draws a border around elements
	ShowBoundingBox bool
	// Opacity of the overlay (0.0 - 1.0)
	Opacity float64
}

// DefaultAnnotationConfig returns the default annotation configuration.
func DefaultAnnotationConfig() *AnnotationConfig {
	return &AnnotationConfig{
		ShowIndex:       true,
		ShowType:        true,
		ShowBoundingBox: true,
		Opacity:         0.8,
	}
}

// annotationCSS returns the CSS for element annotations.
func annotationCSS(opacity float64) string {
	return fmt.Sprintf(`
		.bua-annotation-overlay {
			position: fixed;
			pointer-events: none;
			z-index: 2147483647;
			top: 0;
			left: 0;
			width: 100%%;
			height: 100%%;
		}
		.bua-element-box {
			position: absolute;
			border: 2px solid;
			box-sizing: border-box;
			pointer-events: none;
		}
		.bua-element-label {
			position: absolute;
			font-family: 'SF Mono', 'Monaco', 'Inconsolata', 'Fira Code', monospace;
			font-size: 10px;
			font-weight: bold;
			padding: 2px 4px;
			border-radius: 3px;
			white-space: nowrap;
			opacity: %.2f;
			pointer-events: none;
		}
		/* Element type colors */
		.bua-type-button { border-color: #e74c3c; }
		.bua-type-button .bua-element-label { background: #e74c3c; color: white; }

		.bua-type-link { border-color: #3498db; }
		.bua-type-link .bua-element-label { background: #3498db; color: white; }

		.bua-type-input { border-color: #2ecc71; }
		.bua-type-input .bua-element-label { background: #2ecc71; color: white; }

		.bua-type-select { border-color: #9b59b6; }
		.bua-type-select .bua-element-label { background: #9b59b6; color: white; }

		.bua-type-textarea { border-color: #1abc9c; }
		.bua-type-textarea .bua-element-label { background: #1abc9c; color: white; }

		.bua-type-image { border-color: #f39c12; }
		.bua-type-image .bua-element-label { background: #f39c12; color: white; }

		.bua-type-other { border-color: #95a5a6; }
		.bua-type-other .bua-element-label { background: #95a5a6; color: white; }
	`, opacity)
}

// getElementTypeClass returns the CSS class for an element type.
func getElementTypeClass(tagName string, el *dom.Element) string {
	switch tagName {
	case "button":
		return "bua-type-button"
	case "a":
		return "bua-type-link"
	case "input":
		if el != nil {
			switch el.Type {
			case "submit", "button":
				return "bua-type-button"
			default:
				return "bua-type-input"
			}
		}
		return "bua-type-input"
	case "select":
		return "bua-type-select"
	case "textarea":
		return "bua-type-textarea"
	case "img":
		return "bua-type-image"
	default:
		// Check if it's clickable (has onclick or role=button)
		if el != nil && el.Role == "button" {
			return "bua-type-button"
		}
		return "bua-type-other"
	}
}

// ShowAnnotations draws annotation overlays on all detected elements.
func (b *Browser) ShowAnnotations(ctx context.Context, elements *dom.ElementMap, cfg *AnnotationConfig) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	if cfg == nil {
		cfg = DefaultAnnotationConfig()
	}

	// First, clear any existing annotations
	_, err := b.page.Eval(`() => {
		const existing = document.getElementById('bua-annotation-container');
		if (existing) existing.remove();
	}`)
	if err != nil {
		return fmt.Errorf("failed to clear existing annotations: %w", err)
	}

	// Inject CSS
	css := annotationCSS(cfg.Opacity)
	_, err = b.page.Eval(fmt.Sprintf(`() => {
		let style = document.getElementById('bua-annotation-style');
		if (!style) {
			style = document.createElement('style');
			style.id = 'bua-annotation-style';
			document.head.appendChild(style);
		}
		style.textContent = %q;
	}`, css))
	if err != nil {
		return fmt.Errorf("failed to inject CSS: %w", err)
	}

	// Create overlay container
	_, err = b.page.Eval(`() => {
		const container = document.createElement('div');
		container.id = 'bua-annotation-container';
		container.className = 'bua-annotation-overlay';
		document.body.appendChild(container);
	}`)
	if err != nil {
		return fmt.Errorf("failed to create overlay container: %w", err)
	}

	// Add element boxes
	for _, el := range elements.InteractiveElements() {
		if el.BoundingBox.Width <= 0 || el.BoundingBox.Height <= 0 {
			continue
		}

		typeClass := getElementTypeClass(el.TagName, el)

		labelText := ""
		if cfg.ShowIndex {
			labelText = fmt.Sprintf("%d", el.Index)
		}
		if cfg.ShowType && el.TagName != "" {
			if labelText != "" {
				labelText += " "
			}
			labelText += el.TagName
		}

		js := fmt.Sprintf(`() => {
			const container = document.getElementById('bua-annotation-container');
			if (!container) return;

			const box = document.createElement('div');
			box.className = 'bua-element-box %s';
			box.style.left = '%fpx';
			box.style.top = '%fpx';
			box.style.width = '%fpx';
			box.style.height = '%fpx';

			const label = document.createElement('div');
			label.className = 'bua-element-label';
			label.textContent = '%s';
			label.style.left = '0';
			label.style.top = '-18px';

			box.appendChild(label);
			container.appendChild(box);
		}`,
			typeClass,
			el.BoundingBox.X,
			el.BoundingBox.Y,
			el.BoundingBox.Width,
			el.BoundingBox.Height,
			labelText,
		)

		_, err = b.page.Eval(js)
		if err != nil {
			// Continue with other elements even if one fails
			continue
		}
	}

	return nil
}

// HideAnnotations removes all annotation overlays from the page.
func (b *Browser) HideAnnotations(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.page == nil {
		return fmt.Errorf("no active page")
	}

	_, err := b.page.Eval(`() => {
		const container = document.getElementById('bua-annotation-container');
		if (container) container.remove();
		const style = document.getElementById('bua-annotation-style');
		if (style) style.remove();
	}`)
	if err != nil {
		return fmt.Errorf("failed to remove annotations: %w", err)
	}

	return nil
}

// ToggleAnnotations shows or hides annotations based on current state.
func (b *Browser) ToggleAnnotations(ctx context.Context, elements *dom.ElementMap, cfg *AnnotationConfig) (bool, error) {
	b.mu.RLock()
	if b.page == nil {
		b.mu.RUnlock()
		return false, fmt.Errorf("no active page")
	}
	b.mu.RUnlock()

	// Check if annotations currently exist
	result, err := b.page.Eval(`() => {
		return document.getElementById('bua-annotation-container') !== null;
	}`)
	if err != nil {
		return false, fmt.Errorf("failed to check annotation state: %w", err)
	}

	hasAnnotations := result.Value.Bool()

	if hasAnnotations {
		err = b.HideAnnotations(ctx)
		return false, err
	}

	err = b.ShowAnnotations(ctx, elements, cfg)
	return true, err
}
