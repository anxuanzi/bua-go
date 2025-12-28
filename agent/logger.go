// Package agent provides the ADK-based browser automation agent.
package agent

import (
	"fmt"
	"strings"
	"time"
)

// LogLevel represents the logging level.
type LogLevel int

const (
	LogDebug LogLevel = iota
	LogInfo
	LogWarn
	LogError
	LogAction
)

// Logger provides structured logging with emojis and formatting.
type Logger struct {
	enabled   bool
	stepCount int
}

// NewLogger creates a new logger.
func NewLogger(enabled bool) *Logger {
	return &Logger{
		enabled:   enabled,
		stepCount: 0,
	}
}

// IncrementStep increments the step counter.
func (l *Logger) IncrementStep() int {
	l.stepCount++
	return l.stepCount
}

// GetStep returns the current step count.
func (l *Logger) GetStep() int {
	return l.stepCount
}

// timestamp returns a formatted timestamp.
func timestamp() string {
	return time.Now().Format("15:04:05")
}

// Action logs an action being taken.
func (l *Logger) Action(action, target, reasoning string) {
	if !l.enabled {
		return
	}
	step := l.IncrementStep()
	fmt.Println()
	fmt.Printf("┌─────────────────────────────────────────────────────────────────\n")
	fmt.Printf("│ 🎯 STEP %d │ %s\n", step, timestamp())
	fmt.Printf("├─────────────────────────────────────────────────────────────────\n")
	fmt.Printf("│ 🔧 Action:    %s\n", action)
	if target != "" {
		fmt.Printf("│ 🎪 Target:    %s\n", target)
	}
	if reasoning != "" {
		fmt.Printf("│ 💭 Reasoning: %s\n", truncate(reasoning, 60))
	}
	fmt.Printf("└─────────────────────────────────────────────────────────────────\n")
}

// ActionResult logs the result of an action.
func (l *Logger) ActionResult(success bool, message string) {
	if !l.enabled {
		return
	}
	if success {
		fmt.Printf("   ✅ %s\n", message)
	} else {
		fmt.Printf("   ❌ %s\n", message)
	}
}

// Navigate logs a navigation action.
func (l *Logger) Navigate(url string) {
	if !l.enabled {
		return
	}
	step := l.IncrementStep()
	fmt.Println()
	fmt.Printf("┌─────────────────────────────────────────────────────────────────\n")
	fmt.Printf("│ 🌐 STEP %d │ NAVIGATE │ %s\n", step, timestamp())
	fmt.Printf("├─────────────────────────────────────────────────────────────────\n")
	fmt.Printf("│ 📍 URL: %s\n", truncate(url, 55))
	fmt.Printf("└─────────────────────────────────────────────────────────────────\n")
}

// Click logs a click action.
func (l *Logger) Click(elementIndex int, reasoning string) {
	l.Action("CLICK", fmt.Sprintf("Element #%d", elementIndex), reasoning)
}

// Type logs a type action.
func (l *Logger) Type(elementIndex int, text, reasoning string) {
	l.Action("TYPE", fmt.Sprintf("Element #%d → \"%s\"", elementIndex, truncate(text, 30)), reasoning)
}

// Scroll logs a scroll action.
func (l *Logger) Scroll(direction string, amount int, reasoning string) {
	l.Action("SCROLL", fmt.Sprintf("%s %dpx", strings.ToUpper(direction), amount), reasoning)
}

// Wait logs a wait action.
func (l *Logger) Wait(reason string) {
	if !l.enabled {
		return
	}
	fmt.Printf("   ⏳ Waiting: %s\n", reason)
}

// PageState logs page state retrieval.
func (l *Logger) PageState(url, title string, elementCount int) {
	if !l.enabled {
		return
	}
	fmt.Printf("   📄 Page: %s\n", truncate(title, 50))
	fmt.Printf("   🔗 URL:  %s\n", truncate(url, 50))
	fmt.Printf("   🧩 Elements: %d interactive\n", elementCount)
}

// Screenshot logs screenshot capture.
func (l *Logger) Screenshot(path string, annotated bool) {
	if !l.enabled {
		return
	}
	if annotated {
		fmt.Printf("   📸 Screenshot (annotated): %s\n", path)
	} else {
		fmt.Printf("   📸 Screenshot: %s\n", path)
	}
}

// Annotation logs annotation display.
func (l *Logger) Annotation(elementCount int) {
	if !l.enabled {
		return
	}
	fmt.Printf("   🏷️  Showing annotations for %d elements\n", elementCount)
}

// Extract logs data extraction.
func (l *Logger) Extract(what string) {
	l.Action("EXTRACT", what, "")
}

// Done logs task completion.
func (l *Logger) Done(success bool, summary string) {
	if !l.enabled {
		return
	}
	fmt.Println()
	fmt.Printf("╔═════════════════════════════════════════════════════════════════\n")
	if success {
		fmt.Printf("║ ✅ TASK COMPLETED │ %s\n", timestamp())
	} else {
		fmt.Printf("║ ❌ TASK FAILED │ %s\n", timestamp())
	}
	fmt.Printf("╠═════════════════════════════════════════════════════════════════\n")
	fmt.Printf("║ 📝 %s\n", truncate(summary, 60))
	fmt.Printf("╚═════════════════════════════════════════════════════════════════\n")
}

// HumanTakeover logs human takeover request.
func (l *Logger) HumanTakeover(reason string) {
	if !l.enabled {
		return
	}
	fmt.Println()
	fmt.Printf("╔═════════════════════════════════════════════════════════════════\n")
	fmt.Printf("║ 🙋 HUMAN TAKEOVER REQUESTED │ %s\n", timestamp())
	fmt.Printf("╠═════════════════════════════════════════════════════════════════\n")
	fmt.Printf("║ 💬 %s\n", truncate(reason, 60))
	fmt.Printf("╚═════════════════════════════════════════════════════════════════\n")
}

// Event logs ADK events for debugging.
func (l *Logger) Event(author string, partial bool) {
	if !l.enabled {
		return
	}
	partialStr := ""
	if partial {
		partialStr = " (partial)"
	}
	fmt.Printf("   📨 Event from %s%s\n", author, partialStr)
}

// FunctionCall logs function calls.
func (l *Logger) FunctionCall(name string, args map[string]any) {
	if !l.enabled {
		return
	}
	argsStr := formatArgs(args)
	fmt.Printf("   📞 Call: %s(%s)\n", name, truncate(argsStr, 50))
}

// FunctionResponse logs function responses.
func (l *Logger) FunctionResponse(name string, response any) {
	if !l.enabled {
		return
	}
	respStr := fmt.Sprintf("%v", response)
	fmt.Printf("   📬 Response: %s → %s\n", name, truncate(respStr, 50))
}

// Error logs an error.
func (l *Logger) Error(context string, err error) {
	if !l.enabled {
		return
	}
	fmt.Printf("   ⚠️  Error [%s]: %v\n", context, err)
}

// Debug logs debug information.
func (l *Logger) Debug(format string, args ...any) {
	if !l.enabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("   🔍 %s\n", msg)
}

// Info logs informational messages.
func (l *Logger) Info(format string, args ...any) {
	if !l.enabled {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Printf("   ℹ️  %s\n", msg)
}

// truncate truncates a string to maxLen.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// formatArgs formats function arguments for logging.
func formatArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}
	parts := make([]string, 0, len(args))
	for k, v := range args {
		parts = append(parts, fmt.Sprintf("%s=%v", k, v))
	}
	return strings.Join(parts, ", ")
}
