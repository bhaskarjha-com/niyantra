// Package readiness computes account readiness from stored snapshots.
//
// All computation is purely local — zero network calls, zero I/O.
// Input: latest snapshot per account.
// Output: readiness state with per-group status, staleness, reset countdowns.
package readiness
