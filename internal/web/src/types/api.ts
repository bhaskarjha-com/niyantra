/**
 * Niyantra API Response TypeScript Interfaces
 *
 * Generated from Go struct definitions in:
 *   - internal/readiness/readiness.go
 *   - internal/forecast/forecast.go
 *   - internal/costtrack/costtrack.go
 *   - internal/client/types.go
 *   - internal/store/store.go
 */

// /api/status response

/** Top-level response from GET /api/status */
export interface StatusResponse {
  accounts: AccountReadiness[];
  snapshotCount: number;
  accountCount: number;
  codexSnapshot?: CodexSnapshot;
  claudeSnapshot?: ClaudeSnapshot;
  forecasts?: AccountForecast[];
  estimatedCosts?: AccountCostEstimate[];
}

/** Readiness state for a single Antigravity account */
export interface AccountReadiness {
  accountId: number;
  latestSnapshotId: number;
  email: string;
  planName: string;
  notes: string;
  tags: string;
  pinnedGroup: string;
  creditRenewalDay: number;
  lastSeen: string; // ISO 8601 timestamp
  stalenessLabel: string;
  isReady: boolean;
  groups: GroupReadiness[];
  models: ModelDetail[];
  promptCredits: number;
  monthlyCredits: number;
  aiCredits: AICredit[];
}

/** Readiness state for a single quota group */
export interface GroupReadiness {
  groupKey: string;
  displayName: string;
  remainingPercent: number; // 0-100
  isExhausted: boolean;
  isReady: boolean;
  color: string;
  resetTime?: string; // ISO 8601, optional
  timeUntilResetSec: number;
}

/** Per-model quota detail */
export interface ModelDetail {
  modelId: string;
  label: string;
  remainingPercent: number; // 0-100
  isExhausted: boolean;
  resetSeconds: number;
  groupKey: string;
}

/** AI credit balance (e.g., Google One AI) */
export interface AICredit {
  creditType: string;
  creditAmount: number;
  minimumForUsage: number;
}

// Forecast types (F7: Time-to-Exhaustion)

/** Per-account forecast with per-group predictions */
export interface AccountForecast {
  accountId: number;
  email: string;
  groups: GroupForecast[];
}

/** TTX prediction for a single quota group */
export interface GroupForecast {
  groupKey: string;
  displayName: string;
  burnRate: number;     // fraction consumed per hour (0.0-1.0 scale)
  ttxHours: number;     // hours until exhaustion (-1 = no data, 0 = exhausted)
  ttxLabel: string;     // "~2.3h left", "~45m left"
  remaining: number;    // current remaining fraction (0.0-1.0)
  confidence: string;   // "high", "medium", "low"
  willExhaust: boolean;
  severity: string;     // "safe", "caution", "warning", "critical"
}

// Cost estimation types (F8: Estimated Cost)

/** Per-account cost estimate */
export interface AccountCostEstimate {
  accountId: number;
  email: string;
  totalCost: number;
  totalLabel: string;     // "$4.56"
  groups: GroupCostEstimate[];
}

/** Cost estimate for a single quota group */
export interface GroupCostEstimate {
  groupKey: string;
  displayName: string;
  consumedFraction: number;
  estimatedTokens: number;
  estimatedCost: number;
  costPerHour: number;
  costLabel: string;      // "$1.23"
  hourlyLabel: string;    // "$0.41/hr"
  hasData: boolean;
}

// Subscription types

/** Subscription from GET /api/subscriptions */
export interface Subscription {
  id: number;
  platform: string;
  category: string;
  planName: string;
  email: string;
  costAmount: number;
  costCurrency: string;
  billingCycle: string;
  startDate: string;        // ISO 8601
  nextBillingDate: string;
  status: string;           // "active", "cancelled", "trial", "paused"
  notes: string;
  linkedAccountId: number;
  limitPeriod: string;
  createdAt: string;
  updatedAt: string;
}

// Codex & Claude types

/** Codex snapshot from GET /api/codex/sessions */
export interface CodexSnapshot {
  id: number;
  accountId: number;
  capturedAt: string;
  activeSessionCount: number;
  completedSessions: number;
  totalTokens: number;
  sessions: CodexSession[];
}

/** Single Codex session */
export interface CodexSession {
  sessionId: string;
  status: string;
  model: string;
  startedAt: string;
  totalTokens: number;
}

/** Claude snapshot from internal store */
export interface ClaudeSnapshot {
  id: number;
  capturedAt: string;
  totalTokens: number;
  contextWindow: number;
  sessions: ClaudeSession[];
}

export interface ClaudeSession {
  projectPath: string;
  model: string;
  totalTokens: number;
  contextUsedPercent: number;
}

// Settings / Config types

/** Model pricing entry from F5 config */
export interface ModelPrice {
  modelId: string;
  displayName: string;
  provider: string;      // "anthropic", "google", "openai"
  inputPer1M: number;
  outputPer1M: number;
  cachePer1M: number;
}

/** System alert from alerts API */
export interface SystemAlert {
  id: string;
  severity: string;      // "info", "warning", "error"
  title: string;
  message: string;
  createdAt: string;
  acknowledged: boolean;
  metadata: Record<string, unknown>;
}

/** Activity log entry */
export interface ActivityLogEntry {
  id: number;
  timestamp: string;
  level: string;          // "info", "warn", "error"
  source: string;         // "ui", "agent", "notify", "system"
  action: string;
  email: string;
  snapshotId: number;
  metadata: Record<string, unknown>;
}

// Frontend state types (not from API)

/** Tab identifiers */
export type TabId = 'quotas' | 'subscriptions' | 'overview' | 'settings';

/** Filter state for the quotas tab */
export interface QuotaFilterState {
  status: string;       // "all", "ready", "warning", "critical", "stale"
  provider: string;     // "all", "antigravity", etc.
  tag: string;          // "all" or a specific tag
  search: string;       // free-text search
}

/** Theme mode */
export type ThemeMode = 'dark' | 'light';

/** Overview API response */
export interface OverviewResponse {
  totalAccounts: number;
  readyAccounts: number;
  exhaustedAccounts: number;
  overviewHtml?: string;
  // TODO: type remaining fields as they're discovered
  [key: string]: unknown;
}

/** Presets API response */
export interface PresetsResponse {
  presets: PresetEntry[];
}

/** Single preset entry */
export interface PresetEntry {
  platform: string;
  category: string;
  billingCycle: string;
  costAmount: number;
  costCurrency: string;
}

/** Server config key-value store */
export interface ServerConfig {
  [key: string]: string;
}

/** Sort state for quota table */
export interface QuotaSortState {
  column: string;
  direction: 'asc' | 'desc';
}

/** Group color map */
export interface GroupColorMap {
  [groupKey: string]: string;
}

/** Group name map */
export interface GroupNameMap {
  [groupKey: string]: string;
}
