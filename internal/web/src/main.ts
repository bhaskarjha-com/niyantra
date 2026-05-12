// Niyantra Dashboard — Entry Point
// All core functionality is imported from modules.

// Augment Window for inline onclick handler exposure
declare global {
  interface Window {
    openBudgetModal: typeof openBudgetModal;
    dismissAlert: typeof dismissAlert;
    switchToTab: typeof switchToTab;
    calendarNav: typeof calendarNav;
    handleCodexSnap: typeof handleCodexSnap;
  }
}

import {
  GROUP_ORDER, GROUP_LABELS, GROUP_COLORS, GROUP_NAMES,
  expandedAccounts, collapsedProviders,
  presetsData, setPresetsData,
  activeTagFilter, setActiveTagFilter,
  usageDataCache, setUsageDataCache,
  quotaSortState, latestQuotaData, setLatestQuotaData,
  serverConfig, setServerConfig,
  snapInProgress, setSnapInProgress,
} from './core/state';

import {
  formatSeconds, formatCredits, formatNumber, currencySymbol,
  esc, showToast, updateTimestamp, refreshTimestampDisplay,
  formatTimeAgo, formatPollInterval, formatDurationSec,
} from './core/utils';

import {
  fetchStatus, triggerSnap,
  fetchSubscriptions, createSubscription, updateSubscription, deleteSubscription,
  fetchOverview, fetchPresets, fetchUsage,
} from './core/api';

import { initTheme, initTabs, switchToTab } from './core/theme';

import {
  renderAccounts, filterAccountsArray, sortAccountsArray,
  updateSortHeaders, renderTagFilterStrip, handleTagFilterClick,
  getCodexClaudeStatus, formatResetTime, allExhausted,
} from './quotas/render';
import { setupToggle, initQuotas } from './quotas/expand';
import {
  renderPinnedBadge, renderAccountTags, renderAccountNote,
  renderCreditRenewal, updateAccountMeta, initAccountMetaHandlers,
  setRenderAccounts,
} from './quotas/features';

import { loadSubscriptions, initModal, initSearch } from './subscriptions';

import { getBudget, setBudget, getCurrency, updateConfig, loadConfig, initBudget, openBudgetModal, closeBudget, renderBudgetAlert } from './overview/budget';
import { loadOverview } from './overview/overview';
import { calendarNav } from './overview/calendar';

import { initSnapDropdown, handleSnap } from './advanced/snap';
import { updateChartTheme, loadHistoryChart, populateChartAccountSelect } from './charts/history';
import { initSettings } from './settings/settings';
import { loadMode } from './settings/mode';
import { loadDataSources } from './settings/data';
import { loadActivityLog } from './settings/activity';
import { loadModelPricing } from './settings/pricing';
import { initKeyboardShortcuts } from './advanced/keyboard';
import { initCommandPalette } from './advanced/palette';
import { loadSystemAlerts, dismissAlert } from './advanced/alerts';
import { handleCodexSnap } from './advanced/codex';

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
    var tab = (e as CustomEvent).detail.tab;
    if (tab === 'overview') loadOverview();
    if (tab === 'settings') { loadActivityLog(); loadMode(); loadDataSources(); }
  });

  // Theme-change event: update chart colors
  document.addEventListener('niyantra:theme-change', function(e) {
    updateChartTheme((e as CustomEvent).detail.theme);
  });

  // Chart refresh: triggered by quota expand module after data changes
  document.addEventListener('niyantra:chart-refresh', function() {
    loadHistoryChart();
  });

  // Overview refresh: triggered by calendar navigation
  document.addEventListener('niyantra:overview-refresh', function() {
    loadOverview();
  });

  document.getElementById('snap-btn')!.addEventListener('click', handleSnap);
  initSnapDropdown();

  // Chart controls
  document.getElementById('chart-account')!.addEventListener('change', loadHistoryChart);
  document.getElementById('chart-range')!.addEventListener('change', loadHistoryChart);

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
      list!.appendChild(opt);
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
