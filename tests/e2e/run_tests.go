package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anxuanzi/bua"
	"gopkg.in/yaml.v3"
)

// TestSuite represents a collection of tests from a YAML file.
type TestSuite struct {
	Tests []TestCase `yaml:"tests"`
}

// TestCase represents a single test definition.
type TestCase struct {
	Name        string       `yaml:"name"`
	Description string       `yaml:"description"`
	URL         string       `yaml:"url"`
	Task        string       `yaml:"task"`
	Timeout     string       `yaml:"timeout"`
	Expected    Expectations `yaml:"expected"`
}

// Expectations defines what to validate after task completion.
type Expectations struct {
	Success      bool     `yaml:"success"`
	URLContains  string   `yaml:"url_contains"`
	ContainsData []string `yaml:"contains_data"`
	MinSteps     int      `yaml:"min_steps"`
	MaxSteps     int      `yaml:"max_steps"`
}

// TestResult holds the outcome of a single test.
type TestResult struct {
	Name        string
	Passed      bool
	Duration    time.Duration
	Error       string
	AgentResult *bua.Result
}

// Config holds test runner configuration.
type Config struct {
	Category string
	TestName string
	Verbose  bool
	Headless bool
	Debug    bool
	TasksDir string
	APIKey   string
	Model    string
}

func main() {
	cfg := parseFlags()

	if cfg.APIKey == "" {
		fmt.Println("Error: GEMINI_API_KEY environment variable is required")
		os.Exit(1)
	}

	// Find all test files
	testFiles, err := findTestFiles(cfg.TasksDir, cfg.Category)
	if err != nil {
		fmt.Printf("Error finding test files: %v\n", err)
		os.Exit(1)
	}

	if len(testFiles) == 0 {
		fmt.Println("No test files found")
		os.Exit(1)
	}

	// Load and filter tests
	tests, err := loadTests(testFiles, cfg.TestName)
	if err != nil {
		fmt.Printf("Error loading tests: %v\n", err)
		os.Exit(1)
	}

	if len(tests) == 0 {
		fmt.Println("No tests matched the specified criteria")
		os.Exit(1)
	}

	fmt.Printf("Running %d test(s)...\n\n", len(tests))

	// Run tests
	results := runTests(tests, cfg)

	// Print results
	printResults(results, cfg.Verbose)

	// Exit with appropriate code
	failed := 0
	for _, r := range results {
		if !r.Passed {
			failed++
		}
	}
	if failed > 0 {
		os.Exit(1)
	}
}

func parseFlags() Config {
	cfg := Config{
		APIKey: os.Getenv("GEMINI_API_KEY"),
		Model:  os.Getenv("GEMINI_MODEL"),
	}

	flag.StringVar(&cfg.Category, "category", "", "Run only tests from this category (e.g., basic, forms)")
	flag.StringVar(&cfg.TestName, "test", "", "Run only the test with this name")
	flag.BoolVar(&cfg.Verbose, "verbose", false, "Enable verbose output")
	flag.BoolVar(&cfg.Headless, "headless", false, "Run browser in headless mode")
	flag.BoolVar(&cfg.Debug, "debug", false, "Enable agent debug logging")
	flag.StringVar(&cfg.TasksDir, "tasks", "", "Path to tasks directory")

	// Parse --no-headless as opposite of headless
	noHeadless := flag.Bool("no-headless", false, "Show browser window (opposite of --headless)")

	flag.Parse()

	if *noHeadless {
		cfg.Headless = false
	}

	if cfg.TasksDir == "" {
		// Default to tasks/ directory relative to this file
		exe, _ := os.Executable()
		cfg.TasksDir = filepath.Join(filepath.Dir(exe), "tasks")
		// If running with go run, use relative path
		if _, err := os.Stat(cfg.TasksDir); os.IsNotExist(err) {
			cfg.TasksDir = "tests/e2e/tasks"
		}
	}

	if cfg.Model == "" {
		cfg.Model = "gemini-2.5-flash"
	}

	return cfg
}

func findTestFiles(tasksDir, category string) ([]string, error) {
	pattern := "*.yaml"
	if category != "" {
		pattern = category + ".yaml"
	}

	matches, err := filepath.Glob(filepath.Join(tasksDir, pattern))
	if err != nil {
		return nil, err
	}

	return matches, nil
}

func loadTests(files []string, testName string) ([]TestCase, error) {
	var allTests []TestCase

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read %s: %w", file, err)
		}

		var suite TestSuite
		if err := yaml.Unmarshal(data, &suite); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", file, err)
		}

		for _, test := range suite.Tests {
			if testName == "" || test.Name == testName {
				allTests = append(allTests, test)
			}
		}
	}

	return allTests, nil
}

func runTests(tests []TestCase, cfg Config) []TestResult {
	var results []TestResult

	for i, test := range tests {
		fmt.Printf("[%d/%d] Running: %s\n", i+1, len(tests), test.Name)

		result := runSingleTest(test, cfg)
		results = append(results, result)

		if result.Passed {
			fmt.Printf("       PASSED (%v)\n", result.Duration.Round(time.Millisecond))
		} else {
			fmt.Printf("       FAILED: %s\n", result.Error)
		}
		fmt.Println()
	}

	return results
}

func runSingleTest(test TestCase, cfg Config) TestResult {
	result := TestResult{
		Name: test.Name,
	}

	// Parse timeout
	timeout := 2 * time.Minute
	if test.Timeout != "" {
		if d, err := time.ParseDuration(test.Timeout); err == nil {
			timeout = d
		}
	}

	// Create agent
	agentCfg := bua.Config{
		APIKey:          cfg.APIKey,
		Model:           cfg.Model,
		Headless:        cfg.Headless,
		Debug:           cfg.Debug,
		Preset:          bua.PresetBalanced,
		MaxSteps:        50, // Reasonable limit for tests
		ShowAnnotations: true,
		ShowHighlight:   true,
		ScreenshotDir:   "./screenshots",
	}

	agent, err := bua.New(agentCfg)
	if err != nil {
		result.Error = fmt.Sprintf("failed to create agent: %v", err)
		return result
	}
	defer agent.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Start agent
	startTime := time.Now()
	if err := agent.Start(ctx); err != nil {
		result.Error = fmt.Sprintf("failed to start agent: %v", err)
		result.Duration = time.Since(startTime)
		return result
	}

	// Navigate to URL if specified
	if test.URL != "" {
		if err := agent.Navigate(ctx, test.URL); err != nil {
			result.Error = fmt.Sprintf("failed to navigate: %v", err)
			result.Duration = time.Since(startTime)
			return result
		}
		// Small delay for page load
		time.Sleep(500 * time.Millisecond)
	}

	// Run task
	agentResult, err := agent.Run(ctx, test.Task)
	result.Duration = time.Since(startTime)

	if err != nil {
		result.Error = fmt.Sprintf("task execution error: %v", err)
		return result
	}

	result.AgentResult = agentResult

	// Validate expectations
	if err := validateExpectations(test.Expected, agentResult, agent.GetURL()); err != nil {
		result.Error = err.Error()
		return result
	}

	result.Passed = true
	return result
}

func validateExpectations(exp Expectations, result *bua.Result, finalURL string) error {
	// Check success
	if exp.Success && !result.Success {
		return fmt.Errorf("expected success but task failed: %s", result.Error)
	}

	// Check URL contains
	if exp.URLContains != "" {
		if !strings.Contains(strings.ToLower(finalURL), strings.ToLower(exp.URLContains)) {
			return fmt.Errorf("URL '%s' does not contain '%s'", finalURL, exp.URLContains)
		}
	}

	// Check data contains
	if len(exp.ContainsData) > 0 && result.Data != nil {
		dataStr := fmt.Sprintf("%v", result.Data)
		for _, expected := range exp.ContainsData {
			if !strings.Contains(strings.ToLower(dataStr), strings.ToLower(expected)) {
				return fmt.Errorf("result data does not contain '%s'", expected)
			}
		}
	}

	// Check step count
	stepCount := len(result.Steps)
	if exp.MinSteps > 0 && stepCount < exp.MinSteps {
		return fmt.Errorf("expected at least %d steps, got %d", exp.MinSteps, stepCount)
	}
	if exp.MaxSteps > 0 && stepCount > exp.MaxSteps {
		return fmt.Errorf("expected at most %d steps, got %d (possible infinite loop)", exp.MaxSteps, stepCount)
	}

	return nil
}

func printResults(results []TestResult, verbose bool) {
	fmt.Println("=" + strings.Repeat("=", 59))
	fmt.Println("TEST RESULTS")
	fmt.Println("=" + strings.Repeat("=", 59))

	passed := 0
	failed := 0
	totalDuration := time.Duration(0)

	for _, r := range results {
		totalDuration += r.Duration
		status := "PASS"
		if !r.Passed {
			status = "FAIL"
			failed++
		} else {
			passed++
		}

		fmt.Printf("  [%s] %s (%v)\n", status, r.Name, r.Duration.Round(time.Millisecond))
		if !r.Passed {
			fmt.Printf("         Error: %s\n", r.Error)
		}

		if verbose && r.AgentResult != nil {
			fmt.Printf("         Steps: %d, Tokens: %d\n",
				len(r.AgentResult.Steps), r.AgentResult.TokensUsed)
		}
	}

	fmt.Println("-" + strings.Repeat("-", 59))
	fmt.Printf("Total: %d passed, %d failed (%.1f%% pass rate)\n",
		passed, failed, float64(passed)/float64(len(results))*100)
	fmt.Printf("Total Duration: %v\n", totalDuration.Round(time.Second))
}
