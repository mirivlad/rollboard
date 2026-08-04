// Command validate checks game definitions against the publication rules.
//
// It reads a JSON object of name → definition on standard input and exits
// non-zero if any of them would be refused. scripts/validate-templates.sh
// feeds it the real editor templates, so a template that could not be published
// fails the build rather than a new author's first save.
//
// No database, no server, no HTTP: this is the same validator the API calls,
// which is the point — a second implementation could disagree with the first.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"rollboard/internal/game"
)

func main() {
	var definitions map[string]*game.GameDefinition
	if err := json.NewDecoder(os.Stdin).Decode(&definitions); err != nil {
		fmt.Fprintf(os.Stderr, "could not read definitions: %v\n", err)
		os.Exit(2)
	}
	if len(definitions) == 0 {
		fmt.Fprintln(os.Stderr, "no definitions on stdin")
		os.Exit(2)
	}

	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	failed := 0
	for _, name := range names {
		err := game.ValidateDefinition(definitions[name])
		if err == nil {
			fmt.Printf("  PASS  %s\n", name)
			continue
		}
		failed++
		fmt.Printf("  FAIL  %s\n", name)
		for _, message := range err.Errors {
			fmt.Printf("          %s\n", message)
		}
	}

	fmt.Printf("\n%d checked, %d failed\n", len(names), failed)
	if failed > 0 {
		os.Exit(1)
	}
}
