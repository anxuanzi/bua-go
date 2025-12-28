// Package main demonstrates web scraping with bua-go.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/anxuanzi/bua-go"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is required")
	}

	// Create agent configuration
	cfg := bua.Config{
		APIKey:          apiKey,
		Model:           "gemini-2.5-flash",
		ProfileName:     "scraping",
		Headless:        false, // Show browser for testing
		Viewport:        bua.DesktopViewport,
		Debug:           true,
		ShowAnnotations: true,
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
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	// Example: Scrape Hacker News top stories
	result, err := agent.Run(ctx, `
		Navigate to news.ycombinator.com and extract the titles and URLs
		of the top 5 stories. Return the data as a JSON array with objects
		containing 'title' and 'url' fields.
	`)
	if err != nil {
		log.Fatalf("Task failed: %v", err)
	}

	// Print result
	fmt.Printf("Task completed: success=%v\n", result.Success)

	if result.Data != nil {
		// Pretty print the extracted data
		data, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Printf("Extracted data:\n%s\n", data)
	}

	// Print steps for debugging
	fmt.Printf("\nSteps taken: %d\n", len(result.Steps))
	for i, step := range result.Steps {
		fmt.Printf("  %d. %s: %s\n", i+1, step.Action, step.Reasoning)
	}

	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
}
