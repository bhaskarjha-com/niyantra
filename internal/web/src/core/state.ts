// Niyantra Dashboard — Global State
// Shared state variables used across all modules.

import type { ServerConfig, QuotaSortState, StatusResponse, GroupColorMap, GroupNameMap, PresetEntry } from '../types/api';

export const GROUP_ORDER: string[] = ['claude_gpt', 'gemini_pro', 'gemini_flash'];
export const GROUP_LABELS: string[] = ['Claude + GPT', 'Gemini Pro', 'Gemini Flash'];
export const GROUP_COLORS: GroupColorMap = { claude_gpt: '#D97757', gemini_pro: '#10B981', gemini_flash: '#3B82F6' };
export const GROUP_NAMES: GroupNameMap = { claude_gpt: 'Claude + GPT', gemini_pro: 'Gemini Pro', gemini_flash: 'Gemini Flash' };

// Track which accounts are expanded (survives re-renders)
export const expandedAccounts: Set<number> = new Set();

// Track which provider sections are collapsed (survives re-renders)
export const collapsedProviders: Set<string> = new Set();

// Platform presets (loaded from API)
export let presetsData: PresetEntry[] = [];
export function setPresetsData(data: PresetEntry[]): void { presetsData = data; }

// F4: Active tag filter for Quotas tab (null = show all)
export let activeTagFilter: string | null = null;
export function setActiveTagFilter(val: string | null): void { activeTagFilter = val; }

// Usage intelligence cache (populated by fetchUsage)
export let usageDataCache: Record<string, unknown> | null = null;
export function setUsageDataCache(data: Record<string, unknown> | null): void { usageDataCache = data; }

// Quota sort state
export let quotaSortState: QuotaSortState = { column: 'account', direction: 'asc' };

// Latest quota data cache
export let latestQuotaData: StatusResponse | null = null;
export function setLatestQuotaData(data: StatusResponse | null): void { latestQuotaData = data; }

// Server config cache (loaded from /api/config)
export let serverConfig: ServerConfig = {};
export function setServerConfig(key: string, value: string): void { serverConfig[key] = value; }

// Snap-in-progress flag
export let snapInProgress: boolean = false;
export function setSnapInProgress(val: boolean): void { snapInProgress = val; }
