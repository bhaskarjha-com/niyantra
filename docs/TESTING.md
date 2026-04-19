# Manual Testing Guide: Niyantra

Complete feature checklist for manual verification. Each section has step-by-step test cases with expected results.

**Prerequisites:**
1. Build: `go build -o niyantra.exe ./cmd/niyantra`
2. Run: `.\niyantra.exe serve --debug`
3. Open: `http://localhost:9222`

---

## 1. Dashboard Shell

### 1.1 Page Load
- [ ] Dashboard loads without blank screen or errors
- [ ] Title shows "Niyantra" in browser tab
- [ ] Inter font loads (text should NOT be Times New Roman / serif)
- [ ] Default tab is **Quotas** (highlighted in nav)
- [ ] Dark theme applied by default

### 1.2 Tab Navigation
- [ ] Click **Subscriptions** tab → panel switches, tab highlights
- [ ] Click **Overview** tab → panel switches, tab highlights
- [ ] Click **Settings** tab → panel switches, tab highlights
- [ ] Click **Quotas** tab → returns to quota grid
- [ ] Only one tab is highlighted at a time

### 1.3 Theme Toggle
- [ ] Click moon/sun icon in header → switches to light theme
- [ ] All cards, modals, inputs update colors correctly
- [ ] Click again → switches back to dark
- [ ] Refresh page → theme persists (localStorage)

---

## 2. Quotas Tab (Auto-Tracked)

### 2.1 Empty State
- [ ] If no Antigravity detected: quota grid shows empty or error toast
- [ ] Account count badge shows `0`

### 2.2 Snap
- [ ] Click **Snap Now** button
- [ ] If Antigravity is running: toast shows "✅ Snapshot captured" with email
- [ ] If NOT running: toast shows error "Could not detect..." (red)
- [ ] Account row appears in grid after successful snap

### 2.3 Account Grid (requires at least 1 snap)
- [ ] Row shows: email, plan badge, staleness ("2 min ago"), 3 quota cells, status badge
- [ ] Quota cells show percentage with color-coded bar:
  - Orange = Claude+GPT
  - Green = Gemini Pro
  - Blue = Gemini Flash
- [ ] Status badge: "Ready" (green) if any group > 0%, "Exhausted" (red) if all 0%
- [ ] **Click row** → expands to show per-model breakdown with progress bars
- [ ] Click expanded row again → collapses
- [ ] Multiple rows can be expanded simultaneously

### 2.4 Quota History Chart
- [ ] Chart section appears below the account grid
- [ ] Title: "Quota History"
- [ ] Account selector dropdown: "All Accounts" + one option per tracked account
- [ ] Range selector: "Last 20" / "Last 50" / "Last 100"
- [ ] If no snapshots: shows "No snapshot history yet. Click Snap Now to start tracking."
- [ ] If snapshots exist: shows Chart.js line chart with:
  - X-axis: dates/times
  - Y-axis: 0–100%
  - Color-coded lines per quota group
  - Hover tooltip showing exact percentage
  - Legend at bottom
- [ ] Change account filter → chart updates
- [ ] Change range → chart updates
- [ ] Switch to light theme → chart colors adapt (grid, text, tooltip)

---

## 3. Subscriptions Tab (Manual Tracking)

### 3.1 Empty State
- [ ] Shows empty grid (no error)
- [ ] Filter dropdowns visible (All Statuses, All Categories)
- [ ] Search bar visible with 🔍 placeholder
- [ ] "+ Add" button visible

### 3.2 Add Subscription (Preset)
- [ ] Click **+ Add** → modal opens with title "New Subscription"
- [ ] In Platform field, type "Cursor" → datalist shows "Cursor Pro", "Cursor Pro+", "Cursor Ultra"
- [ ] Select "Cursor Pro" → fields auto-fill:
  - Category: coding
  - Cost: 20
  - Cycle: monthly
  - Notes: contains "500 fast premium requests"
  - URL: cursor.com/settings
  - Status Page: status.cursor.com
- [ ] Set Email: `test@example.com`
- [ ] Set Next Renewal: tomorrow's date
- [ ] Click **Save Subscription**
- [ ] Toast: "✅ Subscription created"
- [ ] Modal closes
- [ ] Subscription card appears in grid under "coding" category

### 3.3 Add Subscription (Custom)
- [ ] Click **+ Add**
- [ ] Enter Platform: "My Custom Tool"
- [ ] Set Category: "other"
- [ ] Set Cost: 9.99, Currency: EUR, Cycle: monthly
- [ ] Set Status: "trial"
- [ ] Set Trial Ends At: 3 days from now
- [ ] Click **Save**
- [ ] Card appears with:
  - "trial" badge (yellow)
  - "other" category badge
  - €9.99/mo cost display
  - "Trial ends in 3 days" red countdown

### 3.4 Edit Subscription
- [ ] Click **Edit** on any subscription card
- [ ] Modal opens with title "Edit Subscription"
- [ ] All fields pre-filled with current values
- [ ] Change cost to 25
- [ ] Click **Save**
- [ ] Toast: "✅ Subscription updated"
- [ ] Card updates to show new cost

### 3.5 Delete Subscription
- [ ] Click **Delete** on any subscription card
- [ ] Confirmation dialog appears: "Delete [platform]?"
- [ ] Click **Cancel** → nothing happens
- [ ] Click **Delete** again → Confirm
- [ ] Toast: "✅ Subscription deleted"
- [ ] Card disappears from grid

### 3.6 Filters
- [ ] Create subscriptions with different statuses (active, trial, paused)
- [ ] Select "Active" from status filter → only active cards shown
- [ ] Select "Trial" → only trial cards shown
- [ ] Select "All Statuses" → all cards shown
- [ ] Select "coding" from category filter → only coding cards shown
- [ ] Select "All Categories" → all cards shown
- [ ] Both filters work simultaneously

### 3.7 Search
- [ ] Type part of a platform name in search box → matching cards shown, others hidden
- [ ] Type an email → cards with that email shown
- [ ] Clear search → all cards visible again
- [ ] Category labels auto-hide when no matching cards exist in that category

### 3.8 Card Details
- [ ] Dashboard URL link: click → opens platform dashboard in new tab
- [ ] Status page link: click → opens status page in new tab
- [ ] Notes text visible on card
- [ ] Limit chips visible (e.g., "500 requests/monthly")
- [ ] Auto-tracked badge ("AUTO") visible on snap-linked subscriptions

### 3.9 Auto-Link on Snap
- [ ] Delete any existing Antigravity subscription
- [ ] Go to Quotas tab → click Snap Now
- [ ] Go to Subscriptions tab
- [ ] An "Antigravity" card should auto-appear with:
  - AUTO badge
  - Email from the snapped account
  - Plan name auto-detected
  - Cost: $15 (Pro) or $60 (Pro+)

---

## 4. Overview Tab

### 4.1 Empty State (no subscriptions)
- [ ] Shows $0.00 monthly spend
- [ ] "No subscriptions yet" in category section
- [ ] "No upcoming renewals" message
- [ ] "Set Budget" prompt visible

### 4.2 With Subscriptions
- [ ] Monthly spend shows sum of all active subscriptions (monthly-normalized)
- [ ] Annual estimate = monthly × 12
- [ ] Category breakdown shows each category with count and spend
- [ ] Categories sorted by highest spend first

### 4.3 Renewals
- [ ] Subscriptions with `next_renewal` set appear in "Upcoming Renewals"
- [ ] Sorted by nearest date first
- [ ] Days shown: "3 days" (red if ≤7), "15 days" (normal)

### 4.4 Quick Links
- [ ] All subscriptions with a dashboard URL appear as clickable links
- [ ] Click → opens in new tab
- [ ] If no URLs set: shows hint text

### 4.5 Budget Alert
- [ ] If no budget set: shows "No monthly budget set" with **Set Budget** button
- [ ] Click **Set Budget** → budget modal opens
- [ ] Enter 200 → click Save
- [ ] Toast: "✅ Budget set to $200/mo"
- [ ] Alert bar updates:
  - ✅ green if spend < 80% of budget
  - ⚠️ yellow if spend 80–99%
  - 🚨 red if spend ≥ 100%
- [ ] **Edit** button on alert → re-opens modal

### 4.6 Smart Insights
- [ ] "Insights" section shows colored chips:
  - 📊 "X active subscriptions" (blue)
  - ⏳ "X trials active" (yellow) — only if trials exist
  - 💰 "Most spent on: [category]" — if any spending
  - 📅 "Next renewal: [platform] in X days" (green)
  - 🔴 "X renewals in next 3 days" (yellow) — only if imminent
  - 📈 "X pay-as-you-go services (unbounded cost)" — if PAYG exists
  - 💡 "Could save ~$X/yr by switching to annual billing" — if monthly > $100

### 4.7 Ready Now (Auto-Tracked)
- [ ] If quota snapshots exist: shows "Ready Now" section with account readiness
- [ ] Each account shows ✅/⚠️/❌ per quota group with percentage

### 4.8 CSV Export
- [ ] Click **📥 Export CSV** button
- [ ] Browser downloads `niyantra-subscriptions-YYYY-MM-DD.csv`
- [ ] Open CSV: columns match (Platform, Category, Plan, Status, Monthly Cost, ...)
- [ ] All subscriptions present in the file

---

## 5. Settings Tab

### 5.1 Capture & Sources
- [ ] "Auto Capture" toggle visible — OFF by default
- [ ] Toggle Auto Capture ON → mode badge in header changes to green "AUTO"
- [ ] Poll Interval row appears when Auto Capture is ON
- [ ] Toggle Auto Capture OFF → mode badge returns to blue "MANUAL"
- [ ] Poll Interval row hides when Auto Capture is OFF
- [ ] "Auto-Link Subscriptions" toggle visible — ON by default
- [ ] Toggle Auto-Link OFF → change persists on page refresh
- [ ] Data Sources section shows 3 sources:
  - Antigravity (ls_poll, Active)
  - Claude Code (log_parse, Disabled)
  - Codex (log_parse, Disabled)
- [ ] Antigravity shows capture count and last capture time

### 5.2 Budget & Display
- [ ] Monthly Budget input shows current value from server config
- [ ] Change to 300 → tab away → toast confirms → refreshes persist
- [ ] Default Currency dropdown: USD, EUR, GBP, INR, CAD, AUD
- [ ] Change currency → toast confirms → change persists on refresh
- [ ] Theme dropdown: Dark, Light, System
- [ ] Select "Light" → page switches to light mode
- [ ] Select "System" → follows OS preference
- [ ] Theme persists on refresh (localStorage)

### 5.3 Data Management
- [ ] Snapshot Retention input shows 365 (default)
- [ ] Change to 180 → persists on refresh
- [ ] CSV export button works (same as Overview)
- [ ] Database location shows path

### 5.4 Activity Log
- [ ] Activity log section shows recent events
- [ ] At minimum: `server_start` entry visible (logged on serve startup)
- [ ] Filter dropdown: All Events, Snaps, Failed Snaps, Config Changes, Server Start
- [ ] Select "Server Start" → only server_start events shown
- [ ] Select "Config Changes" → shows config change events (after changing a setting)
- [ ] ↻ Refresh button reloads the log
- [ ] Each entry shows: timestamp, event type (color-coded badge), detail text
- [ ] Event type colors: snap=blue, snap_failed=red, config_change=purple, server_start=green

### 5.5 Keyboard Shortcuts Reference
- [ ] Grid shows 8 shortcuts with `kbd` styled keys

### 5.6 About
- [ ] Shows "Niyantra — AI Operations Dashboard"
- [ ] Shows "Schema v3 · 26 presets · Mode: Manual · 1 active source"
- [ ] Mode text updates when Auto Capture is toggled

---

## 6. Keyboard Shortcuts

Test each shortcut from the Quotas tab with no modal open:

- [ ] Press `1` → stays on Quotas tab
- [ ] Press `2` → switches to Subscriptions
- [ ] Press `3` → switches to Overview
- [ ] Press `4` → switches to Settings
- [ ] Press `N` → opens new subscription modal
- [ ] Press `Esc` → closes modal
- [ ] Press `S` → triggers snap (toast appears)
- [ ] Press `/` → switches to Subscriptions tab and focuses search box

### Shortcut Safety
- [ ] Shortcuts do NOT fire when typing in an input field
- [ ] Shortcuts do NOT fire when a modal is open (except Esc)
- [ ] `Esc` closes any open modal (subscription, delete confirm, budget)

---

## 7. PWA / Installability

- [ ] Open Chrome DevTools → Application → Manifest
- [ ] Manifest detected with name "Niyantra — AI Dashboard"
- [ ] "Install" prompt available in browser address bar (Chrome/Edge)

---

## 8. Cross-Browser / Responsive

### 8.1 Browser Compatibility
- [ ] Chrome: all features work
- [ ] Firefox: all features work
- [ ] Edge: all features work

### 8.2 Responsive (resize to 768px wide)
- [ ] Account grid stacks vertically
- [ ] Subscription cards stack
- [ ] Modals remain usable
- [ ] Settings page remains readable

---

## 9. CLI Commands

### 9.1 `niyantra snap`
```
.\niyantra.exe snap
```
- [ ] Outputs snapshot summary with email, plan, quota percentages
- [ ] Returns exit code 0

### 9.2 `niyantra status`
```
.\niyantra.exe status
```
- [ ] Shows readiness table for all tracked accounts
- [ ] Zero network calls (reads from SQLite only)

### 9.3 `niyantra serve`
```
.\niyantra.exe serve --port 8080 --debug
```
- [ ] Starts on specified port
- [ ] Debug logging visible in terminal

### 9.4 `niyantra version`
```
.\niyantra.exe version
```
- [ ] Prints version string

---

## 10. Edge Cases

### 10.1 Data Integrity
- [ ] Create subscription with empty optional fields → saves correctly
- [ ] Create subscription with very long notes → displays without breaking layout
- [ ] Create subscription with cost = 0 → shows as free / no cost
- [ ] Edit subscription to change status from active to cancelled → badge updates

### 10.2 Concurrent Access
- [ ] Open dashboard in 2 browser tabs
- [ ] Create subscription in tab 1 → switch to tab 2 → click Subscriptions → shows new data

### 10.3 Database
- [ ] Delete `~/.niyantra/niyantra.db`
- [ ] Restart server → database recreated with empty state
- [ ] All tables created (v2 migration runs)

---

## 11. Mode Badge (Header)

- [ ] Mode badge visible in header (between logo and "Snap Now" button)
- [ ] Default: blue badge with "MANUAL" text and solid dot
- [ ] After enabling Auto Capture in Settings: green badge with "AUTO" text and pulsing dot
- [ ] Badge hidden on small screens (< 768px)

---

## 12. Config API (Server-Side Settings)

### 12.1 Config Persistence
- [ ] Open Settings → change budget → refresh page → budget value persists
- [ ] Open Settings → change currency → refresh page → currency persists
- [ ] All settings survive server restart (stored in SQLite, not localStorage)

### 12.2 localStorage Migration
- [ ] If `niyantra-budget` exists in localStorage: value migrates to server config on first load
- [ ] After migration: localStorage key is removed
- [ ] If `niyantra-currency` exists in localStorage: same migration behavior
- [ ] Theme stays in localStorage (not migrated)

### 12.3 Config Change Logging
- [ ] Change any setting → go to Activity Log → `config_change` event appears
- [ ] Detail shows: key name, old value → new value

---

## 13. Provenance & Audit Trail

### 13.1 Snap Provenance (UI)
- [ ] Click Snap Now on dashboard → go to Activity Log
- [ ] Entry shows: `snap`, email, "manual via ui"

### 13.2 Snap Provenance (CLI)
- [ ] Run `.\niyantra.exe snap` in terminal
- [ ] Open dashboard → Activity Log → entry shows: `snap`, email, "manual via cli"

### 13.3 Failed Snap Logging
- [ ] Close Antigravity IDE → click Snap Now
- [ ] Activity Log shows: `snap_failed` with error message (red badge)

### 13.4 Server Start Logging
- [ ] Stop and restart `.\niyantra.exe serve`
- [ ] Open Activity Log → `server_start` entry with port and mode

### 13.5 Data Source Bookkeeping
- [ ] After successful snap: Antigravity source shows updated capture count
- [ ] Last capture time updates to "just now" or similar

---

## 14. Database Migration (v2 → v3)

- [ ] Delete `~/.niyantra/niyantra.db`
- [ ] Start server → v3 schema created from scratch (all tables)
- [ ] OR: use existing v2 database → v3 migration runs automatically:
  - `config` table created with 6 seeded rows
  - `activity_log` table created
  - `data_sources` table created with 3 seeded rows
  - `snapshots` table gains 3 new columns (capture_method, capture_source, source_id)
  - Existing snapshots default to capture_method='manual'

---

## 15. Auto-Capture Agent (Phase 6)

### 15.1 Enable/Disable Toggle
- [ ] Settings → Capture & Sources → toggle "Auto Capture" ON
- [ ] Toast shows "🟢 Auto-capture started"
- [ ] Poll Interval row appears below toggle
- [ ] Mode badge in header changes to green "Auto" with pulsing dot
- [ ] Toggle "Auto Capture" OFF → toast "⏸️ Auto-capture stopped"
- [ ] Mode badge returns to blue "Manual"
- [ ] Poll Interval row hides

### 15.2 Polling Behavior
- [ ] Set poll interval to 30s, enable auto-capture
- [ ] Within ~30s, new snapshot appears (check via Quotas tab refresh)
- [ ] Activity log shows event with type=snap, source=server, method=auto
- [ ] History endpoint shows new snapshot with `captureMethod: "auto"`, `captureSource: "server"`
- [ ] Data sources list shows updated capture count and "Last: Xs ago"

### 15.3 Polling Status Indicator
- [ ] When auto-capture is active, green status bar appears below poll interval
- [ ] Shows "● Polling every 30s · Last: Xs ago"
- [ ] Status auto-refreshes every 30s
- [ ] Activity log auto-refreshes when settings tab is open and polling is active

### 15.4 Interval Change
- [ ] While auto-capture is running, change poll interval from 30 to 60
- [ ] Agent restarts with new interval (check logs or activity)
- [ ] Polling status updates to show "every 60s"

### 15.5 Mode API Enhancement
- [ ] `GET /api/mode` returns `isPolling: true` when agent is running
- [ ] Returns `pollInterval`, `lastPoll`, `lastPollOK` fields
- [ ] `isPolling: false` when auto-capture is disabled

### 15.6 Server Startup Behavior
- [ ] Enable auto-capture, stop server, restart server
- [ ] Startup banner shows `Mode: auto` and `Polling: every Ns`
- [ ] Agent starts automatically on boot (activity log shows auto-snap events)

### 15.7 Backoff on Failures
- [ ] Enable auto-capture with Antigravity LS NOT running
- [ ] First 3 polls log warnings (activity log: snap_failed events)
- [ ] After 3 failures, polling pauses (debug log: "Auto-capture paused (backoff)")
- [ ] Start Antigravity LS → agent retries and resumes

### 15.8 Graceful Shutdown
- [ ] Enable auto-capture, press Ctrl+C in terminal
- [ ] "Shutting down gracefully..." message appears
- [ ] No error messages, no orphaned goroutine warnings
- [ ] Server exits cleanly

### 15.9 Auto-Link with Auto-Capture
- [ ] Delete all subscriptions, enable auto-capture, ensure auto-link is ON
- [ ] After first auto-snap, subscription is auto-created
- [ ] Activity log shows "auto_link" event

---

## Test Results Template

| Section | Pass | Fail | Notes |
|---------|------|------|-------|
| 1. Dashboard Shell | | | |
| 2. Quotas Tab | | | |
| 3. Subscriptions Tab | | | |
| 4. Overview Tab | | | |
| 5. Settings Tab | | | |
| 6. Keyboard Shortcuts | | | |
| 7. PWA | | | |
| 8. Cross-Browser | | | |
| 9. CLI Commands | | | |
| 10. Edge Cases | | | |
| 11. Mode Badge | | | |
| 12. Config API | | | |
| 13. Provenance & Audit | | | |
| 14. DB Migration | | | |
| 15. Auto-Capture Agent | | | |

**Tester:** _______________  
**Date:** _______________  
**Build:** `niyantra.exe` from commit _______________

