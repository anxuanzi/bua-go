# E2E Testing for bua-go

## Philosophy

**Unit tests don't work for browser automation agents.** The core value of bua-go is its ability to:
1. Understand web pages visually and structurally
2. Execute actions correctly in real browsers
3. Handle dynamic content and edge cases

These capabilities can only be validated by running actual tasks against real websites.

## Test Structure

```
tests/e2e/
├── README.md          # This file
├── run_tests.go       # Test runner
├── tasks/
│   ├── basic.yaml     # Basic navigation/click tests
│   ├── forms.yaml     # Form filling tests
│   ├── scraping.yaml  # Data extraction tests
│   ├── scroll.yaml    # Scrolling and infinite load tests
│   └── modals.yaml    # Modal/popup handling tests
└── results/           # Test execution logs (gitignored)
```

## Running Tests

```bash
# Run all tests
go run tests/e2e/run_tests.go

# Run specific category
go run tests/e2e/run_tests.go --category basic

# Run single test
go run tests/e2e/run_tests.go --test "google-search"

# Verbose mode (shows step details)
go run tests/e2e/run_tests.go --verbose

# Keep browser open for debugging
go run tests/e2e/run_tests.go --no-headless
```

## Test Definition Format

Tests are defined in YAML files:

```yaml
tests:
  - name: google-search
    description: Search for a term on Google
    url: https://www.google.com
    task: "Search for 'browser automation' and report the first result title"
    timeout: 2m
    expected:
      success: true
      contains_data:
        - "browser"
        - "automation"

  - name: example-click
    description: Click a link on example.com
    url: https://example.com
    task: "Click on 'More information...' link"
    timeout: 1m
    expected:
      success: true
      url_contains: "iana.org"
```

## Expected Fields

- `success`: Whether the task should complete successfully
- `url_contains`: Final URL should contain this substring
- `title_contains`: Final page title should contain this
- `contains_data`: Data returned should contain these strings
- `min_steps`: Minimum number of steps taken
- `max_steps`: Maximum number of steps (to catch loops)

## Adding New Tests

1. Create or edit a YAML file in `tasks/`
2. Define the test with clear success criteria
3. Run with `--test "your-test-name"` to verify
4. Commit the test file

## Best Practices

1. **Use stable websites**: example.com, httpbin.org, test sites
2. **Avoid login-required tests**: Unless testing human takeover
3. **Keep timeouts reasonable**: 1-3 minutes for most tests
4. **Define clear success criteria**: Don't rely on vague expectations
5. **Document flaky tests**: Note if a test may fail due to external factors

## Why Not Unit Tests?

Traditional unit tests mock the browser and LLM, which defeats the purpose:

- **Mocked browser**: Can't verify actual page interactions work
- **Mocked LLM**: Can't verify the agent understands pages correctly
- **Mocked DOM**: Can't verify element detection works on real sites

E2E tests are slower but provide real confidence that the system works.
