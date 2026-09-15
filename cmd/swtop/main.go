package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/dbohry/swtop/internal/collector"
	"github.com/dbohry/swtop/internal/config"
	"github.com/dbohry/swtop/internal/tui"
)

var version = "dev"

func defaultConfigPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".config", "swtop", "config.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "swtop.yaml"
}

func main() {
	configPath := flag.String("config", defaultConfigPath(), "path to swtop config YAML")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("swtop " + version)
		return
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "swtop: %v\n", err)
		os.Exit(1)
	}

	c := collector.New(cfg)
	stop := make(chan struct{})
	go c.Run(stop)
	defer close(stop)

	m := tui.New(c.Snapshots())
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "swtop: %v\n", err)
		os.Exit(1)
	}
}
