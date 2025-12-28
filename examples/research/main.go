// Package main demonstrates a complex research task requiring multiple tools,
// cross-site data gathering, conditional logic, and structured output.
//
// This example showcases:
// - Multi-site navigation and data extraction
// - Complex prompt engineering with explicit schemas
// - Error handling and fallback strategies
// - Conditional logic based on extracted data
// - All available tools: navigate, click, type, scroll, wait, extract
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
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
		ProfileName:     "research",
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

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║     🔬 COMPREHENSIVE TECHNOLOGY RESEARCH AGENT 🔬            ║")
	fmt.Println("║     Researching: Rust Programming Language                   ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// ═══════════════════════════════════════════════════════════════════
	// TASK 1: Wikipedia Deep Research
	// ═══════════════════════════════════════════════════════════════════
	fmt.Println("📚 TASK 1: Deep Wikipedia Research")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	wikipediaPrompt := `
OBJECTIVE: Extract comprehensive information about the Rust programming language from Wikipedia.

STEPS:
1. Navigate to https://en.wikipedia.org/wiki/Rust_(programming_language)
   - If the page doesn't load, try searching "Rust programming language" from wikipedia.org

2. Wait for the page to fully load (look for the article title)

3. Extract the following information by scrolling and reading the article:

   a) BASIC INFO (from infobox on the right side):
      - Paradigm(s)
      - First appeared (year)
      - Designed by (names)
      - Developer (organization)
      - Stable release version
      - Typing discipline
      - File extensions

   b) OVERVIEW (from first 2-3 paragraphs):
      - What is Rust? (2-3 sentence summary)
      - Main design goals
      - Key features that distinguish it

   c) HISTORY (scroll to History section if exists):
      - When development started
      - Key milestones
      - Current status

4. If any section is not found, set the value to "Not found on page"

OUTPUT FORMAT (return as JSON):
{
  "source": "Wikipedia",
  "url": "<actual URL>",
  "basic_info": {
    "paradigms": ["<paradigm1>", "<paradigm2>"],
    "first_appeared": "<year>",
    "designers": ["<name1>", "<name2>"],
    "developer": "<organization>",
    "stable_release": "<version>",
    "typing": "<typing discipline>",
    "file_extensions": [".rs"]
  },
  "overview": {
    "summary": "<2-3 sentence description>",
    "design_goals": ["<goal1>", "<goal2>"],
    "key_features": ["<feature1>", "<feature2>"]
  },
  "history": {
    "development_started": "<year or description>",
    "milestones": ["<milestone1>", "<milestone2>"],
    "current_status": "<description>"
  }
}

CONSTRAINTS:
- Do NOT click on external links
- Do NOT navigate away from Wikipedia
- If a popup appears (cookie consent, etc.), dismiss it first
- Scroll to find information, don't assume it's visible
`

	result1, err := agent.Run(ctx, wikipediaPrompt)
	if err != nil {
		log.Printf("Task 1 error: %v", err)
	}
	printResult("Wikipedia Research", result1)

	// ═══════════════════════════════════════════════════════════════════
	// TASK 2: GitHub Repository Analysis
	// ═══════════════════════════════════════════════════════════════════
	fmt.Println("\n🐙 TASK 2: GitHub Repository Analysis")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	githubPrompt := `
OBJECTIVE: Analyze the official Rust repository on GitHub to extract project health metrics.

STEPS:
1. Navigate directly to https://github.com/rust-lang/rust
   - This is the official Rust compiler repository

2. Wait for the repository page to load (look for the repo name header)

3. Extract the following metrics from the repository page:

   a) REPOSITORY STATS (visible near top of page):
      - Star count (number with "Star" or star icon)
      - Fork count (number with "Fork" or fork icon)
      - Watcher count (if visible)
      - License type

   b) ACTIVITY INDICATORS:
      - Last commit date/time (look for "commits" section)
      - Number of contributors (if visible)
      - Open issues count
      - Open pull requests count

   c) REPOSITORY INFO:
      - Primary language
      - Repository description
      - Topics/tags (if any)

4. Scroll down to see the README preview if needed

5. If you encounter a rate limit or login prompt:
   - Extract whatever information is visible
   - Note the limitation in the output

OUTPUT FORMAT (return as JSON):
{
  "source": "GitHub",
  "repository": "rust-lang/rust",
  "url": "https://github.com/rust-lang/rust",
  "stats": {
    "stars": "<number or 'N/A'>",
    "forks": "<number or 'N/A'>",
    "watchers": "<number or 'N/A'>",
    "license": "<license type>"
  },
  "activity": {
    "last_commit": "<date/time or relative time>",
    "contributors": "<number or 'N/A'>",
    "open_issues": "<number or 'N/A'>",
    "open_prs": "<number or 'N/A'>"
  },
  "info": {
    "primary_language": "<language>",
    "description": "<repo description>",
    "topics": ["<topic1>", "<topic2>"]
  },
  "limitations": "<any issues encountered, or 'None'>"
}

CONSTRAINTS:
- Do NOT click on Issues, PRs, or other tabs - stay on main page
- Do NOT attempt to log in
- Extract only what's visible on the main repository page
- If numbers are abbreviated (e.g., "100k"), keep the abbreviation
`

	result2, err := agent.Run(ctx, githubPrompt)
	if err != nil {
		log.Printf("Task 2 error: %v", err)
	}
	printResult("GitHub Analysis", result2)

	// ═══════════════════════════════════════════════════════════════════
	// TASK 3: Official Documentation Structure
	// ═══════════════════════════════════════════════════════════════════
	fmt.Println("\n📖 TASK 3: Official Documentation Analysis")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	docsPrompt := `
OBJECTIVE: Analyze the official Rust documentation to understand learning resources.

STEPS:
1. Navigate to https://www.rust-lang.org/learn
   - This is the official "Learn Rust" page

2. Wait for the page to load completely

3. Analyze the page structure and extract:

   a) MAIN LEARNING PATHS:
      - Identify the primary learning resources listed
      - For each resource, get: name, brief description, target audience

   b) DOCUMENTATION SECTIONS:
      - What official books/guides are available?
      - Are there tutorials or examples mentioned?

   c) GETTING STARTED:
      - What does the page recommend for beginners?
      - Is there a "quick start" or "first steps" section?

4. If there are multiple tabs or sections, focus on what's immediately visible
   - You may scroll to see more content
   - Do NOT click into individual books/guides

5. Note the overall structure and navigation options available

OUTPUT FORMAT (return as JSON):
{
  "source": "Rust Official",
  "url": "https://www.rust-lang.org/learn",
  "learning_paths": [
    {
      "name": "<resource name>",
      "description": "<brief description>",
      "target_audience": "<who it's for>",
      "type": "<book/tutorial/reference/course>"
    }
  ],
  "recommended_for_beginners": "<specific recommendation>",
  "documentation_highlights": [
    "<highlight1>",
    "<highlight2>"
  ],
  "page_structure": {
    "main_sections": ["<section1>", "<section2>"],
    "navigation_options": ["<option1>", "<option2>"]
  }
}

CONSTRAINTS:
- Stay on the /learn page
- Do NOT download any files
- Do NOT click into external links
- Focus on extracting the structure, not reading full content
`

	result3, err := agent.Run(ctx, docsPrompt)
	if err != nil {
		log.Printf("Task 3 error: %v", err)
	}
	printResult("Documentation Analysis", result3)

	// ═══════════════════════════════════════════════════════════════════
	// TASK 4: Community & Ecosystem (Crates.io)
	// ═══════════════════════════════════════════════════════════════════
	fmt.Println("\n📦 TASK 4: Package Ecosystem Analysis (crates.io)")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	cratesPrompt := `
OBJECTIVE: Analyze the Rust package ecosystem by examining crates.io.

STEPS:
1. Navigate to https://crates.io/
   - This is the official Rust package registry

2. Wait for the page to load

3. On the homepage, extract:

   a) ECOSYSTEM STATS (usually displayed prominently):
      - Total number of crates (packages)
      - Total downloads (if shown)
      - Any other ecosystem statistics visible

   b) FEATURED/POPULAR CRATES:
      - Look for "Most Downloaded", "Popular", or "Featured" sections
      - Extract top 5 crates with their:
        * Name
        * Download count (if visible)
        * Brief description

   c) CATEGORIES:
      - What categories of crates are highlighted?
      - Are there any trending sections?

4. Use the search functionality to search for "web framework":
   - Type "web framework" in the search box
   - Press Enter or click search
   - Wait for results
   - Extract the top 3 web framework crates from results

OUTPUT FORMAT (return as JSON):
{
  "source": "crates.io",
  "url": "https://crates.io/",
  "ecosystem_stats": {
    "total_crates": "<number or 'N/A'>",
    "total_downloads": "<number or 'N/A'>",
    "other_stats": {}
  },
  "popular_crates": [
    {
      "name": "<crate name>",
      "downloads": "<count>",
      "description": "<brief description>"
    }
  ],
  "categories_highlighted": ["<category1>", "<category2>"],
  "web_frameworks_search": [
    {
      "name": "<framework name>",
      "downloads": "<count>",
      "description": "<description>"
    }
  ]
}

CONSTRAINTS:
- Do NOT click on individual crate pages
- Stay on main page and search results only
- If search doesn't work, note it and continue with homepage data
`

	result4, err := agent.Run(ctx, cratesPrompt)
	if err != nil {
		log.Printf("Task 4 error: %v", err)
	}
	printResult("Ecosystem Analysis", result4)

	// ═══════════════════════════════════════════════════════════════════
	// TASK 5: Comparative Analysis with Similar Language
	// ═══════════════════════════════════════════════════════════════════
	fmt.Println("\n⚔️ TASK 5: Comparative Research (Rust vs Go)")
	fmt.Println("─────────────────────────────────────────────────────────────────")

	comparePrompt := `
OBJECTIVE: Research Go programming language for comparison with Rust.

STEPS:
1. Navigate to https://go.dev/
   - This is the official Go website

2. Wait for the page to load

3. Extract key information about Go:

   a) TAGLINE/POSITIONING:
      - What is Go's main tagline or value proposition?
      - How does it describe itself?

   b) KEY FEATURES (from homepage):
      - What features are highlighted?
      - What use cases are mentioned?

   c) GETTING STARTED:
      - Is there a quick way to try Go?
      - What's the recommended first step?

4. Navigate to https://go.dev/doc/
   - Extract what documentation is available
   - Note the structure of learning resources

5. Based on your research of both Rust (previous tasks) and Go:
   - Identify key differences in positioning
   - Note different target use cases
   - Compare learning resource approaches

OUTPUT FORMAT (return as JSON):
{
  "go_research": {
    "source": "go.dev",
    "tagline": "<main value proposition>",
    "key_features": ["<feature1>", "<feature2>"],
    "use_cases": ["<usecase1>", "<usecase2>"],
    "getting_started": "<recommended first step>",
    "documentation_structure": ["<doc1>", "<doc2>"]
  },
  "comparison_notes": {
    "positioning_difference": "<how they position differently>",
    "target_audience_difference": "<who each targets>",
    "learning_approach_difference": "<how learning resources differ>",
    "key_technical_difference": "<main technical distinction>"
  }
}

CONSTRAINTS:
- Visit only go.dev pages
- Do NOT attempt to run any code
- Focus on extracting positioning and structure information
- Keep comparison notes brief and factual
`

	result5, err := agent.Run(ctx, comparePrompt)
	if err != nil {
		log.Printf("Task 5 error: %v", err)
	}
	printResult("Comparative Analysis", result5)

	// ═══════════════════════════════════════════════════════════════════
	// FINAL SUMMARY
	// ═══════════════════════════════════════════════════════════════════
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║                    📊 RESEARCH COMPLETE 📊                    ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("✅ Tasks Completed:")
	fmt.Println("   1. Wikipedia Deep Research - Language fundamentals")
	fmt.Println("   2. GitHub Repository Analysis - Project health metrics")
	fmt.Println("   3. Official Documentation - Learning resources structure")
	fmt.Println("   4. Package Ecosystem - crates.io analysis")
	fmt.Println("   5. Comparative Analysis - Rust vs Go positioning")
	fmt.Println()
	fmt.Println("📁 Screenshots saved to: ~/.bua/screenshots/steps/")
	fmt.Println()
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
