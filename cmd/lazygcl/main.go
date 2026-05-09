package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/cozy-corner/lazygcl/internal/gcp"
	"github.com/cozy-corner/lazygcl/internal/tui"
)

const version = "0.0.1"

func main() {
	project := flag.String("project", "", "GCP project ID (overrides GOOGLE_CLOUD_PROJECT and gcloud config)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("lazygcl", version)
		return
	}

	resolved, err := resolveProject(*project)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazygcl:", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client, err := gcp.NewClient(ctx, resolved)
	if err != nil {
		fmt.Fprintln(os.Stderr, "lazygcl: failed to create logging client:", err)
		os.Exit(1)
	}
	defer func() { _ = client.Close() }()

	model := tui.NewModel(tui.Options{Client: client, Project: resolved})
	if _, err := tea.NewProgram(model, tea.WithAltScreen()).Run(); err != nil {
		fmt.Fprintln(os.Stderr, "lazygcl:", err)
		os.Exit(1)
	}
}

// resolveProject picks a project ID from the flag, then GOOGLE_CLOUD_PROJECT,
// then `gcloud config get-value project`. Returns an error if none yields a value.
func resolveProject(flagVal string) (string, error) {
	if flagVal != "" {
		return flagVal, nil
	}
	if env := os.Getenv("GOOGLE_CLOUD_PROJECT"); env != "" {
		return env, nil
	}
	out, err := exec.Command("gcloud", "config", "get-value", "project").Output()
	if err == nil {
		if id := strings.TrimSpace(string(out)); id != "" && id != "(unset)" {
			return id, nil
		}
	}
	return "", fmt.Errorf("no project set: pass --project, set GOOGLE_CLOUD_PROJECT, or run `gcloud config set project <id>`")
}
