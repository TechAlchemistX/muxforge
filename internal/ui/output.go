// Package ui provides terminal output helpers for consistent formatting.
package ui

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fatih/color"
)

// Prefix symbols used in all output functions.
const (
	PrefixSuccess = "✓"
	PrefixError   = "✗"
	PrefixWarning = "!"
	PrefixInfo    = "→"
	PrefixHint    = "·"
)

func init() {
	// Respect NO_COLOR environment variable.
	if os.Getenv("NO_COLOR") != "" {
		color.NoColor = true
	}
}

var (
	green  = color.New(color.FgGreen)
	red    = color.New(color.FgRed)
	yellow = color.New(color.FgYellow)
	cyan   = color.New(color.FgCyan)
	dim    = color.New(color.Faint)
)

// Success prints a green success message to stdout.
func Success(msg string) {
	fmt.Println(green.Sprint(PrefixSuccess) + " " + msg)
}

// Error prints a red error message to stderr.
func Error(msg string) {
	fmt.Fprintln(os.Stderr, red.Sprint(PrefixError)+" "+msg)
}

// Warning prints a yellow warning message to stdout.
func Warning(msg string) {
	fmt.Println(yellow.Sprint(PrefixWarning) + " " + msg)
}

// Info prints a cyan informational message to stdout.
func Info(msg string) {
	fmt.Println(cyan.Sprint(PrefixInfo) + " " + msg)
}

// Hint prints a dim hint message to stdout.
func Hint(msg string) {
	fmt.Println(dim.Sprint(PrefixHint) + " " + msg)
}

// PrintError prints a three-line structured error message to stderr.
// This format matches the project's error output standard:
//
//	Error: <what>
//	       <why>
//	       <howToFix>
func PrintError(what, why, howToFix string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", red.Sprint("Error:"), what)
	fmt.Fprintf(os.Stderr, "       %s\n", why)
	fmt.Fprintf(os.Stderr, "       %s\n", howToFix)
}

// Spinner displays an animated spinner with a message while a long-running
// operation is in progress.
type Spinner struct {
	mu      sync.Mutex
	msg     string
	done    chan struct{}
	stopped bool
}

// Start begins spinning with the given message. It runs in a background
// goroutine until Stop is called.
func (s *Spinner) Start(msg string) {
	s.mu.Lock()
	s.msg = msg
	s.done = make(chan struct{})
	s.stopped = false
	s.mu.Unlock()

	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	go func() {
		i := 0
		for {
			select {
			case <-s.done:
				// Clear the spinner line.
				fmt.Print("\r\033[K")
				return
			case <-time.After(80 * time.Millisecond):
				s.mu.Lock()
				currentMsg := s.msg
				s.mu.Unlock()
				if color.NoColor {
					fmt.Printf("\r%s %s", frames[i%len(frames)], currentMsg)
				} else {
					fmt.Printf("\r%s %s", cyan.Sprint(frames[i%len(frames)]), currentMsg)
				}
				i++
			}
		}
	}()
}

// Stop halts the spinner and clears the spinner line.
func (s *Spinner) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.stopped && s.done != nil {
		close(s.done)
		s.stopped = true
	}
}
