// Package main demonstrates multi-page, multi-step browser automation with bua-go.
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
		ProfileName:     "multipage",
		Headless:        false, // Show browser for demonstration
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Start the browser
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	fmt.Println("=== Multi-Page Browser Automation Demo ===")
	fmt.Println()

	// Task 1: Research a topic on Wikipedia
	fmt.Println("📚 Task 1: Research 'Go programming language' on Wikipedia...")
	result1, err := agent.Run(ctx, `
		1. Navigate to https://en.wikipedia.org
		2. Search for "Go programming language" using the search box
		3. Click on the main article result
		4. Extract the following information from the article:
		   - The first paragraph summary (what is Go?)
		   - Who designed the language
		   - When it was first released
		   - The programming paradigm
		5. Return the extracted data as JSON
	`)
	if err != nil {
		log.Fatalf("Task 1 failed: %v", err)
	}
	printResult("Wikipedia Research", result1)

	// Task 2: Check GitHub for Go repositories
	fmt.Println("\n🐙 Task 2: Find popular Go repositories on GitHub...")
	result2, err := agent.Run(ctx, `
		1. Navigate to https://github.com
		2. Search for "golang" in the search box
		3. Wait for results to load
		4. Filter or look for repositories (not users or other types)
		5. Extract the names and descriptions of the top 3 repositories
		6. For each repository, also get the star count if visible
		7. Return the data as a JSON array
	`)
	if err != nil {
		log.Fatalf("Task 2 failed: %v", err)
	}
	printResult("GitHub Repositories", result2)

	// Task 3: Check Go documentation
	fmt.Println("\n📖 Task 3: Explore Go official documentation...")
	result3, err := agent.Run(ctx, `
		1. Navigate to https://go.dev
		2. Find and click on the "Learn" or "Docs" section
		3. Look for the "Getting Started" guide or tutorial
		4. Extract the main steps or sections listed in the getting started guide
		5. Return a summary of what someone would learn from this guide
	`)
	if err != nil {
		log.Fatalf("Task 3 failed: %v", err)
	}
	printResult("Go Documentation", result3)

	// Final Summary
	fmt.Println("\n==================================================")
	fmt.Println("✅ Multi-page automation completed successfully!")
	fmt.Println("   - Researched Go on Wikipedia")
	fmt.Println("   - Found popular Go repositories on GitHub")
	fmt.Println("   - Explored Go official documentation")
}

func printResult(title string, result *bua.Result) {
	fmt.Printf("\n--- %s ---\n", title)
	fmt.Printf("Success: %v\n", result.Success)

	if result.Data != nil {
		data, _ := json.MarshalIndent(result.Data, "", "  ")
		fmt.Printf("Data:\n%s\n", data)
	}

	if result.Error != "" {
		fmt.Printf("Error: %s\n", result.Error)
	}
}
