// Package readiness computes account readiness from stored snapshots.
//
// All computation is purely local — zero network calls, zero I/O.
// Input: latest snapshot per account.
// Output: AccountReadiness with per-group status, staleness, reset countdowns,
// and LatestSnapshotID (used by Quick Adjust to identify the target snapshot).
package readiness
