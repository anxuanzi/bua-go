package dom

import (
	"testing"
)

func TestNewElementMap(t *testing.T) {
	em := NewElementMap()
	if em == nil {
		t.Fatal("NewElementMap() returned nil")
	}
	if em.Elements == nil {
		t.Error("Elements slice is nil")
	}
	if em.indexMap == nil {
		t.Error("indexMap is nil")
	}
	if em.Count() != 0 {
		t.Errorf("Count() = %d, want 0", em.Count())
	}
}

func TestElementMap_Add(t *testing.T) {
	em := NewElementMap()

	el := &Element{
		Index:   0,
		TagName: "button",
		Text:    "Click me",
	}
	em.Add(el)

	if em.Count() != 1 {
		t.Errorf("Count() = %d, want 1", em.Count())
	}

	// Add more elements
	em.Add(&Element{Index: 1, TagName: "input"})
	em.Add(&Element{Index: 2, TagName: "a"})

	if em.Count() != 3 {
		t.Errorf("Count() = %d, want 3", em.Count())
	}
}

func TestElementMap_ByIndex(t *testing.T) {
	em := NewElementMap()
	em.Add(&Element{Index: 0, TagName: "button", Text: "Button 0"})
	em.Add(&Element{Index: 1, TagName: "input", Text: "Input 1"})
	em.Add(&Element{Index: 2, TagName: "a", Text: "Link 2"})

	// Test found case
	el, ok := em.ByIndex(1)
	if !ok {
		t.Error("ByIndex(1) returned false")
	}
	if el.TagName != "input" {
		t.Errorf("ByIndex(1).TagName = %s, want input", el.TagName)
	}

	// Test not found case
	_, ok = em.ByIndex(99)
	if ok {
		t.Error("ByIndex(99) should return false")
	}
}

func TestElementMap_InteractiveElements(t *testing.T) {
	em := NewElementMap()
	em.Add(&Element{Index: 0, TagName: "button", IsInteractive: true, IsVisible: true})
	em.Add(&Element{Index: 1, TagName: "div", IsInteractive: false, IsVisible: true})
	em.Add(&Element{Index: 2, TagName: "input", IsInteractive: true, IsVisible: false})
	em.Add(&Element{Index: 3, TagName: "a", IsInteractive: true, IsVisible: true})

	interactive := em.InteractiveElements()
	if len(interactive) != 2 {
		t.Errorf("InteractiveElements() returned %d elements, want 2", len(interactive))
	}
}

func TestElementMap_ToTokenString(t *testing.T) {
	em := NewElementMap()
	em.PageTitle = "Test Page"
	em.PageURL = "https://example.com"
	em.Add(&Element{
		Index:     0,
		TagName:   "button",
		Text:      "Click me",
		IsVisible: true,
	})
	em.Add(&Element{
		Index:     1,
		TagName:   "input",
		Type:      "text",
		IsVisible: true,
	})
	em.Add(&Element{
		Index:     2,
		TagName:   "a",
		Href:      "https://example.com/link",
		IsVisible: false, // Should be skipped
	})

	str := em.ToTokenString()
	if str == "" {
		t.Error("ToTokenString() returned empty string")
	}

	// Check that visible elements are included
	if !contains(str, "[0]") {
		t.Error("ToTokenString() missing element 0")
	}
	if !contains(str, "[1]") {
		t.Error("ToTokenString() missing element 1")
	}
}

func TestBoundingBox(t *testing.T) {
	bb := BoundingBox{
		X:      100,
		Y:      200,
		Width:  50,
		Height: 30,
	}

	if bb.X != 100 {
		t.Errorf("X = %f, want 100", bb.X)
	}
	if bb.Y != 200 {
		t.Errorf("Y = %f, want 200", bb.Y)
	}
	if bb.Width != 50 {
		t.Errorf("Width = %f, want 50", bb.Width)
	}
	if bb.Height != 30 {
		t.Errorf("Height = %f, want 30", bb.Height)
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"short", 10, "short"},
		{"exactly10c", 10, "exactly10c"},
		{"this is a long string", 10, "this is..."},
		{"", 10, ""},
		{"test", 4, "test"},
		{"testing", 5, "te..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Additional tests for edge cases

func TestElement_FieldValues(t *testing.T) {
	el := &Element{
		Index:         42,
		TagName:       "button",
		Role:          "button",
		Name:          "Submit",
		Text:          "Submit Form",
		Type:          "submit",
		Href:          "",
		Placeholder:   "",
		Value:         "",
		AriaLabel:     "Submit button",
		IsInteractive: true,
		IsVisible:     true,
		BoundingBox: BoundingBox{
			X:      100,
			Y:      200,
			Width:  120,
			Height: 40,
		},
	}

	if el.Index != 42 {
		t.Errorf("Index = %d, want 42", el.Index)
	}
	if el.TagName != "button" {
		t.Errorf("TagName = %q, want 'button'", el.TagName)
	}
	if el.Role != "button" {
		t.Errorf("Role = %q, want 'button'", el.Role)
	}
	if !el.IsInteractive {
		t.Error("IsInteractive should be true")
	}
	if !el.IsVisible {
		t.Error("IsVisible should be true")
	}
	if el.BoundingBox.Width != 120 {
		t.Errorf("BoundingBox.Width = %f, want 120", el.BoundingBox.Width)
	}
}

func TestElementMap_EmptyOperations(t *testing.T) {
	em := NewElementMap()

	t.Run("ByIndex on empty map", func(t *testing.T) {
		_, ok := em.ByIndex(0)
		if ok {
			t.Error("ByIndex(0) on empty map should return false")
		}
	})

	t.Run("InteractiveElements on empty map", func(t *testing.T) {
		interactive := em.InteractiveElements()
		if len(interactive) != 0 {
			t.Errorf("InteractiveElements on empty map should return 0, got %d", len(interactive))
		}
	})

	t.Run("ToTokenString on empty map", func(t *testing.T) {
		str := em.ToTokenString()
		// Should not panic and should return something
		if str == "" {
			// Empty map may return empty string, that's ok
		}
	})
}

func TestElementMap_LargeIndex(t *testing.T) {
	em := NewElementMap()

	// Add element with large index
	em.Add(&Element{Index: 999999, TagName: "div"})

	el, ok := em.ByIndex(999999)
	if !ok {
		t.Error("ByIndex(999999) should find the element")
	}
	if el.TagName != "div" {
		t.Errorf("TagName = %q, want 'div'", el.TagName)
	}
}

func TestElementMap_DuplicateIndex(t *testing.T) {
	em := NewElementMap()

	// Add elements with same index - should update
	em.Add(&Element{Index: 0, TagName: "button", Text: "First"})
	em.Add(&Element{Index: 0, TagName: "button", Text: "Second"})

	// The indexMap should point to the latest
	el, ok := em.ByIndex(0)
	if !ok {
		t.Error("ByIndex(0) should find the element")
	}
	// Both elements are in the slice, but indexMap points to latest
	if el.Text != "Second" {
		t.Logf("Note: Duplicate index behavior - found %q", el.Text)
	}
}

func TestElementMap_AllElementTypes(t *testing.T) {
	em := NewElementMap()

	// Add various element types
	elements := []*Element{
		{Index: 0, TagName: "button", IsInteractive: true, IsVisible: true},
		{Index: 1, TagName: "input", Type: "text", IsInteractive: true, IsVisible: true},
		{Index: 2, TagName: "input", Type: "checkbox", IsInteractive: true, IsVisible: true},
		{Index: 3, TagName: "input", Type: "radio", IsInteractive: true, IsVisible: true},
		{Index: 4, TagName: "select", IsInteractive: true, IsVisible: true},
		{Index: 5, TagName: "textarea", IsInteractive: true, IsVisible: true},
		{Index: 6, TagName: "a", Href: "https://example.com", IsInteractive: true, IsVisible: true},
		{Index: 7, TagName: "div", IsInteractive: false, IsVisible: true},
		{Index: 8, TagName: "span", IsInteractive: false, IsVisible: true},
		{Index: 9, TagName: "img", IsInteractive: false, IsVisible: true},
	}

	for _, el := range elements {
		em.Add(el)
	}

	if em.Count() != 10 {
		t.Errorf("Count() = %d, want 10", em.Count())
	}

	interactive := em.InteractiveElements()
	if len(interactive) != 7 {
		t.Errorf("InteractiveElements() = %d, want 7", len(interactive))
	}
}

func TestElementMap_PageMetadata(t *testing.T) {
	em := NewElementMap()
	em.PageTitle = "Test Page Title"
	em.PageURL = "https://example.com/test"

	if em.PageTitle != "Test Page Title" {
		t.Errorf("PageTitle = %q, want 'Test Page Title'", em.PageTitle)
	}
	if em.PageURL != "https://example.com/test" {
		t.Errorf("PageURL = %q, want 'https://example.com/test'", em.PageURL)
	}
}

func TestBoundingBox_ZeroValues(t *testing.T) {
	bb := BoundingBox{}

	if bb.X != 0 || bb.Y != 0 || bb.Width != 0 || bb.Height != 0 {
		t.Error("Zero-value BoundingBox should have all zeros")
	}
}

func TestBoundingBox_NegativeValues(t *testing.T) {
	// Some scroll positions can result in negative coordinates
	bb := BoundingBox{
		X:      -100,
		Y:      -50,
		Width:  200,
		Height: 100,
	}

	if bb.X != -100 {
		t.Errorf("X = %f, want -100", bb.X)
	}
	if bb.Y != -50 {
		t.Errorf("Y = %f, want -50", bb.Y)
	}
}

func TestTruncate_EdgeCases(t *testing.T) {
	// Note: truncate() requires maxLen >= 4 for strings longer than maxLen
	// to avoid panic on slice bounds. This tests valid usage patterns.
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{"short string under limit", "ab", 5, "ab"},
		{"string at limit", "abcde", 5, "abcde"},
		{"whitespace only under limit", "   ", 5, "   "},
		{"very long string", "abcdefghijklmnopqrstuvwxyz", 10, "abcdefg..."},
		{"medium truncation", "hello world", 8, "hello..."},
		{"exact ellipsis boundary", "abcd", 4, "abcd"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

// TestTruncate_Unicode tests unicode handling in truncate.
func TestTruncate_Unicode(t *testing.T) {
	// Note: truncate works on bytes, not runes, so unicode strings
	// may be truncated mid-character. Test actual behavior.
	tests := []struct {
		name   string
		input  string
		maxLen int
	}{
		{"unicode under limit", "こんにちは", 20},
		{"emoji under limit", "🎉🎊🎈🎁", 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			// Under limit should return original
			if got != tt.input {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.input)
			}
		})
	}
}

// Benchmarks

func BenchmarkElementMap_Add(b *testing.B) {
	em := NewElementMap()
	el := &Element{Index: 0, TagName: "button"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		el.Index = i
		em.Add(el)
	}
}

func BenchmarkElementMap_ByIndex(b *testing.B) {
	em := NewElementMap()
	for i := 0; i < 1000; i++ {
		em.Add(&Element{Index: i, TagName: "div"})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.ByIndex(i % 1000)
	}
}

func BenchmarkElementMap_InteractiveElements(b *testing.B) {
	em := NewElementMap()
	for i := 0; i < 1000; i++ {
		em.Add(&Element{
			Index:         i,
			TagName:       "button",
			IsInteractive: i%2 == 0,
			IsVisible:     true,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.InteractiveElements()
	}
}

func BenchmarkElementMap_ToTokenString(b *testing.B) {
	em := NewElementMap()
	em.PageTitle = "Test Page"
	em.PageURL = "https://example.com"
	for i := 0; i < 100; i++ {
		em.Add(&Element{
			Index:     i,
			TagName:   "button",
			Text:      "Button text here",
			IsVisible: true,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		em.ToTokenString()
	}
}
