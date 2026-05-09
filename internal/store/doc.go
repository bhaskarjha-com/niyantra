// Package store provides SQLite-based persistence for Niyantra.
//
// It manages 11 tables across schema v9, including:
//   - accounts: unique account identities keyed by email
//   - snapshots: point-in-time quota captures tagged by account
//   - subscriptions: manual subscription tracking with presets
//   - config, activity_log, data_sources: infrastructure tables
//
// Key operations: InsertSnapshot, UpdateSnapshotModels (Quick Adjust),
// LatestPerAccount, History, retention cleanup.
//
// All operations are synchronous and use a single database connection
// with WAL mode and 5s busy timeout.
package store
