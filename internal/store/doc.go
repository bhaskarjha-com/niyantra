// Package store provides SQLite-based persistence for Niyantra.
//
// It manages two tables:
//   - accounts: unique account identities keyed by email
//   - snapshots: point-in-time quota captures tagged by account
//
// All operations are synchronous and use a single database connection.
// The database is created automatically on first use.
package store
