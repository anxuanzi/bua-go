# E2E Testing for bua-go

End-to-end testing framework for validating AI browser agent behavior with real browser interactions.

## Why E2E Tests?

Traditional Go unit tests (`go test ./...`) are useful for pure functions, but they **cannot** validate AI agent behavior because:

1. **Non-deterministic behavior**: LLM responses vary between runs
2. **Real browser required**: DOM extraction, screenshots, and element interaction need actual browsers
3. **Integration is the point**: Testing isolated components misses the core functionality
4. **Vision + DOM hybrid**: The agent sees annotated screenshots - can't be simulated

## Directory Structure

```
tests/e2e/
├── run_tests.go        # Test runner
├── README.md           # This file
└── tasks/
    ├── basic.yaml      # Navigation, clicking, page content
    ├── forms.yaml      # Form filling, dropdowns, checkboxes
    ├── scraping.yaml   # Data extraction, tables, lists
    └── scroll.yaml     # Scrolling, infinite scroll, element scroll
```

## Prerequisites

1. Set the `GEMINI_API_KEY` environment variable:
   ```bash
   export GEMINI_API_KEY="your-api-key"
   ```

2. Optionally set `GEMINI_MODEL` (defaults to `gemini-2.5-flash`):
   ```bash
   export GEMINI_MODEL="gemini-2.5-flash"
   ```

## Running Tests

### Run all tests
```bash
go run tests/e2e/run_tests.go
```

### Run specific category
```bash
go run tests/e2e/run_tests.go --category basic
go run tests/e2e/run_tests.go --category forms
go run tests/e2e/run_tests.go --category scraping
go run tests/e2e/run_tests.go --category scroll
```

### Run single test by name
```bash
go run tests/e2e/run_tests.go --test "google-search"
go run tests/e2e/run_tests.go --test "wikipedia-navigation"
```

### Debug mode (visible browser)
```bash
go run tests/e2e/run_tests.go --no-headless --verbose
```

### All options
```bash
go run tests/e2e/run_tests.go \
  --category basic \        # Filter by category
  --test "test-name" \      # Filter by test name
  --verbose \               # Show detailed output
  --no-headless \           # Show browser window
  --debug \                 # Enable agent debug logging
  --tasks ./custom/path     # Custom tasks directory
```

## YAML Test Format

```yaml
tests:
  - name: test-name              # Unique identifier
    description: What this tests # Human-readable description
    url: https://example.com     # Starting URL (optional)
    task: "Natural language task for the agent"
    timeout: 2m                  # Max execution time
    expected:
      success: true              # Task must complete successfully
      url_contains: "expected"   # Final URL must contain this string
      contains_data:             # Result data must include these (case-insensitive)
        - "expected content"
      min_steps: 2               # Minimum steps (detect insufficient action)
      max_steps: 10              # Maximum steps (detect infinite loops)
```

## Expectations Reference

| Field | Type | Description |
|-------|------|-------------|
| `success` | bool | Whether the task must complete successfully |
| `url_contains` | string | Final URL must contain this substring |
| `contains_data` | []string | Result data must include these strings |
| `min_steps` | int | Minimum number of steps required |
| `max_steps` | int | Maximum steps allowed (loop detection) |

## Test Categories

### basic.yaml
Tests fundamental browser automation:
- Page navigation
- Link clicking
- Search functionality
- Basic page interactions

### forms.yaml
Tests form interactions:
- Text input filling
- Dropdown selection
- Checkbox toggling
- Form submission

### scraping.yaml
Tests data extraction:
- Text content extraction
- List/table data reading
- Structured data extraction
- JSON response reading

### scroll.yaml
Tests scrolling behavior:
- Page scrolling
- Scroll to element
- Infinite scroll handling
- Footer/section navigation

## Writing New Tests

1. Add test case to appropriate YAML file (or create new category file)
2. Use clear, specific task descriptions
3. Set realistic timeouts and step limits
4. Test locally with `--no-headless --verbose`

### Example Test

```yaml
tests:
  - name: my-new-test
    description: Test something specific
    url: https://example.com
    task: "Navigate to the about page and find the company mission statement"
    timeout: 2m
    expected:
      success: true
      url_contains: "about"
      min_steps: 2
      max_steps: 10
```

## Debugging Failed Tests

1. Run with visible browser:
   ```bash
   go run tests/e2e/run_tests.go --test "failing-test" --no-headless --verbose --debug
   ```

2. Check agent steps in output to understand what the agent did

3. Review the task description - is it clear enough for the LLM?

4. Adjust step limits if the agent is taking too many/few steps

## CI/CD Integration

Example GitHub Actions workflow:

```yaml
name: E2E Tests
on: [push, pull_request]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install Chrome
        uses: browser-actions/setup-chrome@latest

      - name: Run E2E Tests
        env:
          GEMINI_API_KEY: ${{ secrets.GEMINI_API_KEY }}
        run: go run tests/e2e/run_tests.go --category basic
```

## Troubleshooting

### "GEMINI_API_KEY environment variable is required"
Set the environment variable with your Gemini API key.

### Tests timeout frequently
- Increase the `timeout` value in test definition
- Check network connectivity
- Target pages may have changed or be slow

### "expected at most X steps, got Y"
The agent may be stuck in a loop. Debug with `--no-headless` to see what's happening.

### Browser fails to start
- Ensure Chrome/Chromium is installed
- Check for conflicting browser processes
- Try increasing startup timeout
