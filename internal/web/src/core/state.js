// Niyantra Dashboard — Global State
// Shared state variables used across all modules.

export const GROUP_ORDER = ['claude_gpt', 'gemini_pro', 'gemini_flash'];
export const GROUP_LABELS = ['Claude + GPT', 'Gemini Pro', 'Gemini Flash'];
export const GROUP_COLORS = { claude_gpt: '#D97757', gemini_pro: '#10B981', gemini_flash: '#3B82F6' };
export const GROUP_NAMES = { claude_gpt: 'Claude + GPT', gemini_pro: 'Gemini Pro', gemini_flash: 'Gemini Flash' };

// Track which accounts are expanded (survives re-renders)
export const expandedAccounts = new Set();

// Track which provider sections are collapsed (survives re-renders)
export const collapsedProviders = new Set();

// Platform presets (loaded from API)
export var presetsData = [];
export function setPresetsData(data) { presetsData = data; }

// F4: Active tag filter for Quotas tab (null = show all)
export var activeTagFilter = null;
export function setActiveTagFilter(val) { activeTagFilter = val; }

// Usage intelligence cache (populated by fetchUsage)
export var usageDataCache = null;
export function setUsageDataCache(data) { usageDataCache = data; }

// Quota sort state
export var quotaSortState = { column: 'account', direction: 'asc' };

// Latest quota data cache
export var latestQuotaData = null;
export function setLatestQuotaData(data) { latestQuotaData = data; }

// Server config cache (loaded from /api/config)
export var serverConfig = {};
export function setServerConfig(key, value) { serverConfig[key] = value; }

// Snap-in-progress flag
export var snapInProgress = false;
export function setSnapInProgress(val) { snapInProgress = val; }
