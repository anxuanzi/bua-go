// E2E Test Runner for bua-go
//
// This program runs end-to-end tests against real websites to verify
// browser automation capabilities. Unit tests don't work for AI agents
// because the value is in real browser interaction, not mocked behavior.
//
// Usage:
//
//	go run tests/e2e/run_tests.go                    # Run all tests
//	go run tests/e2e/run_tests.go --category basic  # Run specific category
//	go run tests/e2e/run_tests.go --test "google-search"  # Run single test
//	go run tests/e2e/run_tests.go --verbose         # Show step details
//	go run tests/e2e/run_tests.go --no-headless     # Keep browser visible
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"github.com/anxuanzi/bua-go"
)

// TestFile represents a YAML test file
type TestFile struct {
	Tests []TestCase `yaml:"tests"`
}

// TestCase represents a single test
type TestCase struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	URL         string   `yaml:"url"`
	Task        string   `yaml:"task"`
	Timeout     string   `yaml:"timeout"`
	Expected    Expected `yaml:"expected"`
}

// Expected defines test success criteria
type Expected struct {
	Success      bool     `yaml:"success"`
	URLContains  string   `yaml:"url_contains"`
	ContainsData []string `yaml:"contains_data"`
	MinSteps     int      `yaml:"min_steps"`
	MaxSteps     int      `yaml:"max_steps"`
}

// TestResult holds the result of running a test
type TestResult struct {
	Name     string
	Passed   bool
	Duration time.Duration
	Error    string
	Steps    int
}

func main() {
	// Parse flags
	category := flag.String("category", "", "Run tests from specific category (basic, forms, scraping, scroll)")
	testName := flag.String("test", "", "Run single test by name")
	verbose := flag.Bool("verbose", false, "Show step details")
	noHeadless := flag.Bool("no-headless", false, "Keep browser visible for debugging")
	flag.Parse()

	// Load .env file
	_ = godotenv.Load()
	_ = godotenv.Load(".env")

	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		fmt.Println("ERROR: GOOGLE_API_KEY environment variable is required")
		fmt.Println("Set it in .env file or environment")
		os.Exit(1)
	}

	// Find test files
	tasksDir := filepath.Join("tests", "e2e", "tasks")
	if _, err := os.Stat(tasksDir); os.IsNotExist(err) {
		// Try relative to script location
		tasksDir = "tasks"
	}

	var testFiles []string
	if *category != "" {
		testFiles = []string{filepath.Join(tasksDir, *category+".yaml")}
	} else {
		files, err := filepath.Glob(filepath.Join(tasksDir, "*.yaml"))
		if err != nil || len(files) == 0 {
			fmt.Println("ERROR: No test files found in", tasksDir)
			os.Exit(1)
		}
		testFiles = files
	}

	// Load and run tests
	var allResults []TestResult
	for _, file := range testFiles {
		results, err := runTestFile(file, apiKey, *testName, *verbose, !*noHeadless)
		if err != nil {
			fmt.Printf("ERROR loading %s: %v\n", file, err)
			continue
		}
		allResults = append(allResults, results...)
	}

	// Print summary
	printSummary(allResults)
}

func runTestFile(file string, apiKey string, singleTest string, verbose bool, headless bool) ([]TestResult, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	var tf TestFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, err
	}

	fmt.Printf("\n=== Running tests from %s ===\n\n", filepath.Base(file))

	var results []TestResult
	for _, tc := range tf.Tests {
		// Skip if running specific test
		if singleTest != "" && tc.Name != singleTest {
			continue
		}

		result := runTest(tc, apiKey, verbose, headless)
		results = append(results, result)

		// Print result
		if result.Passed {
			fmt.Printf("  ✅ %s (%.1fs, %d steps)\n", result.Name, result.Duration.Seconds(), result.Steps)
		} else {
			fmt.Printf("  ❌ %s: %s\n", result.Name, result.Error)
		}
	}

	return results, nil
}

func runTest(tc TestCase, apiKey string, verbose bool, headless bool) TestResult {
	start := time.Now()
	result := TestResult{Name: tc.Name}

	// Parse timeout
	timeout := 2 * time.Minute
	if tc.Timeout != "" {
		if d, err := time.ParseDuration(tc.Timeout); err == nil {
			timeout = d
		}
	}

	// Create agent
	agent, err := bua.New(bua.Config{
		APIKey:   apiKey,
		Model:    bua.ModelGemini3Flash,
		Headless: headless,
		Debug:    verbose,
	})
	if err != nil {
		result.Error = fmt.Sprintf("failed to create agent: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	defer agent.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Start browser
	if err := agent.Start(ctx); err != nil {
		result.Error = fmt.Sprintf("failed to start: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// Navigate to URL
	if err := agent.Navigate(ctx, tc.URL); err != nil {
		result.Error = fmt.Sprintf("failed to navigate: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	// Run task
	taskResult, err := agent.Run(ctx, tc.Task)
	if err != nil {
		result.Error = fmt.Sprintf("task failed: %v", err)
		result.Duration = time.Since(start)
		return result
	}

	result.Steps = len(taskResult.Steps)
	result.Duration = time.Since(start)

	// Validate expectations
	if !validateExpectations(tc.Expected, taskResult, &result) {
		return result
	}

	result.Passed = true
	return result
}

func validateExpectations(exp Expected, taskResult *bua.Result, result *TestResult) bool {
	// Check success
	if exp.Success && !taskResult.Success {
		result.Error = fmt.Sprintf("expected success but got failure: %s", taskResult.Error)
		return false
	}

	// Check min steps
	if exp.MinSteps > 0 && len(taskResult.Steps) < exp.MinSteps {
		result.Error = fmt.Sprintf("expected at least %d steps, got %d", exp.MinSteps, len(taskResult.Steps))
		return false
	}

	// Check max steps
	if exp.MaxSteps > 0 && len(taskResult.Steps) > exp.MaxSteps {
		result.Error = fmt.Sprintf("expected at most %d steps, got %d (possible loop)", exp.MaxSteps, len(taskResult.Steps))
		return false
	}

	// Check data contains
	if len(exp.ContainsData) > 0 && taskResult.Data != nil {
		dataStr := fmt.Sprintf("%v", taskResult.Data)
		for _, needle := range exp.ContainsData {
			if !strings.Contains(strings.ToLower(dataStr), strings.ToLower(needle)) {
				result.Error = fmt.Sprintf("data should contain '%s' but got: %s", needle, truncate(dataStr, 200))
				return false
			}
		}
	}

	return true
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func printSummary(results []TestResult) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("TEST SUMMARY")
	fmt.Println(strings.Repeat("=", 60))

	passed := 0
	failed := 0
	totalDuration := time.Duration(0)

	for _, r := range results {
		totalDuration += r.Duration
		if r.Passed {
			passed++
		} else {
			failed++
		}
	}

	fmt.Printf("\nTotal: %d tests\n", len(results))
	fmt.Printf("Passed: %d ✅\n", passed)
	fmt.Printf("Failed: %d ❌\n", failed)
	fmt.Printf("Duration: %.1fs\n", totalDuration.Seconds())

	if failed > 0 {
		fmt.Println("\nFailed tests:")
		for _, r := range results {
			if !r.Passed {
				fmt.Printf("  - %s: %s\n", r.Name, r.Error)
			}
		}
		os.Exit(1)
	}

	fmt.Println("\nAll tests passed! ✅")
}
