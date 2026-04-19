package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/web"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(0)
	}

	cmd := os.Args[1]

	// Global flags
	fs := flag.NewFlagSet("niyantra", flag.ExitOnError)
	dbPath := fs.String("db", defaultDBPath(), "Database path")
	debug := fs.Bool("debug", false, "Enable verbose logging")
	port := fs.Int("port", 9222, "Dashboard port")
	auth := fs.String("auth", "", "HTTP basic auth (user:pass)")
	fs.Parse(os.Args[2:])

	// Logger
	level := slog.LevelInfo
	if *debug {
		level = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))

	switch cmd {
	case "snap":
		cmdSnap(logger, *dbPath)
	case "status":
		cmdStatus(logger, *dbPath)
	case "serve":
		cmdServe(logger, *dbPath, *port, *auth)
	case "version":
		fmt.Printf("niyantra %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

// cmdSnap captures the current Antigravity account's quota.
func cmdSnap(logger *slog.Logger, dbPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Detect and fetch
	c := client.New(logger)
	fmt.Print("⏳ Detecting Antigravity language server... ")

	resp, err := c.FetchQuotas(ctx)
	if err != nil {
		fmt.Println("❌")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅")

	// 2. Convert to snapshot
	snap := resp.ToSnapshot(time.Now().UTC())

	// Tag provenance: captured via CLI
	snap.CaptureMethod = "manual"
	snap.CaptureSource = "cli"
	snap.SourceID = "antigravity"

	// 3. Store
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	accountID, err := db.GetOrCreateAccount(snap.Email, snap.PlanName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating account: %v\n", err)
		os.Exit(1)
	}
	snap.AccountID = accountID

	snapID, err := db.InsertSnapshot(snap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error storing snapshot: %v\n", err)
		os.Exit(1)
	}

	// Log successful snap
	db.LogInfoSnap("cli", "snap", snap.Email, snapID, map[string]interface{}{
		"plan": snap.PlanName, "method": "manual", "source": "cli",
	})
	db.UpdateSourceCapture("antigravity")

	// 4. Display result
	groups := client.GroupModels(snap.Models)

	fmt.Println()
	fmt.Printf("  📸 Snapshot #%d captured\n", snapID)
	fmt.Printf("  📧 %s (%s)\n", snap.Email, snap.PlanName)
	fmt.Println()

	for _, g := range groups {
		pct := g.RemainingFraction * 100
		status := "✅"
		if g.IsExhausted || pct == 0 {
			status = "❌"
		} else if pct < 20 {
			status = "⚠️"
		}

		resetStr := ""
		if g.TimeUntilReset > 0 {
			resetStr = fmt.Sprintf("  ↻ %s", formatDuration(g.TimeUntilReset))
		}
		fmt.Printf("  %s %-16s %5.1f%%%s\n", status, g.DisplayName+":", pct, resetStr)
	}
	fmt.Println()
}

// cmdStatus shows readiness for all tracked accounts.
func cmdStatus(logger *slog.Logger, dbPath string) {
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	snapshots, err := db.LatestPerAccount()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading snapshots: %v\n", err)
		os.Exit(1)
	}

	if len(snapshots) == 0 {
		fmt.Println("No accounts tracked yet. Run 'niyantra snap' to capture your first snapshot.")
		return
	}

	accounts := readiness.Calculate(snapshots, 0.0)

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║  NIYANTRA — Multi-Account Readiness                         ║")
	fmt.Println("  ╠══════════════════════════════════════════════════════════════╣")

	for i, acc := range accounts {
		status := "✅"
		if !acc.IsReady {
			status = "⚠️"
		}

		fmt.Printf("  ║  %-40s %-12s %s  ║\n",
			truncate(acc.Email, 40),
			acc.StalenessLabel,
			status,
		)

		if acc.PlanName != "" {
			fmt.Printf("  ║    Plan: %-50s  ║\n", acc.PlanName)
		}

		for _, g := range acc.Groups {
			pct := g.RemainingPercent
			indicator := "✅"
			if g.IsExhausted || pct == 0 {
				indicator = "❌"
			} else if pct < 20 {
				indicator = "⚠️"
			}

			resetStr := ""
			if g.TimeUntilResetSec > 0 {
				resetStr = fmt.Sprintf("  ↻ %s", formatDuration(time.Duration(g.TimeUntilResetSec*float64(time.Second))))
			}

			fmt.Printf("  ║    %s %-14s %5.1f%%%s%s  ║\n",
				indicator,
				g.DisplayName+":",
				pct,
				resetStr,
				strings.Repeat(" ", maxInt(0, 30-len(resetStr)-6)),
			)
		}

		if i < len(accounts)-1 {
			fmt.Println("  ║                                                              ║")
		}
	}

	fmt.Println("  ╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("\n  %d accounts · %d snapshots · db: %s\n\n",
		db.AccountCount(), db.SnapshotCount(), dbPath)
}

// cmdServe starts the web dashboard.
func cmdServe(logger *slog.Logger, dbPath string, port int, auth string) {
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	c := client.New(logger)

	srv := web.NewServer(logger, db, c, port, auth)
	defer srv.Shutdown()

	autoCapture := db.GetConfigBool("auto_capture")
	mode := "manual"
	if autoCapture {
		mode = "auto"
	}
	pollInterval := db.GetConfigInt("poll_interval", 300)

	// Log server start
	db.LogInfo("system", "server_start", "", map[string]interface{}{
		"port": port, "mode": mode, "version": version,
		"autoCapture": autoCapture, "pollInterval": pollInterval,
	})

	fmt.Println()
	fmt.Println("  ╔══════════════════════════════════════╗")
	fmt.Printf("  ║  Niyantra %-27s ║\n", version)
	fmt.Println("  ╠══════════════════════════════════════╣")
	fmt.Printf("  ║  Dashboard: http://localhost:%-8d ║\n", port)
	fmt.Printf("  ║  Database:  %-26s ║\n", truncate(dbPath, 26))
	fmt.Printf("  ║  Mode:      %-26s ║\n", mode)
	if autoCapture {
		fmt.Printf("  ║  Polling:   every %-20s ║\n", fmt.Sprintf("%ds", pollInterval))
	}
	if auth != "" {
		fmt.Println("  ║  Auth:      enabled                  ║")
	}
	fmt.Println("  ╚══════════════════════════════════════╝")
	fmt.Println()

	// Handle shutdown signals
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	serverErr := make(chan error, 1)
	go func() {
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		fmt.Println("\n  Shutting down gracefully...")
		srv.Shutdown()
	case err := <-serverErr:
		if err != nil {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Println(`Niyantra — Multi-account quota ledger for Antigravity

Usage:
  niyantra <command> [flags]

Commands:
  snap       Capture current account's quota (1 API call)
  status     Show all accounts' readiness (0 network calls)
  serve      Start the web dashboard
  version    Print version information

Flags:
  --port     Dashboard port (default: 9222)
  --db       Database path (default: ~/.niyantra/niyantra.db)
  --auth     HTTP basic auth for dashboard (user:pass)
  --debug    Enable verbose logging`)
}

func defaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".niyantra", "niyantra.db")
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	if h >= 24 {
		return fmt.Sprintf("%dd %dh", h/24, h%24)
	}
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
