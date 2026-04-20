// Script to extract the most recent RAW JSON payload from Niyantra.
// Run this anytime from the Niyantra directory:
//   go run scripts/dump_antigravity_payload.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bhaskarjha-com/niyantra/internal/client"
	"github.com/bhaskarjha-com/niyantra/internal/store"
)

func main() {
	fmt.Println("Locating Niyantra database...")
	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		userProfile = os.Getenv("HOME")
	}
	
	dbPath := filepath.Join(userProfile, ".niyantra", "niyantra.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("Error: Database not found at %s. Please run 'niyantra snap' first.\n", dbPath)
		os.Exit(1)
	}

	db, err := store.Open(dbPath)
	if err != nil {
		fmt.Printf("Error: Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Use LatestPerAccount (or History) to retrieve records
	snaps, err := db.LatestPerAccount()
	if err != nil {
		fmt.Printf("Error: Failed to query database: %v\n", err)
		os.Exit(1)
	}

	if len(snaps) == 0 {
		fmt.Println("Error: No snapshots found in database.")
		os.Exit(1)
	}

	// We'll iterate to pick the absolute most recent snapshot among all accounts
	var latest *client.Snapshot
	for _, s := range snaps {
		if latest == nil || s.CapturedAt.After(latest.CapturedAt) {
			latest = s
		}
	}

	if latest == nil || latest.RawJSON == "" {
		fmt.Println("Error: No raw JSON data available in the latest snapshot.")
		// Wait, we can explain why:
		fmt.Println("Note: Only snapshots captured from this point onwards will contain unmarshalled Raw JSON strings.")
		fmt.Println("Run 'niyantra snap' to generate a fresh raw snapshot.")
		os.Exit(1)
	}

	var out bytes.Buffer
	err = json.Indent(&out, []byte(latest.RawJSON), "", "  ")
	if err != nil {
		fmt.Printf("Error: Failed to format JSON: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=======================================================\n")
	fmt.Printf("Raw JSON For %s\n", latest.Email)
	fmt.Printf("Captured At: %s (%s)\n", latest.CapturedAt.Format(time.RFC1123), latest.CaptureMethod)
	fmt.Printf("=======================================================\n\n")

	filename := "antigravity_payload_dump.json"
	err = os.WriteFile(filename, out.Bytes(), 0644)
	if err != nil {
		fmt.Printf("Error: Failed to save JSON to file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Payload successfully saved to: %s\n", filename)
}
