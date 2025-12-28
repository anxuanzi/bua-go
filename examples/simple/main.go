// Package main demonstrates basic usage of the bua-go library.
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
	// Load .env file from project root
	if err := godotenv.Load("../../.env"); err != nil {
		log.Printf("Warning: Could not load .env file: %v", err)
	}

	// Get API key from environment
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is required")
	}

	// Create agent configuration with all features enabled
	cfg := bua.Config{
		APIKey:          apiKey,
		Model:           "gemini-3-flash-preview", // Latest model with 1M input, 65K output
		ProfileName:     "simple",
		Headless:        false, // Show browser for debugging
		Viewport:        bua.DesktopViewport,
		Debug:           true, // Enable debug logging
		ShowAnnotations: true, // Show element annotations
	}

	// Create the agent
	agent, err := bua.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}
	defer agent.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Start the browser
	fmt.Println("🚀 Starting browser...")
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Navigate to a website
	fmt.Println("🌐 Navigating to Google...")
	if err := agent.Navigate(ctx, "https://www.google.com"); err != nil {
		log.Fatalf("Failed to navigate: %v", err)
	}

	// Run a task with natural language
	fmt.Println("🔍 Running search task...")
	result, err := agent.Run(ctx, `
OBJECTIVE: Search for "Go programming language" and navigate to an informational result.

STEPS:
1. Look for the search input field (usually a textarea or input with placeholder text)
2. Click on the search field to focus it
3. Type: Go programming language
4. Press Enter or click the search button to submit
5. Wait for search results to load
6. Look at the search results:
   - Skip any ads (usually marked as "Ad" or "Sponsored")
   - Find the first organic/natural result
   - Prefer results from official sources (go.dev, wikipedia, golang.org)
7. Click on that result
8. Wait for the page to load

FALLBACK:
- If Google shows a CAPTCHA or blocks the request, try using DuckDuckGo (https://duckduckgo.com) instead
- If no search box is found, the page may have changed - report what you see

SUCCESS CRITERIA:
- You have navigated to a page about the Go programming language
- The page is not a search results page

OUTPUT: Report the final URL you landed on and a brief description of the page content.
`)
	if err != nil {
		log.Fatalf("Task failed: %v", err)
	}

	// Print result
	fmt.Println()
	fmt.Printf("✅ Task completed: success=%v\n", result.Success)
	fmt.Printf("📝 Steps taken: %d\n", len(result.Steps))
	for i, step := range result.Steps {
		fmt.Printf("  %d. %s: %s (%s)\n", i+1, step.Action, step.Target, step.Reasoning)
	}

	if result.Error != "" {
		fmt.Printf("❌ Error: %s\n", result.Error)
	}

	fmt.Println("✨ Done!")
}
