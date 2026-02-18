package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ninetyfive/p95/internal/domain"
	"github.com/ninetyfive/p95/internal/server"
	"github.com/ninetyfive/p95/internal/storage/file"
	"github.com/ninetyfive/p95/internal/tui"
	"github.com/ninetyfive/p95/pkg/client"
	"github.com/ninetyfive/p95/web"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]

	switch cmd {
	case "serve", "server":
		serveCmd(os.Args[2:])
	case "ls", "list":
		listCmd(os.Args[2:])
	case "show":
		showCmd(os.Args[2:])
	case "tui":
		tuiCmd(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Println("pnf v0.1.0")
	case "help", "--help", "-h":
		printUsage()
	default:
		// If first arg looks like a flag, assume serve command
		if strings.HasPrefix(cmd, "-") {
			serveCmd(os.Args[1:])
		} else {
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", cmd)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println(`pnf - ML experiment tracking

Usage:
  pnf <command> [options]

Commands:
  tui     Interactive terminal UI with charts
  ls      List projects and runs
  show    Show metrics for a run
  serve   Start the web viewer

Examples:
  pnf tui --logdir ./logs
  pnf ls --logdir ./logs
  pnf ls --logdir ./logs --project demo-project
  pnf show <run-id> --logdir ./logs
  pnf serve --logdir ./logs

Options:
  --logdir    Directory containing logs (default: ~/.p95/logs)
  --help      Show this help message`)
}

// ============================================
// ls command - list projects and runs
// ============================================

func listCmd(args []string) {
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	logdir := fs.String("logdir", "", "Directory containing logs")
	project := fs.String("project", "", "Filter by project name")
	fs.Parse(args)

	if *logdir == "" {
		*logdir = defaultLogDir()
	}
	*logdir = expandPath(*logdir)

	store, err := file.New(*logdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()

	if *project != "" {
		// List runs for a specific project
		listRuns(ctx, store, *project)
	} else {
		// List all projects
		listProjects(ctx, store)
	}
}

func listProjects(ctx context.Context, store *file.Storage) {
	projects, err := store.ListProjects(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing projects: %v\n", err)
		os.Exit(1)
	}

	if len(projects) == 0 {
		fmt.Println("No projects found.")
		fmt.Printf("Logdir: %s\n", store.LogDir())
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PROJECT\tRUNS\tLAST UPDATED")
	fmt.Fprintln(w, "-------\t----\t------------")

	for _, p := range projects {
		// Get runs to count and find last updated
		runs, _ := store.ListRuns(ctx, p.Slug, domain.RunListOptions{Limit: 100})
		lastUpdated := "-"
		if len(runs) > 0 && !runs[0].StartedAt.IsZero() {
			lastUpdated = runs[0].StartedAt.Format("2006-01-02 15:04")
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n", p.Name, len(runs), lastUpdated)
	}
	w.Flush()
}

func listRuns(ctx context.Context, store *file.Storage, project string) {
	runs, err := store.ListRuns(ctx, project, domain.RunListOptions{
		Limit:    100,
		OrderBy:  "started_at",
		OrderDir: "desc",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing runs: %v\n", err)
		os.Exit(1)
	}

	if len(runs) == 0 {
		fmt.Printf("No runs found in project '%s'.\n", project)
		return
	}

	fmt.Printf("Project: %s (%d runs)\n\n", project, len(runs))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RUN ID\tNAME\tSTATUS\tCREATED")
	fmt.Fprintln(w, "------\t----\t------\t-------")

	for _, r := range runs {
		created := "-"
		if !r.StartedAt.IsZero() {
			created = r.StartedAt.Format("2006-01-02 15:04")
		}
		// Truncate run ID for display
		idStr := r.ID.String()
		shortID := idStr
		if len(idStr) > 12 {
			shortID = idStr[:12]
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", shortID, r.Name, r.Status, created)
	}
	w.Flush()
}

// ============================================
// show command - show metrics for a run
// ============================================

func showCmd(args []string) {
	// Extract positional args (non-flag args) before parsing,
	// because Go's flag package stops at the first non-flag argument.
	var flagArgs []string
	var positional []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--logdir" && i+1 < len(args) {
			flagArgs = append(flagArgs, args[i], args[i+1])
			i++
		} else if len(args[i]) > 0 && args[i][0] == '-' {
			flagArgs = append(flagArgs, args[i])
		} else {
			positional = append(positional, args[i])
		}
	}

	fs := flag.NewFlagSet("show", flag.ExitOnError)
	logdir := fs.String("logdir", "", "Directory containing logs")
	fs.Parse(flagArgs)

	if *logdir == "" {
		*logdir = defaultLogDir()
	}
	*logdir = expandPath(*logdir)

	if len(positional) == 0 {
		fmt.Fprintf(os.Stderr, "Error: run ID required\n")
		fmt.Fprintf(os.Stderr, "Usage: pnf show <run-id> --logdir ./logs\n")
		os.Exit(1)
	}
	runID := positional[0]

	store, err := file.New(*logdir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	ctx := context.Background()

	// Get run info
	run, err := store.GetRun(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	runID = run.ID.String()

	// Print run header
	fmt.Printf("Run: %s\n", run.Name)
	fmt.Printf("ID:  %s\n", run.ID.String())
	fmt.Printf("Status: %s\n", run.Status)
	if !run.StartedAt.IsZero() {
		fmt.Printf("Started: %s\n", run.StartedAt.Format("2006-01-02 15:04:05"))
	}
	fmt.Println()

	// Print config if available
	if len(run.Config) > 0 {
		fmt.Println("Config:")
		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		keys := make([]string, 0, len(run.Config))
		for k := range run.Config {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(w, "  %s:\t%v\n", k, run.Config[k])
		}
		w.Flush()
		fmt.Println()
	}

	// Print tags if available
	if len(run.Tags) > 0 {
		fmt.Printf("Tags: %s\n\n", strings.Join(run.Tags, ", "))
	}

	// Get and print metrics
	metricNames, err := store.GetMetricNames(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting metrics: %v\n", err)
		os.Exit(1)
	}

	if len(metricNames) == 0 {
		fmt.Println("No metrics recorded.")
		return
	}

	// Get latest values
	latest, err := store.GetLatestMetrics(ctx, runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting latest metrics: %v\n", err)
		os.Exit(1)
	}

	// Get summary for min/max - convert to map for easy lookup
	summaryData, _ := store.GetMetricsSummary(ctx, runID)
	summaryMap := make(map[string]struct {
		Min   float64
		Max   float64
		Count int64
	})
	if summaryData != nil {
		for _, m := range summaryData.Metrics {
			summaryMap[m.Name] = struct {
				Min   float64
				Max   float64
				Count int64
			}{m.MinValue, m.MaxValue, m.Count}
		}
	}

	fmt.Println("Metrics:")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tLATEST\tMIN\tMAX\tCOUNT")
	fmt.Fprintln(w, "  ----\t------\t---\t---\t-----")

	sort.Strings(metricNames)
	for _, name := range metricNames {
		latestVal := "-"
		if v, ok := latest[name]; ok {
			latestVal = fmt.Sprintf("%.4f", v)
		}

		minVal, maxVal, count := "-", "-", "-"
		if s, ok := summaryMap[name]; ok {
			minVal = fmt.Sprintf("%.4f", s.Min)
			maxVal = fmt.Sprintf("%.4f", s.Max)
			count = fmt.Sprintf("%d", s.Count)
		}

		fmt.Fprintf(w, "  %s\t%s\t%s\t%s\t%s\n", name, latestVal, minVal, maxVal, count)
	}
	w.Flush()
}

// ============================================
// tui command - interactive terminal UI
// ============================================

func tuiCmd(args []string) {
	fs := flag.NewFlagSet("tui", flag.ExitOnError)
	logdir := fs.String("logdir", "", "Directory containing logs")
	fs.Parse(args)

	if *logdir == "" {
		*logdir = defaultLogDir()
	}
	*logdir = expandPath(*logdir)

	// Disable all logging - it breaks the TUI
	log.SetOutput(io.Discard)

	// Find an available port for the local server
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Start local server in background (no web UI needed for TUI mode)
	srv, err := server.New(*logdir, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating server: %v\n", err)
		os.Exit(1)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", port)
	go func() {
		srv.Start(addr)
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Create API client pointing at local server
	apiClient := client.New(fmt.Sprintf("http://%s", addr))

	// Create and run TUI
	app := tui.New(apiClient)
	p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseAllMotion())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	// Shutdown server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}

// ============================================
// serve command - start web server
// ============================================

func serveCmd(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	logdir := fs.String("logdir", "", "Directory containing logs (default: platform-specific)")
	port := fs.Int("port", 6767, "Port to listen on")
	host := fs.String("host", "localhost", "Host to bind to")
	openBrowser := fs.Bool("open", true, "Open browser automatically")
	fs.Parse(args)

	if *logdir == "" {
		*logdir = defaultLogDir()
	}
	*logdir = expandPath(*logdir)

	// Ensure logdir exists
	if err := os.MkdirAll(*logdir, 0755); err != nil {
		log.Fatalf("Failed to create logdir: %v", err)
	}

	// Use embedded web UI if available
	webFS := web.DistFS()

	// Create server
	srv, err := server.New(*logdir, webFS)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	addr := fmt.Sprintf("%s:%d", *host, *port)

	// Start server in goroutine
	go func() {
		log.Printf("p95 local viewer")
		log.Printf("  Logdir: %s", *logdir)
		log.Printf("  URL:    http://%s", addr)
		log.Println()
		log.Println("Waiting for runs... (use Ctrl+C to stop)")

		if err := srv.Start(addr); err != nil && err != http.ErrServerClosed {
			log.Printf("Server error: %v", err)
		}
	}()

	// Open browser if requested
	if *openBrowser {
		go func() {
			time.Sleep(500 * time.Millisecond)
			openURL(fmt.Sprintf("http://%s", addr))
		}()
	}

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\nShutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}
}

// ============================================
// Helper functions
// ============================================

func defaultLogDir() string {
	home, _ := os.UserHomeDir()

	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "p95", "logs")
	case "windows":
		appdata := os.Getenv("LOCALAPPDATA")
		if appdata == "" {
			appdata = filepath.Join(home, "AppData", "Local")
		}
		return filepath.Join(appdata, "p95", "logs")
	default: // Linux and others
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(home, ".local", "share")
		}
		return filepath.Join(xdgData, "p95", "logs")
	}
}

func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func openURL(url string) {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default: // Linux
		cmd = exec.Command("xdg-open", url)
	}

	// Best-effort - ignore errors
	_ = cmd.Start()
}
