// Package main demonstrates multi-page, multi-step browser automation with bua-go.
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

	// Create agent configuration
	cfg := bua.Config{
		APIKey:          apiKey,
		Model:           "gemini-3-flash-preview", // Latest model with 1M input, 65K output
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
OBJECTIVE: Research the Go programming language on Wikipedia.

STEPS:
1. Navigate directly to https://en.wikipedia.org/wiki/Go_(programming_language)
   - This skips the search step and goes straight to the article
2. Wait for the article to load (look for the article title "Go (programming language)")
3. Extract information from the article:

   FROM THE INFOBOX (right sidebar):
   - paradigm: Programming paradigm(s)
   - designed_by: Designer name(s)
   - first_appeared: Year of first release
   - typing_discipline: Type system

   FROM THE ARTICLE TEXT (first 2 paragraphs):
   - summary: 2-3 sentence description of what Go is

4. If any information is not found, set the value to "Not found"

OUTPUT FORMAT (return as JSON):
{
  "source": "Wikipedia",
  "url": "https://en.wikipedia.org/wiki/Go_(programming_language)",
  "paradigm": "<paradigm(s)>",
  "designed_by": "<designer names>",
  "first_appeared": "<year>",
  "typing_discipline": "<typing info>",
  "summary": "<2-3 sentence description>"
}

CONSTRAINTS:
- Stay on the Wikipedia article page
- Do NOT click external links
- Dismiss any popups (donation banners, etc.) if they appear
`)
	if err != nil {
		log.Fatalf("Task 1 failed: %v", err)
	}
	printResult("Wikipedia Research", result1)

	// Task 2: Check GitHub for Go repositories
	fmt.Println("\n🐙 Task 2: Find popular Go repositories on GitHub...")
	result2, err := agent.Run(ctx, `
OBJECTIVE: Find and extract information about popular Go repositories on GitHub.

STEPS:
1. Navigate to https://github.com/search?q=language%3AGo&type=repositories&s=stars&o=desc
   - This URL directly shows Go repositories sorted by stars
2. Wait for results to load (look for repository cards/list items)
3. For the TOP 3 repositories in the results:
   - name: Repository name (format: owner/repo)
   - description: Repository description text
   - stars: Star count (may be abbreviated like "45.2k")
   - url: Full GitHub URL to the repository

OUTPUT FORMAT (return as JSON):
{
  "source": "GitHub",
  "search_query": "Go repositories by stars",
  "repositories": [
    {
      "rank": 1,
      "name": "<owner/repo>",
      "description": "<description>",
      "stars": "<star count>",
      "url": "<github url>"
    }
  ]
}

ERROR HANDLING:
- If GitHub shows a rate limit message, extract what's visible
- If fewer than 3 repos appear, return what's available
- Note any login prompts or restrictions encountered

CONSTRAINTS:
- Do NOT click into individual repositories
- Do NOT attempt to log in
- Extract only from the search results page
`)
	if err != nil {
		log.Fatalf("Task 2 failed: %v", err)
	}
	printResult("GitHub Repositories", result2)

	// Task 3: Check Go documentation
	fmt.Println("\n📖 Task 3: Explore Go official documentation...")
	result3, err := agent.Run(ctx, `
OBJECTIVE: Explore the official Go documentation and learning resources.

STEPS:
1. Navigate to https://go.dev/learn/
   - This is the official "Learn Go" page
2. Wait for the page to load completely
3. Identify the main learning sections/resources available:
   - What tutorials or guides are offered?
   - What is recommended for beginners?
   - Are there interactive features (playground, tour)?
4. Extract the structure of available resources

OUTPUT FORMAT (return as JSON):
{
  "source": "go.dev",
  "url": "https://go.dev/learn/",
  "main_sections": [
    {
      "title": "<section title>",
      "description": "<what this section offers>",
      "target_audience": "<who it's for>"
    }
  ],
  "beginner_recommendation": "<what's recommended for beginners>",
  "interactive_features": ["<feature1>", "<feature2>"]
}

CONSTRAINTS:
- Stay on the /learn page or immediate child pages
- Do NOT download any files
- Focus on understanding the structure, not reading full tutorials
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
