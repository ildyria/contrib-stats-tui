// Command contributors renders a GitHub-style contributors page and
// contribution calendar for a git repository as a terminal UI.
package main

import (
	"os"

	"github.com/ildyria/contrib-stats-tui/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
