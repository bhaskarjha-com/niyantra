// Niyantra Dashboard — Entry Point
// All core functionality is imported from modules.

import {
  GROUP_ORDER, GROUP_LABELS, GROUP_COLORS, GROUP_NAMES,
  expandedAccounts, collapsedProviders,
  presetsData, setPresetsData,
  activeTagFilter, setActiveTagFilter,
  usageDataCache, setUsageDataCache,
  quotaSortState, latestQuotaData, setLatestQuotaData,
  serverConfig, setServerConfig,
  snapInProgress, setSnapInProgress,
} from './core/state.js';

import {
  formatSeconds, formatCredits, formatNumber, currencySymbol,
  esc, showToast, updateTimestamp, refreshTimestampDisplay,
  formatTimeAgo, formatPollInterval, formatDurationSec,
} from './core/utils.js';

import {
  fetchStatus, triggerSnap,
  fetchSubscriptions, createSubscription, updateSubscription, deleteSubscription,
  fetchOverview, fetchPresets, fetchUsage,
} from './core/api.js';

import { initTheme, initTabs, switchToTab } from './core/theme.js';

import {
  renderAccounts, filterAccountsArray, sortAccountsArray,
  updateSortHeaders, renderTagFilterStrip, handleTagFilterClick,
  getCodexClaudeStatus, formatResetTime, allExhausted,
} from './quotas/render.js';
import { setupToggle, initQuotas } from './quotas/expand.js';
import {
  renderPinnedBadge, renderAccountTags, renderAccountNote,
  renderCreditRenewal, updateAccountMeta, initAccountMetaHandlers,
  setRenderAccounts,
} from './quotas/features.js';

import { loadSubscriptions, initModal, initSearch } from './subscriptions.js';

import { getBudget, setBudget, getCurrency, updateConfig, loadConfig, initBudget, openBudgetModal, closeBudget, renderBudgetAlert } from './overview/budget.js';
import { loadOverview } from './overview/overview.js';
import { calendarNav } from './overview/calendar.js';

import { initSnapDropdown, handleSnap } from './advanced/snap.js';
import { updateChartTheme, loadHistoryChart, populateChartAccountSelect } from './charts/history.js';
import { initSettings } from './settings/settings.js';
import { loadMode } from './settings/mode.js';
import { loadDataSources } from './settings/data.js';
import { loadActivityLog } from './settings/activity.js';
import { loadModelPricing } from './settings/pricing.js';
import { initKeyboardShortcuts } from './advanced/keyboard.js';
import { initCommandPalette } from './advanced/palette.js';
import { loadSystemAlerts, dismissAlert } from './advanced/alerts.js';
import { handleCodexSnap } from './advanced/codex.js';

// ════════════════════════════════════════════
//  INIT
// ════════════════════════════════════════════


document.addEventListener('DOMContentLoaded', function() {
  initTheme();
  initTabs();
  setRenderAccounts(renderAccounts);
  initQuotas();
  setupToggle();
  initModal();
  initBudget();
  initSettings();
  initSearch();
  initKeyboardShortcuts();
  initAccountMetaHandlers();

  // Tab-change event: domain modules react to tab activation
  document.addEventListener('niyantra:tab-change', function(e) {
    var tab = e.detail.tab;
    if (tab === 'overview') loadOverview();
    if (tab === 'settings') { loadActivityLog(); loadMode(); loadDataSources(); }
  });

  // Theme-change event: update chart colors
  document.addEventListener('niyantra:theme-change', function(e) {
    updateChartTheme(e.detail.theme);
  });

  // Chart refresh: triggered by quota expand module after data changes
  document.addEventListener('niyantra:chart-refresh', function() {
    loadHistoryChart();
  });

  // Overview refresh: triggered by calendar navigation
  document.addEventListener('niyantra:overview-refresh', function() {
    loadOverview();
  });

  document.getElementById('snap-btn').addEventListener('click', handleSnap);
  initSnapDropdown();

  // Chart controls
  document.getElementById('chart-account').addEventListener('change', loadHistoryChart);
  document.getElementById('chart-range').addEventListener('change', loadHistoryChart);

  // Load quotas and usage intelligence
  Promise.all([fetchStatus(), fetchUsage()]).then(function(results) {
    var data = results[0];
    renderAccounts(data);
    updateTimestamp();
    populateChartAccountSelect(data);
    loadHistoryChart();

    // Bug 1 fix: If no codex/claude data on first load, retry after 3s
    // (first serve often beats the initial codex capture to the DB)
    if (!data.codexSnapshot || !data.claudeSnapshot) {
      setTimeout(function() {
        fetchStatus().then(function(data2) {
          if (data2.codexSnapshot || data2.claudeSnapshot) {
            renderAccounts(data2);
          }
        }).catch(function() {});
      }, 3000);
    }
  }).catch(function(err) {
    console.error('Failed to load status:', err);
  });

  // Bug 3 fix: Pre-load subscriptions so tab is ready when first visited
  loadSubscriptions();

  // Load presets for the datalist
  fetchPresets().then(function(data) {
    setPresetsData(data.presets || []);
    var list = document.getElementById('preset-list');
    for (var i = 0; i < presetsData.length; i++) {
      var opt = document.createElement('option');
      opt.value = presetsData[i].platform;
      list.appendChild(opt);
    }
  });

  // Load mode badge in header (manual/auto status)
  loadMode();

  // Init command palette
  initCommandPalette();

  // Phase 10: Load system alerts
  loadSystemAlerts();

  // Auto-capture polling is handled server-side by the agent.
  // Manual data refreshes on snap or page reload.

  // H2: Refresh relative timestamp every 30s
  setInterval(refreshTimestampDisplay, 30000);
});


// ── IIFE Safety: Expose functions used by inline onclick="..." HTML attributes ──
window.openBudgetModal = openBudgetModal;
window.dismissAlert = dismissAlert;
window.switchToTab = switchToTab;
window.calendarNav = calendarNav;
window.handleCodexSnap = handleCodexSnap;
