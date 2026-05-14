package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/mcpserver"
	"github.com/bhaskarjha-com/niyantra/internal/readiness"
	"github.com/bhaskarjha-com/niyantra/internal/store"
	"github.com/bhaskarjha-com/niyantra/internal/tracker"
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
	dbPath := fs.String("db", envString("NIYANTRA_DB", defaultDBPath()), "Database path")
	debug := fs.Bool("debug", false, "Enable verbose logging")
	port := fs.Int("port", envInt("NIYANTRA_PORT", 9222), "Dashboard port")
	auth := fs.String("auth", envString("NIYANTRA_AUTH", ""), "HTTP basic auth (user:pass)")
	bind := fs.String("bind", envString("NIYANTRA_BIND", "127.0.0.1"), "Bind address")
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
		cmdServe(logger, *dbPath, *port, *auth, *bind)
	case "mcp":
		cmdMCP(logger, *dbPath)
	case "backup":
		cmdBackup(logger, *dbPath)
	case "restore":
		if fs.NArg() < 1 {
			fmt.Fprintln(os.Stderr, "Usage: niyantra restore <backup-file>")
			os.Exit(1)
		}
		cmdRestore(logger, *dbPath, fs.Arg(0))
	case "demo":
		cmdDemo(logger, *dbPath)
	case "version":
		fmt.Printf("niyantra %s\n", version)
	case "healthcheck":
		cmdHealthcheck(*port)
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

	accountID, err := db.GetOrCreateAccount(snap.Email, snap.PlanName, "antigravity")
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
func cmdServe(logger *slog.Logger, dbPath string, port int, auth string, bind string) {
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	c := client.New(logger)

	srv := web.NewServer(logger, db, c, port, auth, version, bind)
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
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
			os.Exit(1)
		}
	}
}

// cmdMCP starts the MCP server over stdio for AI agent integration.
func cmdMCP(logger *slog.Logger, dbPath string) {
	// Open store read-only (MCP server only queries data)
	s, err := store.Open(dbPath)
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer s.Close()

	t := tracker.New(s, logger)
	t.LoadBaseline()
	srv := mcpserver.New(s, t, logger, version)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	if err := srv.Run(ctx); err != nil {
		logger.Error("MCP server error", "error", err)
		os.Exit(1)
	}
}

// cmdBackup creates a timestamped backup of the database file.
func cmdBackup(logger *slog.Logger, dbPath string) {
	// Check source exists
	info, err := os.Stat(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Database not found: %s\n", dbPath)
		os.Exit(1)
	}

	dir := filepath.Dir(dbPath)
	backupName := fmt.Sprintf("niyantra-%s.db.bak", time.Now().Format("2006-01-02-150405"))
	backupPath := filepath.Join(dir, backupName)

	// Copy file
	src, err := os.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open database: %v\n", err)
		os.Exit(1)
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create backup: %v\n", err)
		os.Exit(1)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Backup failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Backup created: %s (%d bytes, source: %d bytes)\n", backupPath, written, info.Size())
}

// cmdRestore restores a database from a backup file.
func cmdRestore(logger *slog.Logger, dbPath, backupPath string) {
	// Validate backup exists
	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Backup file not found: %s\n", backupPath)
		os.Exit(1)
	}

	// Validate backup is a valid Niyantra database by checking schema version
	backupStore, err := store.Open(backupPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid backup file: %v\n", err)
		os.Exit(1)
	}
	backupStore.Close()

	// Confirm with user
	fmt.Printf("⚠️  This will replace your current database with:\n")
	fmt.Printf("   %s (%d bytes)\n", backupPath, backupInfo.Size())
	fmt.Printf("   Current DB: %s\n", dbPath)
	fmt.Print("\nType 'yes' to confirm: ")

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	if strings.TrimSpace(scanner.Text()) != "yes" {
		fmt.Println("Restore cancelled.")
		return
	}

	// Copy backup over current DB
	src, err := os.Open(backupPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot open backup: %v\n", err)
		os.Exit(1)
	}
	defer src.Close()

	dst, err := os.Create(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot write database: %v\n", err)
		os.Exit(1)
	}
	defer dst.Close()

	written, err := io.Copy(dst, src)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Restore failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Database restored: %d bytes written to %s\n", written, dbPath)
}

// cmdDemo seeds the database with sample data for evaluation and screenshots.
func cmdDemo(logger *slog.Logger, dbPath string) {
	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Safety check: don't overwrite real data
	if db.AccountCount() > 0 {
		fmt.Println("⚠️  Database already contains data.")
		fmt.Println("   To seed demo data, use a fresh database:")
		fmt.Println("   niyantra demo --db /tmp/demo.db")
		os.Exit(1)
	}

	fmt.Print("\n  🌱 Seeding demo data...\n\n")

	now := time.Now().UTC()

	// --- Accounts & Snapshots ---
	type demoAccount struct {
		email   string
		plan    string
		credits float64
	}
	accounts := []demoAccount{
		{"alex.chen@company.com", "Pro", 1000},
		{"alex.personal@gmail.com", "Ultra", 25000},
	}

	totalSnaps := 0
	for _, acc := range accounts {
		accID, _ := db.GetOrCreateAccount(acc.email, acc.plan, "antigravity")

		// Generate 12 snapshots spanning the last 24 hours
		for i := 11; i >= 0; i-- {
			capturedAt := now.Add(-time.Duration(i) * 2 * time.Hour)

			claudeRemaining := 0.3 + rand.Float64()*0.5
			if i < 3 {
				claudeRemaining = 0.05 + rand.Float64()*0.15
			}
			geminiProRemaining := 0.6 + rand.Float64()*0.35
			geminiFlashRemaining := 0.8 + rand.Float64()*0.2

			resetTime := capturedAt.Add(3*time.Hour + time.Duration(rand.Intn(120))*time.Minute)
			resetTime2 := resetTime.Add(1 * time.Hour)
			resetTime3 := resetTime.Add(2 * time.Hour)

			models := []client.ModelQuota{
				{
					Label:             "Claude Sonnet 4.6 (Thinking)",
					RemainingFraction: claudeRemaining,
					ResetTime:         &resetTime,
				},
				{
					Label:             "GPT-4.1",
					RemainingFraction: claudeRemaining * 0.9,
					ResetTime:         &resetTime,
				},
				{
					Label:             "Gemini 2.5 Pro",
					RemainingFraction: geminiProRemaining,
					ResetTime:         &resetTime2,
				},
				{
					Label:             "Gemini 2.5 Flash",
					RemainingFraction: geminiFlashRemaining,
					ResetTime:         &resetTime3,
				},
			}

			snap := &client.Snapshot{
				AccountID:     accID,
				CapturedAt:    capturedAt,
				Email:         acc.email,
				PlanName:      acc.plan,
				PromptCredits: 500, // Legacy fallback
				AICredits: []client.AICredit{
					{
						CreditType:      "AI_CREDIT",
						CreditAmount:    acc.credits * (1.0 - (float64(11-i) * 0.02)), // Simulate 2% burn per snapshot
						MinimumForUsage: 0,
					},
				},
				Models:        models,
				CaptureMethod: "auto",
				CaptureSource: "server",
				SourceID:      "antigravity",
			}

			snapID, err := db.InsertSnapshot(snap)
			if err == nil {
				totalSnaps++
				if i == 0 {
					db.LogInfoSnap("server", "snap", acc.email, snapID, map[string]interface{}{
						"plan": acc.plan, "method": "auto", "source": "server",
					})
				}
			}
		}
	}
	fmt.Printf("  ✓ Created %d accounts\n", len(accounts))
	fmt.Printf("  ✓ Inserted %d quota snapshots\n", totalSnaps)

	// --- Subscriptions ---
	type demoSub struct {
		platform, category, plan, status, email, cycle, currency string
		cost                                                     float64
		dashURL, statusURL                                       string
	}
	subs := []demoSub{
		{"Antigravity", "coding", "Pro", "active", "alex.chen@company.com", "monthly", "USD", 15.00,
			"https://antigravity.google", "https://statusgator.com/services/google-antigravity"},
		{"Claude", "coding", "Pro", "active", "alex.personal@gmail.com", "monthly", "USD", 20.00,
			"https://console.anthropic.com", "https://status.anthropic.com"},
		{"ChatGPT", "chat", "Plus", "active", "alex.personal@gmail.com", "monthly", "USD", 20.00,
			"https://chat.openai.com", "https://status.openai.com"},
		{"Cursor", "coding", "Pro", "active", "alex.chen@company.com", "monthly", "USD", 20.00,
			"https://cursor.sh/settings", ""},
		{"Midjourney", "image", "Standard", "active", "alex.personal@gmail.com", "monthly", "USD", 30.00,
			"https://midjourney.com/account", ""},
	}

	for _, s := range subs {
		renewal := now.AddDate(0, 0, 7+rand.Intn(21))
		sub := &store.Subscription{
			Platform:      s.platform,
			Category:      s.category,
			PlanName:      s.plan,
			Status:        s.status,
			CostAmount:    s.cost,
			CostCurrency:  s.currency,
			BillingCycle:  s.cycle,
			Email:         s.email,
			NextRenewal:   renewal.Format("2006-01-02"),
			URL:           s.dashURL,
			StatusPageURL: s.statusURL,
		}
		db.InsertSubscription(sub)
	}
	fmt.Printf("  ✓ Created %d subscriptions\n", len(subs))

	// --- Config ---
	db.SetConfig("budget_monthly", "150")
	db.SetConfig("currency", "USD")
	db.SetConfig("retention_days", "365")
	fmt.Println("  ✓ Set budget to $150/mo")

	// --- Activity Log ---
	db.LogInfo("system", "server_start", "", map[string]interface{}{
		"port": 9222, "mode": "auto", "version": version,
	})
	db.LogInfo("system", "config_change", "", map[string]interface{}{
		"key": "budget_monthly", "from": "0", "to": "150",
	})
	fmt.Println("  ✓ Logged activity events")

	fmt.Print("\n  ✅ Demo data ready. Run 'niyantra serve' to explore the dashboard.\n\n")
}

func printUsage() {
	fmt.Println(`Niyantra — Multi-account quota ledger for Antigravity

Usage:
  niyantra <command> [flags]

Commands:
  snap       Capture current account's quota (1 API call)
  status     Show all accounts' readiness (0 network calls)
  serve      Start the web dashboard
  mcp        Start MCP server (stdio) for AI agent integration
  demo       Seed database with sample data for evaluation
  backup     Create a database backup
  restore    Restore database from a backup file
  healthcheck  Docker health probe (GET /healthz)
  version    Print version information

Flags:
  --port     Dashboard port (default: 9222)
  --bind     Bind address (default: 127.0.0.1)
  --db       Database path (default: ~/.niyantra/niyantra.db)
  --auth     HTTP basic auth for dashboard (user:pass)
  --debug    Enable verbose logging

Environment Variables:
  NIYANTRA_PORT   Dashboard port (overridden by --port)
  NIYANTRA_BIND   Bind address (overridden by --bind)
  NIYANTRA_DB     Database path (overridden by --db)
  NIYANTRA_AUTH   HTTP basic auth (overridden by --auth)`)
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

// envString returns the value of an environment variable, or fallback if unset.
func envString(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envInt returns the integer value of an environment variable, or fallback if unset/invalid.
func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

// cmdHealthcheck performs a Docker health probe by hitting /healthz on localhost.
// Used in distroless containers where curl/wget are unavailable.
func cmdHealthcheck(port int) {
	url := fmt.Sprintf("http://127.0.0.1:%d/healthz", port)
	client := &http.Client{Timeout: 5 * time.Second}

	resp, err := client.Get(url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "healthcheck failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "healthcheck failed: HTTP %d\n", resp.StatusCode)
		os.Exit(1)
	}

	os.Exit(0)
}
