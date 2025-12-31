// Package main demonstrates the action highlighting system in bua-go.
// Run this example to see:
// - Corner brackets around clicked elements
// - "typing..." labels on input fields
// - Scroll direction indicators
// - Crosshairs for coordinate-based clicks
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/anxuanzi/bua-go"
)

func main() {
	// Load .env file
	_ = godotenv.Load(".env")

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is required")
	}

	// Create agent with visible browser and highlights
	// The ShowHighlights option controls action indicators:
	// - Corner brackets around elements being clicked
	// - Crosshairs for coordinate-based clicks
	// - "typing..." labels when entering text
	// - Scroll direction indicators
	showHighlights := true
	agent, err := bua.New(bua.Config{
		APIKey:          apiKey,
		Model:           bua.ModelGemini3Pro,
		Preset:          bua.PresetQuality,
		Headless:        false,                  // Must be non-headless to see highlights
		ShowHighlights:  &showHighlights,        // Explicitly enable (default is true for non-headless)
		HighlightDelay:  500 * time.Millisecond, // Slower highlights for demo visibility
		Debug:           true,
		ShowAnnotations: true,
		EnhancedDOM:     true,
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	fmt.Println("Starting browser...")
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start: %v", err)
	}

	// Demo 1: Search with click and type highlights
	fmt.Println("\n=== Demo 1: Search (click + type highlights) ===")
	if err := agent.Navigate(ctx, "https://www.google.com"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	result, err := agent.Run(ctx, `Search for "browser automation". Watch for orange corner brackets on the search box and submit button.`)
	if err != nil {
		log.Printf("Demo 1 failed: %v", err)
	} else {
		fmt.Printf("Demo 1 result: success=%v, steps=%d\n", result.Success, len(result.Steps))
	}

	// Demo 2: Scroll demo
	fmt.Println("\n=== Demo 2: Scroll (scroll indicator) ===")
	if err := agent.Navigate(ctx, "https://news.ycombinator.com"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	result, err = agent.Run(ctx, `Scroll down the page to see more headlines. Watch for the scroll indicator.`)
	if err != nil {
		log.Printf("Demo 2 failed: %v", err)
	} else {
		fmt.Printf("Demo 2 result: success=%v, steps=%d\n", result.Success, len(result.Steps))
	}

	// Demo 3: Multiple clicks
	fmt.Println("\n=== Demo 3: Navigation (multiple click highlights) ===")
	result, err = agent.Run(ctx, `Click on the first headline link to view the article. Watch the corner brackets highlight each element.`)
	if err != nil {
		log.Printf("Demo 3 failed: %v", err)
	} else {
		fmt.Printf("Demo 3 result: success=%v, steps=%d\n", result.Success, len(result.Steps))
	}

	fmt.Println("\n=== Highlight Demo Complete ===")
	fmt.Println("You should have seen:")
	fmt.Println("  - Orange corner brackets on clicked elements")
	fmt.Println("  - 'typing...' labels on input fields")
	fmt.Println("  - Scroll direction indicators")
	fmt.Println("\nHighlights are:")
	fmt.Println("  - Enabled by default in non-headless mode")
	fmt.Println("  - Configurable via ShowHighlights and HighlightDelay")
	fmt.Println("  - Auto-removed after each action")
}
