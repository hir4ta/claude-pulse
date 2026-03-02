package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"

	"github.com/hir4ta/claude-pulse/internal/embedder"
	"github.com/hir4ta/claude-pulse/internal/guard"
	"github.com/hir4ta/claude-pulse/internal/mcpserver"
	"github.com/hir4ta/claude-pulse/internal/store"
)

// version is set at build time via ldflags (-X main.version=...).
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cmd := ""
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "serve":
		return runServe()
	case "hook":
		if len(os.Args) < 3 {
			return fmt.Errorf("usage: pulse hook <EventName>")
		}
		return runHook(os.Args[2])
	case "seed-presets":
		return runSeedPresets()
	case "version", "--version", "-v":
		fmt.Printf("pulse %s\n", version)
		return nil
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		if cmd == "" {
			return nil
		}
		return fmt.Errorf("unknown command: %s", cmd)
	}
}

func runServe() error {
	st, err := store.OpenDefault()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer st.Close()

	// Seed default guardrail presets on first run.
	_ = st.SeedPresets(guard.DefaultPresets())

	emb, _ := embedder.NewEmbedder() // nil when VOYAGE_API_KEY is unset

	s := mcpserver.New(st, emb)
	return server.ServeStdio(s)
}

func runSeedPresets() error {
	st, err := store.OpenDefault()
	if err != nil {
		return fmt.Errorf("failed to open store: %w", err)
	}
	defer st.Close()

	return st.SeedPresets(guard.DefaultPresets())
}

func printUsage() {
	fmt.Println(`pulse - Your development health companion for Claude Code

Usage:
  pulse [command]

Commands:
  serve          Run as MCP server (stdio) for Claude Code integration
  hook <Event>   Handle hook events (called by Claude Code)
  seed-presets   Seed default guardrail presets into database
  version        Show version
  help           Show this help

Environment:
  VOYAGE_API_KEY     Optional. Enables semantic vector search.
  PULSE_DEBUG        Enable debug logging (~/.claude-pulse/debug.log).`)
}
