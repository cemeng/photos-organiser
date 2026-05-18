package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: importer <source-directory>\n")
		os.Exit(1)
	}

	source := NormaliseDir(os.Args[1])

	info, err := os.Stat(source)
	if err != nil || !info.IsDir() {
		fmt.Fprintf(os.Stderr, "Error: %q is not a valid directory\n", source)
		os.Exit(1)
	}

	p := tea.NewProgram(newModel(source), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
