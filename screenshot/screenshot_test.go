// Package screenshot provides tests for screenshot capture and annotation.
package screenshot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/anxuanzi/bua-go/dom"
)

// TestDefaultAnnotationStyle verifies default annotation style values.
func TestDefaultAnnotationStyle(t *testing.T) {
	style := DefaultAnnotationStyle()

	if style == nil {
		t.Fatal("DefaultAnnotationStyle() returned nil")
	}

	if style.BoxWidth != 2 {
		t.Errorf("BoxWidth = %f, want 2", style.BoxWidth)
	}
	if style.FontSize != 12 {
		t.Errorf("FontSize = %f, want 12", style.FontSize)
	}
	if !style.ShowIndex {
		t.Error("ShowIndex should be true by default")
	}
	if style.ShowRole {
		t.Error("ShowRole should be false by default")
	}

	// Check colors are not nil/zero
	if style.BoxColor == nil {
		t.Error("BoxColor should not be nil")
	}
	if style.LabelColor == nil {
		t.Error("LabelColor should not be nil")
	}
	if style.TextColor == nil {
		t.Error("TextColor should not be nil")
	}
}

// TestNewManager tests manager creation.
func TestNewManager(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		cfg := &Config{}
		m := NewManager(cfg)

		if m == nil {
			t.Fatal("NewManager() returned nil")
		}
		if m.config.ImageFormat != "png" {
			t.Errorf("default ImageFormat = %q, want 'png'", m.config.ImageFormat)
		}
		if m.config.Quality != 90 {
			t.Errorf("default Quality = %d, want 90", m.config.Quality)
		}
		if m.config.AnnotationStyle == nil {
			t.Error("AnnotationStyle should be set to default")
		}
	})

	t.Run("with custom config", func(t *testing.T) {
		cfg := &Config{
			Enabled:        true,
			Annotate:       true,
			ImageFormat:    "jpeg",
			Quality:        80,
			MaxScreenshots: 10,
		}
		m := NewManager(cfg)

		if m.config.ImageFormat != "jpeg" {
			t.Errorf("ImageFormat = %q, want 'jpeg'", m.config.ImageFormat)
		}
		if m.config.Quality != 80 {
			t.Errorf("Quality = %d, want 80", m.config.Quality)
		}
		if m.config.MaxScreenshots != 10 {
			t.Errorf("MaxScreenshots = %d, want 10", m.config.MaxScreenshots)
		}
	})

	t.Run("with storage dir", func(t *testing.T) {
		tempDir := t.TempDir()
		storageDir := filepath.Join(tempDir, "screenshots")

		cfg := &Config{
			StorageDir: storageDir,
		}
		m := NewManager(cfg)

		// Directory should be created
		if _, err := os.Stat(storageDir); os.IsNotExist(err) {
			t.Error("StorageDir should be created")
		}

		if m.config.StorageDir != storageDir {
			t.Errorf("StorageDir = %q, want %q", m.config.StorageDir, storageDir)
		}
	})
}

// TestSanitizeFilename tests filename sanitization.
func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with spaces", "with_spaces"},
		{"with/slashes", "withslashes"},
		{"with\\backslash", "withbackslash"},
		{"Special!@#$%", "Special"},
		{"numbers123", "numbers123"},
		{"dashes-and_underscores", "dashes-and_underscores"},
		{"", "screenshot"},
		{"   ", "___"}, // Spaces convert to underscores
		{"a b c", "a_b_c"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeFilename(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSanitizeFilenameTruncation tests length limiting.
func TestSanitizeFilenameTruncation(t *testing.T) {
	longName := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	result := sanitizeFilename(longName)

	if len(result) > 50 {
		t.Errorf("length = %d, want <= 50", len(result))
	}
}

// TestIsScreenshotFile tests file extension detection.
func TestIsScreenshotFile(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"image.png", true},
		{"image.jpg", true},
		{"image.jpeg", true},
		{"image.PNG", false}, // Case sensitive
		{"image.gif", false},
		{"image.bmp", false},
		{"document.txt", false},
		{"file", false},
		{".png", true}, // Edge case
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isScreenshotFile(tt.name)
			if got != tt.want {
				t.Errorf("isScreenshotFile(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

// createTestPNG creates a simple PNG image for testing.
func createTestPNG(width, height int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with solid color
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{255, 255, 255, 255})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// TestAnnotate tests screenshot annotation.
func TestAnnotate(t *testing.T) {
	m := NewManager(&Config{})

	t.Run("nil elements", func(t *testing.T) {
		testPNG, err := createTestPNG(100, 100)
		if err != nil {
			t.Fatalf("Failed to create test PNG: %v", err)
		}

		result, err := m.Annotate(testPNG, nil)
		if err != nil {
			t.Fatalf("Annotate() error = %v", err)
		}
		if !bytes.Equal(result, testPNG) {
			t.Error("Annotate with nil elements should return original data")
		}
	})

	t.Run("empty elements", func(t *testing.T) {
		testPNG, err := createTestPNG(100, 100)
		if err != nil {
			t.Fatalf("Failed to create test PNG: %v", err)
		}

		em := dom.NewElementMap()
		result, err := m.Annotate(testPNG, em)
		if err != nil {
			t.Fatalf("Annotate() error = %v", err)
		}
		if !bytes.Equal(result, testPNG) {
			t.Error("Annotate with empty elements should return original data")
		}
	})

	t.Run("with elements", func(t *testing.T) {
		testPNG, err := createTestPNG(200, 200)
		if err != nil {
			t.Fatalf("Failed to create test PNG: %v", err)
		}

		em := dom.NewElementMap()
		em.Add(&dom.Element{
			Index:     0,
			TagName:   "button",
			IsVisible: true,
			BoundingBox: dom.BoundingBox{
				X:      50,
				Y:      50,
				Width:  100,
				Height: 30,
			},
		})

		result, err := m.Annotate(testPNG, em)
		if err != nil {
			t.Fatalf("Annotate() error = %v", err)
		}

		// Result should be different (has annotations)
		if bytes.Equal(result, testPNG) {
			t.Error("Annotated screenshot should be different from original")
		}

		// Result should be valid PNG
		_, err = png.Decode(bytes.NewReader(result))
		if err != nil {
			t.Errorf("Result is not valid PNG: %v", err)
		}
	})

	t.Run("skip invisible elements", func(t *testing.T) {
		testPNG, err := createTestPNG(200, 200)
		if err != nil {
			t.Fatalf("Failed to create test PNG: %v", err)
		}

		em := dom.NewElementMap()
		em.Add(&dom.Element{
			Index:     0,
			TagName:   "button",
			IsVisible: false, // Not visible
			BoundingBox: dom.BoundingBox{
				X:      50,
				Y:      50,
				Width:  100,
				Height: 30,
			},
		})

		result, err := m.Annotate(testPNG, em)
		if err != nil {
			t.Fatalf("Annotate() error = %v", err)
		}

		// Should return valid PNG (but may still differ due to PNG encoding)
		_, err = png.Decode(bytes.NewReader(result))
		if err != nil {
			t.Errorf("Result is not valid PNG: %v", err)
		}
	})

	t.Run("skip zero-size elements", func(t *testing.T) {
		testPNG, err := createTestPNG(200, 200)
		if err != nil {
			t.Fatalf("Failed to create test PNG: %v", err)
		}

		em := dom.NewElementMap()
		em.Add(&dom.Element{
			Index:     0,
			TagName:   "button",
			IsVisible: true,
			BoundingBox: dom.BoundingBox{
				X:      50,
				Y:      50,
				Width:  0, // Zero width
				Height: 30,
			},
		})

		result, err := m.Annotate(testPNG, em)
		if err != nil {
			t.Fatalf("Annotate() error = %v", err)
		}

		// Should return valid PNG
		_, err = png.Decode(bytes.NewReader(result))
		if err != nil {
			t.Errorf("Result is not valid PNG: %v", err)
		}
	})

	t.Run("invalid image data", func(t *testing.T) {
		_, err := m.Annotate([]byte("not an image"), nil)
		// With nil elements, should return original data
		if err != nil {
			t.Error("Should return original data with nil elements")
		}
	})
}

// TestSave tests screenshot saving.
func TestSave(t *testing.T) {
	t.Run("no storage dir", func(t *testing.T) {
		m := NewManager(&Config{})

		_, err := m.Save([]byte("data"), "test")
		if err == nil {
			t.Error("Save should fail without storage dir")
		}
	})

	t.Run("save screenshot", func(t *testing.T) {
		tempDir := t.TempDir()
		m := NewManager(&Config{
			StorageDir: tempDir,
		})

		testPNG, err := createTestPNG(100, 100)
		if err != nil {
			t.Fatalf("Failed to create test PNG: %v", err)
		}

		path, err := m.Save(testPNG, "test_screenshot")
		if err != nil {
			t.Fatalf("Save() error = %v", err)
		}

		if path == "" {
			t.Error("Save should return file path")
		}

		// File should exist
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Screenshot file should exist")
		}

		// File should contain the data
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read saved file: %v", err)
		}
		if !bytes.Equal(data, testPNG) {
			t.Error("Saved data should match original")
		}
	})
}

// TestList tests screenshot listing.
func TestList(t *testing.T) {
	t.Run("no storage dir", func(t *testing.T) {
		m := NewManager(&Config{})

		paths, err := m.List()
		if err != nil {
			t.Errorf("List() error = %v", err)
		}
		if paths != nil {
			t.Error("List should return nil without storage dir")
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		tempDir := t.TempDir()
		m := NewManager(&Config{
			StorageDir: tempDir,
		})

		paths, err := m.List()
		if err != nil {
			t.Errorf("List() error = %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("List should return empty for empty dir, got %d", len(paths))
		}
	})

	t.Run("list screenshots", func(t *testing.T) {
		tempDir := t.TempDir()
		m := NewManager(&Config{
			StorageDir: tempDir,
		})

		testPNG, _ := createTestPNG(10, 10)

		// Save multiple screenshots with unique names to avoid timestamp collision
		for i := 0; i < 3; i++ {
			_, err := m.Save(testPNG, fmt.Sprintf("test_%d", i))
			if err != nil {
				t.Fatalf("Failed to save screenshot: %v", err)
			}
		}

		paths, err := m.List()
		if err != nil {
			t.Errorf("List() error = %v", err)
		}
		if len(paths) != 3 {
			t.Errorf("List should return 3 screenshots, got %d", len(paths))
		}
	})

	t.Run("ignores non-screenshot files", func(t *testing.T) {
		tempDir := t.TempDir()
		m := NewManager(&Config{
			StorageDir: tempDir,
		})

		// Create non-screenshot files
		os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("test"), 0644)
		os.WriteFile(filepath.Join(tempDir, "data.json"), []byte("{}"), 0644)

		paths, err := m.List()
		if err != nil {
			t.Errorf("List() error = %v", err)
		}
		if len(paths) != 0 {
			t.Errorf("List should ignore non-screenshot files, got %d", len(paths))
		}
	})
}

// TestClear tests screenshot clearing.
func TestClear(t *testing.T) {
	t.Run("no storage dir", func(t *testing.T) {
		m := NewManager(&Config{})

		err := m.Clear()
		if err != nil {
			t.Errorf("Clear() error = %v", err)
		}
	})

	t.Run("clear screenshots", func(t *testing.T) {
		tempDir := t.TempDir()
		m := NewManager(&Config{
			StorageDir: tempDir,
		})

		testPNG, _ := createTestPNG(10, 10)

		// Save screenshots
		for i := 0; i < 3; i++ {
			m.Save(testPNG, "test")
		}

		// Clear
		err := m.Clear()
		if err != nil {
			t.Errorf("Clear() error = %v", err)
		}

		// List should be empty
		paths, _ := m.List()
		if len(paths) != 0 {
			t.Errorf("After Clear, List should return 0, got %d", len(paths))
		}
	})

	t.Run("preserves non-screenshot files", func(t *testing.T) {
		tempDir := t.TempDir()
		m := NewManager(&Config{
			StorageDir: tempDir,
		})

		testPNG, _ := createTestPNG(10, 10)
		m.Save(testPNG, "test")

		// Create non-screenshot file
		txtFile := filepath.Join(tempDir, "readme.txt")
		os.WriteFile(txtFile, []byte("test"), 0644)

		// Clear
		m.Clear()

		// Non-screenshot file should still exist
		if _, err := os.Stat(txtFile); os.IsNotExist(err) {
			t.Error("Clear should not remove non-screenshot files")
		}
	})
}

// TestCleanup tests automatic cleanup of old screenshots.
func TestCleanup(t *testing.T) {
	tempDir := t.TempDir()
	m := NewManager(&Config{
		StorageDir:     tempDir,
		MaxScreenshots: 3,
	})

	testPNG, _ := createTestPNG(10, 10)

	// Save more than max
	for i := 0; i < 5; i++ {
		_, err := m.Save(testPNG, "test")
		if err != nil {
			t.Fatalf("Failed to save screenshot: %v", err)
		}
	}

	// Should only have MaxScreenshots
	paths, _ := m.List()
	if len(paths) > 3 {
		t.Errorf("Should have at most %d screenshots, got %d", 3, len(paths))
	}
}

// TestConfig tests config struct.
func TestConfig(t *testing.T) {
	cfg := Config{
		Enabled:        true,
		Annotate:       true,
		StorageDir:     "/tmp/screenshots",
		MaxScreenshots: 100,
		ImageFormat:    "jpeg",
		Quality:        85,
		AnnotationStyle: &AnnotationStyle{
			BoxWidth: 3,
			FontSize: 14,
		},
	}

	if !cfg.Enabled {
		t.Error("Enabled should be true")
	}
	if !cfg.Annotate {
		t.Error("Annotate should be true")
	}
	if cfg.MaxScreenshots != 100 {
		t.Errorf("MaxScreenshots = %d, want 100", cfg.MaxScreenshots)
	}
	if cfg.AnnotationStyle.BoxWidth != 3 {
		t.Errorf("BoxWidth = %f, want 3", cfg.AnnotationStyle.BoxWidth)
	}
}

// Benchmarks

func BenchmarkAnnotate(b *testing.B) {
	m := NewManager(&Config{})
	testPNG, _ := createTestPNG(1280, 720)

	em := dom.NewElementMap()
	for i := 0; i < 50; i++ {
		em.Add(&dom.Element{
			Index:     i,
			TagName:   "button",
			IsVisible: true,
			BoundingBox: dom.BoundingBox{
				X:      float64(i * 25),
				Y:      float64(i * 10),
				Width:  100,
				Height: 30,
			},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Annotate(testPNG, em)
	}
}

func BenchmarkSave(b *testing.B) {
	tempDir := b.TempDir()
	m := NewManager(&Config{
		StorageDir: tempDir,
	})
	testPNG, _ := createTestPNG(100, 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Save(testPNG, "benchmark")
	}
}
