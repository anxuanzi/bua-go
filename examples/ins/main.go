package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/anxuanzi/bua-go"
)

func main() {
	// Get API key from environment
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		log.Fatal("GOOGLE_API_KEY environment variable is required")
	}

	// Create agent configuration with all features enabled
	cfg := bua.Config{
		APIKey:          apiKey,
		Model:           "gemini-3-pro-preview", // Latest model with 1M input, 65K output
		ProfileName:     "instagram",
		Headless:        false, // Show browser for debugging
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

	// Create context with extended timeout for complex tasks
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()

	// Start the browser
	fmt.Println("🚀 Starting browser...")
	if err := agent.Start(ctx); err != nil {
		log.Fatalf("Failed to start agent: %v", err)
	}

	prompt := `
**Objective:**
Act as a Lead Generation Scout. Your goal is to find 3 qualified potential customers for a "Pet Slow Feeder Bowl" on Instagram.

**Output Format:**
Append all found leads to the result in this format: 
[Username] | [Follower Count] | [Why they are a match]

**Execution Rules:**
1. DO NOT interact with ads.
2. DO NOT get stuck in loops; if a page doesn't load, go back.

**Step-by-Step Instructions:**

1. **Search:** Go to "https://www.instagram.com/explore/tags/doglife/" (or search for hashtag #doglife).
	2. **Select Post:** Click on one of the "Top posts" that looks like a real owner photo (not a graphic/ad).
	3. **Scrape Comments:** Open the comments section. Identify 5 distinct usernames who commented text that sounds human (ignore generic "Great pic!" or "Promote on @" bots).
	4. **Investigate Profiles:** For each of those 5 usernames, do the following sequence:
	a. Open their profile
		b. **Check Criteria 1 (Influencer Filter):** Look at "Followers". If they have >10,000 or <50 followers, skip them, go back.
		c. **Check Criteria 2 (Dog Owner):** Look at their Bio and the first 6 photos in their grid. Do you see a dog or mention of a "dog mom/dad"?
	d. **Result:**
		- IF YES (Dog found + Correct follower count): Save their Username and URL.
	- IF NO: skip, next.
	5. **Finish:** Once you have processed the 5 profiles, stop.
`

	result1, err := agent.Run(ctx, prompt)
	if err != nil {
		log.Printf("Task error: %v", err)
	}
	printResult("DOG LIFE RESEARCH ON INS", result1)
}

func printResult(title string, result *bua.Result) {
	fmt.Printf("\n┌─ %s ─", title)
	for i := 0; i < 50-len(title); i++ {
		fmt.Print("─")
	}
	fmt.Println("┐")

	if result == nil {
		fmt.Println("│ ❌ Result is nil")
		fmt.Println("└" + strings.Repeat("─", 60) + "┘")
		return
	}

	fmt.Printf("│ Status: %s\n", statusIcon(result.Success))

	if result.Data != nil {
		data, err := json.MarshalIndent(result.Data, "│ ", "  ")
		if err == nil {
			fmt.Printf("│ Data:\n%s\n", string(data))
		}
	}

	if result.Error != "" {
		fmt.Printf("│ ⚠️  Error: %s\n", result.Error)
	}

	fmt.Println("└" + strings.Repeat("─", 60) + "┘")
}

func statusIcon(success bool) string {
	if success {
		return "✅ Success"
	}
	return "❌ Failed"
}
