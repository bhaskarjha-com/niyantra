// Package client provides the Antigravity language server API client.
//
// It auto-detects the running Antigravity language server process,
// discovers its listening port, and fetches quota information via
// the Connect RPC protocol.
//
// This package makes exactly ONE network call per FetchQuotas invocation,
// to localhost only (127.0.0.1). No external network traffic.
//
// The LS maintains its own cache (~60-120s refresh cycle). Data is always
// for the correct current account. Users can fine-tune stale values via
// the Quick Adjust feature after snapping (PATCH /api/snap/adjust).
package client
