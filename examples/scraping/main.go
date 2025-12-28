// Package main demonstrates web scraping with bua-go.
package main

import (
	"context"
	"encoding/json"
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
		ProfileName:     "scraping",
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

	// Example: Scrape Hacker News top stories
	fmt.Println("📰 Scraping Hacker News top stories...")
	result, err := agent.Run(ctx, `
OBJECTIVE: Extract the top 5 stories from Hacker News with their metadata.

STEPS:
1. Navigate to https://news.ycombinator.com
2. Wait for the page to fully load (look for the orange header bar)
3. Identify the story list structure:
   - Each story has a title link (class "titleline" or similar)
   - Below each title is metadata: points, author, time, comments
4. For each of the TOP 5 stories (numbered 1-5 on the left), extract:
   - title: The story headline text
   - url: The href of the title link (may be external or internal)
   - points: Number of points/upvotes (e.g., "142 points")
   - comments: Number of comments (e.g., "89 comments")
   - posted_by: Username who posted it
5. Scroll if needed to see all 5 stories (they should all be visible)

OUTPUT FORMAT (return as JSON):
{
  "source": "Hacker News",
  "scraped_at": "<current page title or time>",
  "stories": [
    {
      "rank": 1,
      "title": "<story title>",
      "url": "<story URL>",
      "points": "<number>",
      "comments": "<number>",
      "posted_by": "<username>"
    }
  ]
}

ERROR HANDLING:
- If a field is not visible, set it to "N/A"
- If fewer than 5 stories exist, return what's available
- If the page structure is different than expected, describe what you see

CONSTRAINTS:
- Do NOT click on any stories
- Do NOT navigate away from the main page
- Extract data only from what's visible on the homepage
`)
	if err != nil {
		log.Fatalf("Task failed: %v", err)
	}

	// Print result
	fmt.Println()
	fmt.Printf("✅ Task completed: success=%v\n", result.Success)

	if result.Data != nil {
		// Pretty print the extracted data
		data, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Printf("📊 Extracted data:\n%s\n", data)
	}

	// Print steps for debugging
	fmt.Printf("\n📝 Steps taken: %d\n", len(result.Steps))
	for i, step := range result.Steps {
		fmt.Printf("  %d. %s: %s\n", i+1, step.Action, step.Reasoning)
	}

	if result.Error != "" {
		fmt.Printf("❌ Error: %s\n", result.Error)
	}
}
