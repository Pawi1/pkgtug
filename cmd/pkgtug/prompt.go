package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

var stdinReader = bufio.NewReader(os.Stdin)

// prompt prints the question with an optional default and reads a line from stdin.
// If the user presses Enter with no input, defaultVal is returned.
func prompt(question, defaultVal string) string {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", question, defaultVal)
	} else {
		fmt.Printf("%s: ", question)
	}
	line, _ := stdinReader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal
	}
	return line
}

// promptOptional is like prompt but returns "" when the user skips (no default shown).
func promptOptional(question string) string {
	fmt.Printf("%s (Enter to skip): ", question)
	line, _ := stdinReader.ReadString('\n')
	return strings.TrimSpace(line)
}

// pickFromList shows a numbered list and returns the index of the chosen item.
func pickFromList(header string, items []string) (int, error) {
	fmt.Println(header)
	for i, item := range items {
		fmt.Printf("  %d) %s\n", i+1, item)
	}
	for {
		raw := prompt("Select", "1")
		var n int
		if _, err := fmt.Sscan(raw, &n); err == nil && n >= 1 && n <= len(items) {
			return n - 1, nil
		}
		fmt.Printf("  enter a number between 1 and %d\n", len(items))
	}
}

// pickFromListOptional shows a numbered list with a "skip" option (0).
// Returns ("", false) when skipped, (item, true) when chosen.
func pickFromListOptional(header string, items []string) (string, bool) {
	fmt.Println(header)
	fmt.Println("  0) (skip)")
	for i, item := range items {
		fmt.Printf("  %d) %s\n", i+1, item)
	}
	for {
		raw := prompt("Select", "0")
		var n int
		if _, err := fmt.Sscan(raw, &n); err == nil && n >= 0 && n <= len(items) {
			if n == 0 {
				return "", false
			}
			return items[n-1], true
		}
		fmt.Printf("  enter a number between 0 and %d\n", len(items))
	}
}

// pickMultiFromList shows a numbered list; user enters comma-separated indices.
// Returns the selected items. Empty input = none selected.
func pickMultiFromList(header string, items []string) []string {
	if len(items) == 0 {
		return nil
	}
	fmt.Println(header)
	for i, item := range items {
		fmt.Printf("  %d) %s\n", i+1, item)
	}
	fmt.Print("Select (comma-separated numbers, Enter to skip): ")
	line, _ := stdinReader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	var result []string
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		var n int
		if _, err := fmt.Sscan(part, &n); err == nil && n >= 1 && n <= len(items) {
			result = append(result, items[n-1])
		}
	}
	return result
}
