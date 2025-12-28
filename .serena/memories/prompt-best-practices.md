# Prompt Engineering Best Practices for bua-go

## Prompt Structure Template

```
OBJECTIVE: [Clear, single-sentence goal]

STEPS:
1. [Action] - [Optional detail/fallback]
2. [Action]
3. [Specific extraction/interaction instructions]
   - Field 1: description
   - Field 2: description

OUTPUT FORMAT (return as JSON):
{
  "field1": "<description>",
  "field2": "<description>",
  "nested": {
    "subfield": "<description>"
  }
}

ERROR HANDLING:
- If X fails, do Y
- If field not found, set to "N/A"

CONSTRAINTS:
- Do NOT [prohibited action]
- Stay on [specific page/domain]
- Do NOT attempt to [blocked action]
```

## Key Principles

### 1. Explicit Objectives
BAD: "Search for Go and click results"
GOOD: "OBJECTIVE: Search for 'Go programming language' and navigate to an informational result."

### 2. Numbered Steps
BAD: "Go to the page and get the data"
GOOD:
```
STEPS:
1. Navigate to https://example.com
2. Wait for page to load (look for header element)
3. Extract fields from the sidebar
4. Scroll if needed to find additional info
```

### 3. Explicit Output Schema
BAD: "Return the data as JSON"
GOOD:
```
OUTPUT FORMAT (return as JSON):
{
  "title": "<page title>",
  "metrics": {
    "stars": "<number>",
    "forks": "<number>"
  }
}
```

### 4. Error Handling Instructions
BAD: [No error handling]
GOOD:
```
ERROR HANDLING:
- If CAPTCHA appears, try alternative site (DuckDuckGo)
- If field not visible, set to "N/A"
- If fewer than expected items, return what's available
```

### 5. Clear Constraints
BAD: [No constraints]
GOOD:
```
CONSTRAINTS:
- Do NOT click external links
- Stay on the main page
- Do NOT attempt to log in
- Extract only visible data
```

### 6. Fallback Strategies
BAD: [No fallbacks]
GOOD:
```
FALLBACK:
- If Google blocks, try DuckDuckGo
- If search fails, navigate directly to known URL
- If element not found, describe what you see
```

### 7. Success Criteria
BAD: [No criteria]
GOOD:
```
SUCCESS CRITERIA:
- Navigated to a page about the topic (not search results)
- Extracted at least 3 of the 5 required fields
- Final URL matches expected domain
```

## Tools and When to Use Them

| Tool | Use For | Tips |
|------|---------|------|
| navigate | Going to URLs | Use direct URLs when possible, skip search steps |
| click | Interacting with buttons/links | Specify element purpose, not just index |
| type_text | Filling inputs | Clear what to type, include field identifier |
| scroll | Finding content | Specify direction and purpose |
| wait | Page stability | Explain what you're waiting for |
| extract | Getting data | List specific fields to extract |
| get_page_state | Understanding page | Use for debugging or when confused |
| done | Completing task | Include extracted_data JSON and summary |

## Common Patterns

### Direct Navigation (Preferred)
```
1. Navigate directly to https://en.wikipedia.org/wiki/Rust_(programming_language)
   - This skips the search step and goes straight to the article
```

### Search with Fallback
```
1. Look for the search input field
2. Type: search term
3. Press Enter or click search button
4. Wait for results
5. Find organic result (skip ads marked "Sponsored")
6. Click result

FALLBACK:
- If site blocks request, try alternative (DuckDuckGo)
```

### Data Extraction
```
3. Extract the following from the page:

   FROM THE SIDEBAR:
   - field1: description
   - field2: description

   FROM THE MAIN CONTENT:
   - field3: description

4. If any field is not found, set value to "Not found"
```

### Pagination/Scrolling
```
5. Scroll down to see more content
   - Look for [specific section name]
   - Stop when [condition] is visible
```

## Anti-Patterns to Avoid

1. **Vague objectives**: "Do stuff with the page"
2. **Missing output format**: "Return the data"
3. **No error handling**: Silent failures
4. **Overly complex single prompts**: Break into multiple tasks
5. **Assuming page structure**: Always verify with wait/get_page_state
6. **No constraints**: Agent may do unexpected things
7. **Hard-coded element indices**: Use descriptive purposes instead

## Examples Directory

- `examples/simple/` - Basic search with fallback
- `examples/scraping/` - Structured data extraction
- `examples/multipage/` - Multi-site research workflow
- `examples/research/` - Complex 5-task comprehensive research

## Model: gemini-3-flash-preview

Token limits:
- Input: 1,048,576 tokens
- Output: 65,536 tokens

This allows for very detailed prompts and comprehensive outputs.
