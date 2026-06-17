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

// pickFromList shows a numbered list and returns the chosen item (1-based).
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
