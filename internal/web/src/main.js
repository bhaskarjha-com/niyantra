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


import {
  loadSubscriptions, initModal, initSearch,
} from './subscriptions.js';



import { getBudget, setBudget, getCurrency, updateConfig, loadConfig, initBudget, openBudgetModal, closeBudget, renderBudgetAlert } from './overview/budget.js';
import { loadOverview } from './overview/overview.js';
import { calendarNav } from './overview/calendar.js';

// ════════════════════════════════════════════
//  SNAP HANDLER
// ════════════════════════════════════════════



// H3: Split-button snap — source-aware snapping
var snapDefault = localStorage.getItem('niyantra_snap_default') || 'antigravity';

function initSnapDropdown() {
  var caret = document.getElementById('snap-caret');
  var dropdown = document.getElementById('snap-dropdown');
  if (!caret || !dropdown) return;

  // Toggle dropdown
  caret.addEventListener('click', function(e) {
    e.stopPropagation();
    dropdown.classList.toggle('open');
  });

  // Close on outside click
  document.addEventListener('click', function() {
    dropdown.classList.remove('open');
  });

  // Option clicks
  dropdown.querySelectorAll('.snap-option').forEach(function(opt) {
    opt.addEventListener('click', function(e) {
      e.stopPropagation();
      var source = opt.dataset.source;
      dropdown.classList.remove('open');
      if (source === 'all') {
        snapSource('all');
      } else {
        // Set as new default + snap it
        snapDefault = source;
        localStorage.setItem('niyantra_snap_default', source);
        updateSnapDropdownIndicators();
        snapSource(source);
      }
    });
  });

  updateSnapDropdownIndicators();
}

function updateSnapDropdownIndicators() {
  var dropdown = document.getElementById('snap-dropdown');
  if (!dropdown) return;
  dropdown.querySelectorAll('.snap-option').forEach(function(opt) {
    if (opt.dataset.source === 'all') return; // divider option
    var isActive = opt.dataset.source === snapDefault;
    opt.textContent = (isActive ? '◉ ' : '○ ') + opt.textContent.replace(/^[◉○] /, '');
    opt.classList.toggle('active', isActive);
  });
}

function handleSnap() {
  snapSource(snapDefault);
}

function snapSource(source) {
  var btn = document.getElementById('snap-btn');
  if (!btn || btn.disabled || snapInProgress) return;

  setSnapInProgress(true);
  btn.disabled = true;
  btn.classList.add('snapping');
  var orig = btn.innerHTML;
  btn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/></svg> Capturing...';

  var promises = [];

  if (source === 'antigravity' || source === 'all') {
    promises.push(
      triggerSnap().then(function(data) {
        return { source: 'Antigravity', data: data, label: data.email || 'Antigravity' };
      }).catch(function(err) {
        return { source: 'Antigravity', error: err.message };
      })
    );
  }

  if (source === 'codex' || source === 'all') {
    promises.push(
      fetch('/api/codex/snap', { method: 'POST' }).then(function(r) { return r.json(); })
      .then(function(d) {
        var label = d.plan ? ('Codex · ' + d.plan) : 'Codex';
        return { source: 'Codex', data: d, label: label };
      })
      .catch(function() { return { source: 'Codex', error: 'capture failed' }; })
    );
  }

  if (promises.length === 0) {
    btn.innerHTML = orig;
    btn.disabled = false;
    setSnapInProgress(false);
    showToast('No snap source selected', 'warning');
    return;
  }

  Promise.all(promises).then(function(results) {
    var msgs = [];
    var antigravityData = null;
    for (var i = 0; i < results.length; i++) {
      var r = results[i];
      if (r.error) {
        msgs.push('❌ ' + r.source + ': ' + r.error);
      } else {
        msgs.push('✅ ' + r.label);
        if (r.source === 'Antigravity') antigravityData = r.data;
      }
    }
    showToast(msgs.join(' · '), msgs.some(function(m) { return m.startsWith('❌'); }) ? 'warning' : 'success');
    if (antigravityData) {
      renderAccounts(antigravityData);
      updateTimestamp();
    }
  }).finally(function() {
    btn.innerHTML = orig;
    btn.disabled = false;
    btn.classList.remove('snapping');
    setSnapInProgress(false);
  });
}



// ════════════════════════════════════════════
//  CHART — Quota History
// ════════════════════════════════════════════

var historyChart = null;

// M2: Update chart colors in-place on theme toggle (avoids destroy+rebuild flash)
function updateChartTheme(theme) {
  if (!historyChart) return;
  var isDark = theme !== 'light';
  var gridColor = isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)';
  var textColor = isDark ? '#94a3b8' : '#64748b';
  if (historyChart.options.scales && historyChart.options.scales.y) {
    historyChart.options.scales.y.grid.color = gridColor;
    historyChart.options.scales.y.ticks.color = textColor;
  }
  if (historyChart.options.scales && historyChart.options.scales.x) {
    historyChart.options.scales.x.grid.color = gridColor;
    historyChart.options.scales.x.ticks.color = textColor;
  }
  historyChart.update('none'); // 'none' = no animation, instant repaint
}

function loadHistoryChart() {
  if (typeof Chart === 'undefined') return; // CDN not loaded (offline)

  var accountId = parseInt(document.getElementById('chart-account').value) || 0;
  var limit = parseInt(document.getElementById('chart-range').value) || 20;

  var url = '/api/history?limit=' + limit;
  if (accountId > 0) url += '&account=' + accountId;

  fetch(url).then(function(res) { return res.json(); }).then(function(data) {
    renderHistoryChart(data.snapshots || []);
  }).catch(function(err) {
    console.error('Failed to load history:', err);
  });
}

function renderHistoryChart(snapshots) {
  var container = document.querySelector('.chart-container');
  if (!container || typeof Chart === 'undefined') return;

  if (snapshots.length === 0) {
    container.innerHTML = '<div class="chart-empty">No snapshot history yet. Click Snap Now to start tracking.</div>';
    return;
  }
  container.innerHTML = '<canvas id="history-chart"></canvas>';

  // Reverse so oldest is first (left-to-right timeline)
  snapshots = snapshots.slice().reverse();

  var labels = snapshots.map(function(s) {
    var d = new Date(s.capturedAt);
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' }) +
      ' ' + d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  });

  // Build datasets per group
  var groupData = {};
  var groupNames = { claude_gpt: 'Claude + GPT', gemini_pro: 'Gemini Pro', gemini_flash: 'Gemini Flash' };
  var groupColors = { claude_gpt: '#D97757', gemini_pro: '#10B981', gemini_flash: '#3B82F6' };

  for (var i = 0; i < snapshots.length; i++) {
    var groups = snapshots[i].groups || [];
    for (var j = 0; j < groups.length; j++) {
      var g = groups[j];
      if (!groupData[g.groupKey]) groupData[g.groupKey] = [];
    }
  }

  var aiCreditsData = [];
  var hasAICredits = false;

  for (var i = 0; i < snapshots.length; i++) {
    var snap = snapshots[i];
    var groups = snap.groups || [];
    var seen = {};
    for (var j = 0; j < groups.length; j++) {
      var g = groups[j];
      if (!groupData[g.groupKey]) groupData[g.groupKey] = [];
      groupData[g.groupKey].push(Math.round(g.remainingPercent || 0));
      seen[g.groupKey] = true;
    }
    // Fill nulls for missing groups
    var keys = Object.keys(groupData);
    for (var k = 0; k < keys.length; k++) {
      if (!seen[keys[k]]) groupData[keys[k]].push(null);
    }

    // Capture AI credits
    if (snap.aiCredits && snap.aiCredits.length > 0) {
      aiCreditsData.push(snap.aiCredits[0].creditAmount);
      hasAICredits = true;
    } else {
      aiCreditsData.push(null);
    }
  }

  var datasets = [];
  var keys = Object.keys(groupData);
  for (var k = 0; k < keys.length; k++) {
    var key = keys[k];
    if (!key || !groupNames[key]) continue; // Skip unknown/empty groups
    datasets.push({
      label: groupNames[key],
      data: groupData[key],
      borderColor: groupColors[key] || '#94a3b8',
      backgroundColor: (groupColors[key] || '#94a3b8') + '20',
      yAxisID: 'y',
      fill: true,
      tension: 0.3,
      pointRadius: 3,
      pointHoverRadius: 6,
      borderWidth: 2,
    });
  }

  if (hasAICredits) {
    datasets.push({
      label: 'AI Credits',
      data: aiCreditsData,
      borderColor: '#fbbf24', // Amber
      backgroundColor: 'transparent',
      yAxisID: 'yCredits',
      borderDash: [5, 5],
      tension: 0.3,
      pointRadius: 4,
      pointBackgroundColor: '#fbbf24',
      pointHoverRadius: 6,
      borderWidth: 3,
    });
  }

  // Determine theme for chart
  var isDark = document.documentElement.getAttribute('data-theme') !== 'light';
  var gridColor = isDark ? 'rgba(255,255,255,0.06)' : 'rgba(0,0,0,0.06)';
  var textColor = isDark ? '#94a3b8' : '#64748b';

  if (historyChart) historyChart.destroy();

  var ctx = document.getElementById('history-chart');
  if (!ctx) return;

  historyChart = new Chart(ctx, {
    type: 'line',
    data: { labels: labels, datasets: datasets },
    options: {
      responsive: true,
      maintainAspectRatio: false,
      interaction: { mode: 'index', intersect: false },
      plugins: {
        legend: {
          position: 'bottom',
          labels: { color: textColor, font: { family: "'Inter', sans-serif", size: 11 }, boxWidth: 12, padding: 16 }
        },
        tooltip: {
          backgroundColor: isDark ? '#1e293b' : '#fff',
          titleColor: isDark ? '#f1f5f9' : '#0f172a',
          bodyColor: isDark ? '#94a3b8' : '#475569',
          borderColor: isDark ? '#334155' : '#e2e8f0',
          borderWidth: 1,
          padding: 10,
          titleFont: { family: "'Inter', sans-serif", weight: '600' },
          bodyFont: { family: "'Inter', sans-serif" },
          callbacks: {
            label: function(ctx) {
              if (ctx.dataset.yAxisID === 'yCredits') return ctx.dataset.label + ': ' + ctx.parsed.y.toLocaleString();
              return ctx.dataset.label + ': ' + ctx.parsed.y + '%';
            }
          }
        }
      },
      scales: {
        y: {
          type: 'linear',
          display: true,
          position: 'left',
          min: 0, max: 100,
          grid: { color: gridColor },
          ticks: { color: textColor, font: { family: "'Inter', sans-serif", size: 11 }, callback: function(v) { return v + '%'; } },
          border: { display: false }
        },
        yCredits: {
          type: 'linear',
          display: hasAICredits,
          position: 'right',
          grid: { display: false },
          ticks: { color: isDark ? '#fbbf24' : '#d97706', font: { family: "'Inter', sans-serif", size: 11 } },
          border: { display: false }
        },
        x: {
          grid: { display: false },
          ticks: { color: textColor, font: { family: "'Inter', sans-serif", size: 10 }, maxRotation: 45, maxTicksLimit: 12 },
          border: { display: false }
        }
      }
    }
  });
}

function populateChartAccountSelect(data) {
  var sel = document.getElementById('chart-account');
  if (!sel || !data.accounts) return;
  // Keep "All Accounts" option, remove others
  while (sel.options.length > 1) sel.remove(1);
  for (var i = 0; i < data.accounts.length; i++) {
    var opt = document.createElement('option');
    opt.value = data.accounts[i].accountId;
    opt.textContent = data.accounts[i].email;
    sel.appendChild(opt);
  }
}

// ════════════════════════════════════════════
// ════════════════════════════════════════════
// ════════════════════════════════════════════
// ════════════════════════════════════════════
// ════════════════════════════════════════════
//  SETTINGS
// ════════════════════════════════════════════

function initSettings() {
  var themeEl = document.getElementById('s-theme');

  // Theme stays in localStorage (visual-only)
  var savedTheme = localStorage.getItem('niyantra-theme') || 'dark';
  themeEl.value = savedTheme;

  themeEl.addEventListener('change', function() {
    var val = themeEl.value;
    if (val === 'system') {
      localStorage.removeItem('niyantra-theme');
      var prefer = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', prefer);
    } else {
      localStorage.setItem('niyantra-theme', val);
      document.documentElement.setAttribute('data-theme', val);
    }
    // M2: Update chart theme in-place instead of destroy+rebuild
    var applied = document.documentElement.getAttribute('data-theme');
    updateChartTheme(applied);
  });

  // Load server config and populate settings UI
  loadConfig().then(function(cfg) {
    // Migrate localStorage budget/currency to server config (one-time)
    migrateLocalStorage(cfg);

    // Populate fields from server config
    var budgetEl = document.getElementById('s-budget');
    var currencyEl = document.getElementById('s-currency');
    var autoCaptureEl = document.getElementById('s-auto-capture');
    var autoLinkEl = document.getElementById('s-auto-link');
    var pollEl = document.getElementById('s-poll-interval');
    var retentionEl = document.getElementById('s-retention');

    budgetEl.value = parseFloat(cfg['budget_monthly'] || '0') || '';
    currencyEl.value = cfg['currency'] || 'USD';
    autoCaptureEl.checked = cfg['auto_capture'] === 'true';
    autoLinkEl.checked = cfg['auto_link_subs'] !== 'false';
    pollEl.value = cfg['poll_interval'] || '300';
    retentionEl.value = cfg['retention_days'] || '365';

    // Show/hide poll interval based on auto_capture
    document.getElementById('poll-interval-row').style.display =
      autoCaptureEl.checked ? '' : 'none';

    // Auto-save handlers
    budgetEl.addEventListener('change', function() {
      var val = parseFloat(budgetEl.value) || 0;
      setBudget(val);
      if (val > 0) showToast('✅ Budget: $' + val.toFixed(0) + '/mo', 'success');
    });

    currencyEl.addEventListener('change', function() {
      updateConfig('currency', currencyEl.value);
      showToast('✅ Currency: ' + currencyEl.value, 'success');
    });

    autoCaptureEl.addEventListener('change', function() {
      var val = autoCaptureEl.checked ? 'true' : 'false';
      updateConfig('auto_capture', val).then(function() {
        loadMode();
        showToast(autoCaptureEl.checked ? '🟢 Auto-capture started' : '⏸️ Auto-capture stopped', 'success');
      });
      document.getElementById('poll-interval-row').style.display =
        autoCaptureEl.checked ? '' : 'none';
    });

    autoLinkEl.addEventListener('change', function() {
      updateConfig('auto_link_subs', autoLinkEl.checked ? 'true' : 'false');
    });

    pollEl.addEventListener('change', function() {
      var v = pollEl.value;
      updateConfig('poll_interval', v).then(function() {
        // Show human-readable label from the selected option
        var label = pollEl.options[pollEl.selectedIndex].text;
        showToast('⏱️ Interval updated to ' + label + ' — takes effect on next cycle.', 'success');
        loadMode();
      });
    });

    retentionEl.addEventListener('change', function() {
      var v = parseInt(retentionEl.value);
      if (v >= 30 && v <= 3650) updateConfig('retention_days', v.toString());
    });

    // ── Phase 9: Claude Code Bridge ──
    var claudeBridgeEl = document.getElementById('s-claude-bridge');
    if (claudeBridgeEl) {
      claudeBridgeEl.checked = cfg['claude_bridge'] === 'true';
      claudeBridgeEl.addEventListener('change', function() {
        var val = claudeBridgeEl.checked ? 'true' : 'false';
        updateConfig('claude_bridge', val).then(function() {
          showToast(claudeBridgeEl.checked ? '🔗 Claude Code bridge enabled' : '🔗 Bridge disabled', 'success');
          loadClaudeBridgeStatus();
        });
      });
      loadClaudeBridgeStatus();
    }

    // ── Phase 9: Notifications ──
    var notifyEl = document.getElementById('s-notify-enabled');
    var thresholdEl = document.getElementById('s-notify-threshold');
    var thresholdRow = document.getElementById('notify-threshold-row');
    var testRow = document.getElementById('notify-test-row');
    if (notifyEl) {
      notifyEl.checked = cfg['notify_enabled'] === 'true';
      thresholdEl.value = cfg['notify_threshold'] || '10';
      thresholdRow.style.display = notifyEl.checked ? '' : 'none';
      testRow.style.display = notifyEl.checked ? '' : 'none';

      notifyEl.addEventListener('change', function() {
        var val = notifyEl.checked ? 'true' : 'false';
        updateConfig('notify_enabled', val).then(function() {
          showToast(notifyEl.checked ? '🔔 Notifications enabled' : '🔕 Notifications disabled', 'success');
        });
        thresholdRow.style.display = notifyEl.checked ? '' : 'none';
        testRow.style.display = notifyEl.checked ? '' : 'none';
      });

      thresholdEl.addEventListener('change', function() {
        var v = parseInt(thresholdEl.value);
        if (v >= 5 && v <= 50) {
          updateConfig('notify_threshold', v.toString());
          showToast('🔔 Threshold: ' + v + '%', 'success');
        }
      });

      document.getElementById('notify-test-btn').addEventListener('click', function() {
        fetch('/api/notify/test', { method: 'POST' })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) showToast('❌ ' + data.error, 'error');
            else showToast('🔔 Test notification sent!', 'success');
          })
          .catch(function() { showToast('❌ Failed to send test', 'error'); });
      });
    }

    // ── Phase 11: Codex Capture Toggle ──
    var codexCaptureEl = document.getElementById('s-codex-capture');
    if (codexCaptureEl) {
      codexCaptureEl.checked = cfg['codex_capture'] === 'true';
      codexCaptureEl.addEventListener('change', function() {
        var val = codexCaptureEl.checked ? 'true' : 'false';
        updateConfig('codex_capture', val).then(function() {
          showToast(codexCaptureEl.checked ? '🤖 Codex capture enabled' : '🤖 Codex capture disabled', 'success');
          loadCodexSettingsStatus();
          // T1: Refresh data sources list to reflect new enabled state
          loadDataSources();
        });
      });
      loadCodexSettingsStatus();
    }

    // ── Phase 11: JSON Import ──
    var importBtn = document.getElementById('import-json-btn');
    var importFile = document.getElementById('import-file');
    if (importBtn && importFile) {
      importBtn.addEventListener('click', function() { importFile.click(); });
      importFile.addEventListener('change', function() {
        if (!importFile.files || !importFile.files[0]) return;
        var file = importFile.files[0];
        showToast('📥 Importing ' + file.name + '...', 'info');
        var reader = new FileReader();
        reader.onload = function(e) {
          fetch('/api/import/json', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: e.target.result
          })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) {
              showToast('❌ Import failed: ' + data.error, 'error');
              return;
            }
            var msg = '✅ Imported: ' + (data.accountsCreated || 0) + ' accounts, ' +
                      (data.subsCreated || 0) + ' subs, ' +
                      (data.snapshotsImported || 0) + ' snapshots';
            showToast(msg, 'success');
            var resultEl = document.getElementById('import-result');
            if (resultEl) {
              resultEl.style.display = '';
              resultEl.innerHTML = '<span style="color:var(--accent)">' + msg + '</span>' +
                (data.accountsSkipped ? '<br>Accounts skipped (existing): ' + data.accountsSkipped : '') +
                (data.subsSkipped ? '<br>Subs skipped (existing): ' + data.subsSkipped : '') +
                (data.snapshotsDuped ? '<br>Snapshots deduped: ' + data.snapshotsDuped : '') +
                (data.errors && data.errors.length ? '<br>⚠️ Errors: ' + data.errors.length : '');
            }
            // Refresh data
            fetchStatus().then(renderAccounts);
            loadSubscriptions();
          })
          .catch(function() { showToast('❌ Import failed', 'error'); });
        };
        reader.readAsText(file);
        importFile.value = ''; // reset for re-import
      });
    }
  });

  // ── Phase 13 F5: Model Pricing ──
  loadModelPricing();
  document.getElementById('pricing-add-btn').addEventListener('click', addPricingRow);
  document.getElementById('pricing-reset-btn').addEventListener('click', resetPricingDefaults);

  // Load mode badge
  loadMode();

  // Load data sources
  loadDataSources();

  // Activity log controls
  document.getElementById('activity-refresh').addEventListener('click', loadActivityLog);
  document.getElementById('activity-filter').addEventListener('change', loadActivityLog);
  loadActivityLog();
}

// One-time migration of localStorage budget/currency to server config
function migrateLocalStorage(cfg) {
  var lsBudget = localStorage.getItem('niyantra-budget');
  var lsCurrency = localStorage.getItem('niyantra-currency');

  if (lsBudget && (!cfg['budget_monthly'] || cfg['budget_monthly'] === '0')) {
    updateConfig('budget_monthly', lsBudget);
    serverConfig['budget_monthly'] = lsBudget;
    localStorage.removeItem('niyantra-budget');
  }
  if (lsCurrency && cfg['currency'] === 'USD') {
    updateConfig('currency', lsCurrency);
    serverConfig['currency'] = lsCurrency;
    localStorage.removeItem('niyantra-currency');
  }
}

// ════════════════════════════════════════════
//  MODE BADGE
// ════════════════════════════════════════════

var modeRefreshTimer = null;

function loadMode() {
  fetch('/api/mode').then(function(r) { return r.json(); })
  .then(function(data) {
    var badge = document.getElementById('mode-badge');
    var label = document.getElementById('mode-label');
    if (data.mode === 'auto') {
      badge.className = 'mode-badge mode-auto';
      label.textContent = 'Auto';
    } else {
      badge.className = 'mode-badge mode-manual';
      label.textContent = 'Manual';
    }

    // Show polling status indicator
    var statusEl = document.getElementById('polling-status');
    if (statusEl) {
      if (data.isPolling) {
        var lastMsg = '';
        if (data.lastPoll) {
          lastMsg = 'Last: ' + formatTimeAgo(data.lastPoll);
          if (data.lastPollOK === false) lastMsg += ' (failed)';
        } else {
          lastMsg = 'Starting...';
        }
        statusEl.innerHTML = '<span class="polling-dot"></span> Polling every ' +
          formatPollInterval(data.pollInterval) + ' · ' + lastMsg;
        statusEl.style.display = '';
      } else {
        statusEl.style.display = 'none';
      }
    }

    // Update about section
    var aboutEl = document.getElementById('s-about-info');
    if (aboutEl) {
      var srcCount = (data.sources || []).filter(function(s) { return s.enabled; }).length;
      var schemaV = data.schemaVersion ? ('Schema v' + data.schemaVersion) : 'Schema';
      var presetCount = presetsData.length || 0;
      aboutEl.textContent = schemaV + ' · ' + presetCount + ' presets · Mode: ' +
        (data.mode === 'auto' ? 'Auto' : 'Manual') +
        (data.isPolling ? ' (polling)' : '') +
        ' · ' + srcCount + ' active source' + (srcCount !== 1 ? 's' : '');
    }

    // Auto-refresh mode badge every 30s when auto-capture is active
    if (modeRefreshTimer) { clearInterval(modeRefreshTimer); modeRefreshTimer = null; }
    if (data.isPolling) {
      modeRefreshTimer = setInterval(function() {
        loadMode();
        // F9: Refresh alert banner so new quota alerts appear without page reload
        loadSystemAlerts();
        // Also refresh activity log if settings tab is active
        var activeTab = document.querySelector('.tab-btn.active');
        if (activeTab && activeTab.getAttribute('data-tab') === 'settings') {
          loadActivityLog();
        }
      }, 30000);
    }
  }).catch(function() {});
}

// ════════════════════════════════════════════
//  DATA SOURCES
// ════════════════════════════════════════════

function loadDataSources() {
  fetch('/api/mode').then(function(r) { return r.json(); })
  .then(function(data) {
    var container = document.getElementById('data-sources-list');
    if (!data.sources || data.sources.length === 0) {
      container.innerHTML = '';
      return;
    }
    var html = '<div style="font-size:12px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;margin-top:4px">Data Sources</div>';
    data.sources.forEach(function(src) {
      var meta = src.captureCount + ' captures';
      if (src.lastCapture) {
        meta += ' · Last: ' + formatTimeAgo(src.lastCapture);
      }
      html += '<div class="data-source-item">' +
        '<div class="data-source-info">' +
          '<span class="data-source-name">' + esc(src.name) + '</span>' +
          '<span class="data-source-meta">' + esc(src.sourceType) + ' · ' + meta + '</span>' +
        '</div>' +
        '<span class="data-source-status ' + (src.enabled ? 'enabled' : 'disabled') + '">' +
          (src.enabled ? '● Active' : '○ Disabled') +
        '</span>' +
      '</div>';
    });
    container.innerHTML = html;
  }).catch(function() {});
}



// ════════════════════════════════════════════
//  ACTIVITY LOG
// ════════════════════════════════════════════

function loadActivityLog() {
  var filter = document.getElementById('activity-filter').value;
  var url = '/api/activity?limit=50';
  if (filter) url += '&type=' + filter;

  fetch(url).then(function(r) { return r.json(); })
  .then(function(data) {
    var container = document.getElementById('activity-log');
    if (!data.entries || data.entries.length === 0) {
      container.innerHTML = '<div class="activity-empty">No activity' +
        (filter ? ' for "' + filter + '"' : '') + ' yet</div>';
      return;
    }

    var html = '';
    data.entries.forEach(function(entry) {
      var time = entry.timestamp ? entry.timestamp.replace('T', ' ').substring(5, 16) : '';
      var detail = formatActivityDetail(entry);
      html += '<div class="activity-entry">' +
        '<span class="activity-time">' + time + '</span>' +
        '<span class="activity-type ' + esc(entry.eventType) + '">' +
          esc(entry.eventType.replace(/_/g, ' ')) +
        '</span>' +
        '<span class="activity-detail">' + detail + '</span>' +
      '</div>';
    });
    container.innerHTML = html;
  }).catch(function() {
    document.getElementById('activity-log').innerHTML =
      '<div class="activity-empty">Failed to load activity log</div>';
  });
}

function formatActivityDetail(entry) {
  try {
    var d = JSON.parse(entry.details || '{}');
    switch (entry.eventType) {
      case 'snap':
        return esc(entry.accountEmail || '') +
          (d.method ? ' · ' + d.method : '') +
          (d.source ? ' via ' + d.source : '');
      case 'snap_failed':
        return esc(d.error || 'Unknown error');
      case 'config_change':
        return esc(d.key || '') + ': ' + esc(d.from || '""') + ' → ' + esc(d.to || '""');
      case 'server_start':
        return 'Port ' + (d.port || '?') + ' · ' + esc(d.mode || 'manual') + ' mode';
      case 'sub_created':
      case 'sub_deleted':
        return esc(d.platform || '');
      case 'auto_link':
        return esc(entry.accountEmail || '') + ' → ' + esc(d.platform || '');
      case 'codex_snap':
        // T2: Truncate UUID for readability
        var acctId = entry.accountEmail || '';
        if (acctId.length > 20) acctId = acctId.substring(0, 6) + '..' + acctId.slice(-6);
        return esc(acctId) + (d.plan ? ' (' + esc(d.plan) + ')' : '');
      case 'model_reset':
        return esc(entry.accountEmail || '');
      case 'quota_alert':
        return '🔔 ' + esc(d.model || '') + ' — ' + (d.remainingPct != null ? d.remainingPct.toFixed(1) + '% remaining' : '');
      default:
        return entry.accountEmail ? esc(entry.accountEmail) : '';
    }
  } catch(e) {
    return '';
  }
}

// ════════════════════════════════════════════
//  MODEL PRICING CONFIG (Phase 13: F5)
// ════════════════════════════════════════════

var pricingDataCache = null;

function loadModelPricing() {
  fetch('/api/config/pricing').then(function(res) { return res.json(); })
  .then(function(data) {
    pricingDataCache = data.pricing || [];
    renderPricingTable(pricingDataCache);
  }).catch(function(err) {
    console.error('Failed to load model pricing:', err);
  });
}

function renderPricingTable(pricing) {
  var tbody = document.getElementById('pricing-tbody');
  if (!tbody) return;

  var providerIcons = { anthropic: '🟤', openai: '🟢', google: '🔵' };

  var html = '';
  for (var i = 0; i < pricing.length; i++) {
    var p = pricing[i];
    var providerCls = p.provider || 'custom';
    var providerLabel = p.provider ? (p.provider.charAt(0).toUpperCase() + p.provider.slice(1)) : 'Custom';
    var icon = providerIcons[p.provider] || '⚪';

    html += '<tr data-pricing-idx="' + i + '">' +
      '<td><span class="pricing-model-name">' + esc(p.displayName) + '</span></td>' +
      '<td><span class="pricing-provider ' + esc(providerCls) + '">' + icon + ' ' + esc(providerLabel) + '</span></td>' +
      '<td style="text-align:right"><input type="number" class="pricing-input" data-field="inputPer1M" step="0.01" min="0" value="' + p.inputPer1M + '"></td>' +
      '<td style="text-align:right"><input type="number" class="pricing-input" data-field="outputPer1M" step="0.01" min="0" value="' + p.outputPer1M + '"></td>' +
      '<td style="text-align:right"><input type="number" class="pricing-input" data-field="cachePer1M" step="0.001" min="0" value="' + p.cachePer1M + '"></td>' +
      '<td><button class="pricing-delete-btn" data-pricing-del="' + i + '" title="Remove this model">✕</button></td>' +
      '</tr>';
  }

  tbody.innerHTML = html;

  // Wire change handlers on inputs
  tbody.querySelectorAll('.pricing-input').forEach(function(input) {
    input.addEventListener('change', function() {
      var tr = input.closest('tr');
      var idx = parseInt(tr.dataset.pricingIdx);
      var field = input.dataset.field;
      var val = parseFloat(input.value) || 0;
      if (val < 0) val = 0;
      input.value = val;
      if (pricingDataCache && pricingDataCache[idx]) {
        pricingDataCache[idx][field] = val;
        savePricingFromTable();
      }
    });
  });

  // Wire delete buttons
  tbody.querySelectorAll('.pricing-delete-btn').forEach(function(btn) {
    btn.addEventListener('click', function() {
      var idx = parseInt(btn.dataset.pricingDel);
      deletePricingRow(idx);
    });
  });
}

function savePricingFromTable() {
  if (!pricingDataCache || pricingDataCache.length === 0) return;

  fetch('/api/config/pricing', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pricing: pricingDataCache })
  })
  .then(function(res) { return res.json(); })
  .then(function(data) {
    if (data.error) {
      showToast('❌ ' + data.error, 'error');
      return;
    }
    showToast('💰 Pricing saved', 'success');
  })
  .catch(function() { showToast('❌ Failed to save pricing', 'error'); });
}

function addPricingRow() {
  if (!pricingDataCache) pricingDataCache = [];

  var newModel = {
    modelId: 'custom-' + Date.now(),
    displayName: 'New Model',
    provider: 'custom',
    inputPer1M: 1.00,
    outputPer1M: 5.00,
    cachePer1M: 0.10
  };
  pricingDataCache.push(newModel);
  renderPricingTable(pricingDataCache);

  // Focus the name cell of the new row for editing
  var tbody = document.getElementById('pricing-tbody');
  var lastRow = tbody.lastElementChild;
  if (lastRow) {
    var nameCell = lastRow.querySelector('.pricing-model-name');
    if (nameCell) {
      // Make name editable inline
      nameCell.contentEditable = 'true';
      nameCell.focus();
      // Select all text for quick replace
      var range = document.createRange();
      range.selectNodeContents(nameCell);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);

      nameCell.addEventListener('blur', function() {
        nameCell.contentEditable = 'false';
        var idx = parseInt(lastRow.dataset.pricingIdx);
        var newName = nameCell.textContent.trim();
        if (newName && pricingDataCache[idx]) {
          pricingDataCache[idx].displayName = newName;
          pricingDataCache[idx].modelId = newName.toLowerCase().replace(/[^a-z0-9]+/g, '-');
          savePricingFromTable();
        }
      }, { once: true });

      nameCell.addEventListener('keydown', function(e) {
        if (e.key === 'Enter') {
          e.preventDefault();
          nameCell.blur();
        }
      });
    }
  }

  showToast('💰 New model added — edit the name and prices', 'info');
}

function deletePricingRow(idx) {
  if (!pricingDataCache || idx < 0 || idx >= pricingDataCache.length) return;

  var name = pricingDataCache[idx].displayName;
  if (!confirm('Remove pricing for "' + name + '"?')) return;

  pricingDataCache.splice(idx, 1);
  renderPricingTable(pricingDataCache);
  savePricingFromTable();
  showToast('🗑️ Removed ' + name, 'success');
}

function resetPricingDefaults() {
  if (!confirm('Reset all model pricing to current market defaults? This will overwrite your custom prices.')) return;

  // Fetch defaults from API by deleting the config key and re-fetching
  // We can't easily get defaults from the backend without a dedicated endpoint,
  // so we'll use the hardcoded defaults matching the backend.
  var defaults = [
    { modelId: 'claude-opus-4.6', displayName: 'Claude Opus 4.6', provider: 'anthropic', inputPer1M: 5.00, outputPer1M: 25.00, cachePer1M: 0.50 },
    { modelId: 'claude-sonnet-4.6', displayName: 'Claude Sonnet 4.6', provider: 'anthropic', inputPer1M: 3.00, outputPer1M: 15.00, cachePer1M: 0.30 },
    { modelId: 'claude-haiku-4.5', displayName: 'Claude Haiku 4.5', provider: 'anthropic', inputPer1M: 1.00, outputPer1M: 5.00, cachePer1M: 0.10 },
    { modelId: 'gpt-4o', displayName: 'GPT-4o', provider: 'openai', inputPer1M: 2.50, outputPer1M: 10.00, cachePer1M: 1.25 },
    { modelId: 'gemini-3.1-pro', displayName: 'Gemini 3.1 Pro', provider: 'google', inputPer1M: 2.00, outputPer1M: 12.00, cachePer1M: 0.50 },
    { modelId: 'gemini-2.5-flash', displayName: 'Gemini 2.5 Flash', provider: 'google', inputPer1M: 0.30, outputPer1M: 2.50, cachePer1M: 0.075 }
  ];

  pricingDataCache = defaults;
  renderPricingTable(pricingDataCache);

  fetch('/api/config/pricing', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pricing: defaults })
  })
  .then(function(res) { return res.json(); })
  .then(function(data) {
    if (data.error) {
      showToast('❌ ' + data.error, 'error');
      return;
    }
    showToast('↻ Pricing reset to defaults', 'success');
  })
  .catch(function() { showToast('❌ Failed to reset pricing', 'error'); });
}


// ════════════════════════════════════════════
//  KEYBOARD SHORTCUTS
// ════════════════════════════════════════════

function initKeyboardShortcuts() {
  document.addEventListener('keydown', function(e) {
    // Skip if user is typing in an input/textarea/select
    var tag = document.activeElement.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
      if (e.key === 'Escape') {
        document.activeElement.blur();
        closeModal();
        closeDelete();
        closeBudget();
      }
      return;
    }

    // Don't fire shortcuts when modals are open (except Escape)
    var anyModal = !document.getElementById('modal-overlay').hidden ||
                   !document.getElementById('delete-overlay').hidden ||
                   !document.getElementById('budget-overlay').hidden;

    if (e.key === 'Escape') {
      closeModal();
      closeDelete();
      closeBudget();
      return;
    }

    if (anyModal) return;

    switch (e.key) {
      case '1': switchToTab('quotas'); break;
      case '2': switchToTab('subscriptions'); break;
      case '3': switchToTab('overview'); break;
      case '4': switchToTab('settings'); break;
      case 'n': case 'N': openModal(); e.preventDefault(); break;
      case 's': case 'S': handleSnap(); e.preventDefault(); break;
      case '/':
        e.preventDefault();
        switchToTab('subscriptions');
        setTimeout(function() {
          var search = document.getElementById('search-subs');
          if (search) search.focus();
        }, 100);
        break;
    }
  });

  // Ctrl+K / Cmd+K for command palette
  document.addEventListener('keydown', function(e) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      toggleCommandPalette();
    }
  });
}



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

// ════════════════════════════════════════════
//  COMMAND PALETTE (Phase 9)
// ════════════════════════════════════════════

var PALETTE_COMMANDS = [
  { name: 'Snap Now',            key: 'S',    icon: '📸', action: function() { handleSnap(); } },
  { name: 'Show Quotas',         key: '1',    icon: '📊', action: function() { switchToTab('quotas'); } },
  { name: 'Show Subscriptions',  key: '2',    icon: '💳', action: function() { switchToTab('subscriptions'); } },
  { name: 'Show Overview',       key: '3',    icon: '📋', action: function() { switchToTab('overview'); } },
  { name: 'Show Settings',       key: '4',    icon: '⚙️', action: function() { switchToTab('settings'); } },
  { name: 'New Subscription',    key: 'N',    icon: '➕', action: function() { openModal(); } },
  { name: 'Toggle Auto-Capture',              icon: '🔄', action: function() {
    var el = document.getElementById('s-auto-capture');
    if (el) { el.checked = !el.checked; el.dispatchEvent(new Event('change')); }
  }},
  { name: 'Export CSV',                       icon: '📥', action: function() { window.location.href = '/api/export/csv'; } },
  { name: 'Export JSON',                      icon: '📦', action: function() { window.location.href = '/api/export/json'; } },
  { name: 'Download Backup',                  icon: '💾', action: function() { window.location.href = '/api/backup'; } },
  { name: 'Search Subscriptions', key: '/',   icon: '🔍', action: function() {
    switchToTab('subscriptions');
    setTimeout(function() { var s = document.getElementById('search-subs'); if (s) s.focus(); }, 100);
  }},
  { name: 'Set Budget',                       icon: '💰', action: function() { openBudgetModal(); } },
  { name: 'Toggle Theme',                     icon: '🌓', action: function() {
    var cur = document.documentElement.getAttribute('data-theme');
    var next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('niyantra-theme', next);
    var themeEl = document.getElementById('s-theme');
    if (themeEl) themeEl.value = next;
    updateChartTheme(next);
  }},
  { name: 'Codex Snap',                        icon: '🤖', action: function() { handleCodexSnap(); } },
  { name: 'Import JSON',                       icon: '📥', action: function() {
    var f = document.getElementById('import-file');
    if (f) f.click();
  }},
];

var paletteSelectedIndex = 0;
var paletteFilteredCommands = PALETTE_COMMANDS;

function initCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  var search = document.getElementById('command-palette-search');
  if (!overlay || !search) return;

  overlay.addEventListener('click', function(e) {
    if (e.target === overlay) closeCommandPalette();
  });

  search.addEventListener('input', function() {
    var query = search.value.toLowerCase().trim();
    paletteFilteredCommands = PALETTE_COMMANDS.filter(function(cmd) {
      return cmd.name.toLowerCase().indexOf(query) >= 0;
    });
    paletteSelectedIndex = 0;
    renderPaletteList();
  });

  search.addEventListener('keydown', function(e) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      paletteSelectedIndex = Math.min(paletteSelectedIndex + 1, paletteFilteredCommands.length - 1);
      renderPaletteList();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      paletteSelectedIndex = Math.max(paletteSelectedIndex - 1, 0);
      renderPaletteList();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (paletteFilteredCommands[paletteSelectedIndex]) {
        closeCommandPalette();
        paletteFilteredCommands[paletteSelectedIndex].action();
      }
    } else if (e.key === 'Escape') {
      closeCommandPalette();
    }
  });
}

function toggleCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  if (overlay.hidden) {
    openCommandPalette();
  } else {
    closeCommandPalette();
  }
}

function openCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  var search = document.getElementById('command-palette-search');
  overlay.hidden = false;
  search.value = '';
  paletteFilteredCommands = PALETTE_COMMANDS;
  paletteSelectedIndex = 0;
  renderPaletteList();
  setTimeout(function() { search.focus(); }, 50);
}

function closeCommandPalette() {
  document.getElementById('command-palette-overlay').hidden = true;
}

function renderPaletteList() {
  var list = document.getElementById('command-palette-list');
  if (paletteFilteredCommands.length === 0) {
    list.innerHTML = '<div class="command-palette-empty">No matching commands</div>';
    return;
  }
  var html = '';
  for (var i = 0; i < paletteFilteredCommands.length; i++) {
    var cmd = paletteFilteredCommands[i];
    var sel = i === paletteSelectedIndex ? ' selected' : '';
    html += '<div class="command-palette-item' + sel + '" data-idx="' + i + '">' +
      '<span class="cp-icon">' + cmd.icon + '</span>' +
      '<span class="cp-name">' + esc(cmd.name) + '</span>' +
      (cmd.key ? '<span class="cp-shortcut">' + cmd.key + '</span>' : '') +
      '</div>';
  }
  list.innerHTML = html;

  // Click handlers
  list.querySelectorAll('.command-palette-item').forEach(function(el) {
    el.addEventListener('click', function() {
      var idx = parseInt(el.getAttribute('data-idx'));
      closeCommandPalette();
      paletteFilteredCommands[idx].action();
    });
  });

  // Scroll selected into view
  var selected = list.querySelector('.selected');
  if (selected) selected.scrollIntoView({ block: 'nearest' });
}

// ════════════════════════════════════════════
//  CLAUDE CODE BRIDGE (Phase 9)
// ════════════════════════════════════════════

function loadClaudeBridgeStatus() {
  fetch('/api/claude/status').then(function(r) { return r.json(); })
  .then(function(data) {
    var statusEl = document.getElementById('claude-bridge-status');
    if (!statusEl) return;

    var bridgeOn = data.bridgeEnabled;
    var installed = data.installed;

    if (!bridgeOn) {
      statusEl.style.display = 'none';
      return;
    }

    var msg = '';
    if (!installed) {
      msg = '⚠️ Claude Code not detected (~/.claude/ not found)';
    } else if (data.bridgeFresh) {
      msg = '<span class="claude-bridge-dot"></span> Bridge active';
      if (data.snapshot) {
        msg += ' · 5h: ' + data.snapshot.fiveHourPct.toFixed(1) + '% used';
      }
    } else if (data.snapshot) {
      msg = '<span class="claude-bridge-dot stale"></span> Last data: ' + formatTimeAgo(data.snapshot.capturedAt);
    } else {
      msg = '<span class="claude-bridge-dot off"></span> Waiting for Claude Code statusline data...';
    }

    statusEl.innerHTML = msg;
    statusEl.style.display = '';
  }).catch(function() {});
}

function renderClaudeCodeCard() {
  return '<div class="claude-card" id="claude-code-card">' +
    '<h3>🔗 Claude Code</h3>' +
    '<div id="claude-card-body"><div class="empty-hint">Loading...</div></div>' +
    '</div>';
}

function loadClaudeCardData() {
  fetch('/api/claude/status').then(function(r) { return r.json(); })
  .then(function(data) {
    var body = document.getElementById('claude-card-body');
    if (!body) return;

    if (!data.snapshot) {
      body.innerHTML = '<div class="empty-hint">No Claude Code data yet. Start a Claude Code session to see rate limits.</div>';
      return;
    }

    var snap = data.snapshot;
    var html = '';

    // 5-hour meter
    var fiveColor = meterColor(snap.fiveHourPct);
    var fiveReset = snap.fiveHourReset ? '↻ ' + formatResetTime(snap.fiveHourReset) : '';
    html += '<div class="claude-meter">' +
      '<span class="claude-meter-label">5-Hour</span>' +
      '<div class="claude-meter-track"><div class="claude-meter-fill" style="width:' + snap.fiveHourPct + '%;background:' + fiveColor + '"></div></div>' +
      '<span class="claude-meter-pct" style="color:' + fiveColor + '">' + snap.fiveHourPct.toFixed(1) + '%</span>' +
      '<span class="claude-meter-reset">' + fiveReset + '</span>' +
      '</div>';

    // 7-day meter (if available)
    if (snap.sevenDayPct !== undefined) {
      var sevenColor = meterColor(snap.sevenDayPct);
      var sevenReset = snap.sevenDayReset ? '↻ ' + formatResetTime(snap.sevenDayReset) : '';
      html += '<div class="claude-meter">' +
        '<span class="claude-meter-label">7-Day</span>' +
        '<div class="claude-meter-track"><div class="claude-meter-fill" style="width:' + snap.sevenDayPct + '%;background:' + sevenColor + '"></div></div>' +
        '<span class="claude-meter-pct" style="color:' + sevenColor + '">' + snap.sevenDayPct.toFixed(1) + '%</span>' +
        '<span class="claude-meter-reset">' + sevenReset + '</span>' +
        '</div>';
    }

    // Bridge status badge
    var dotCls = data.bridgeFresh ? '' : 'stale';
    var agoStr = formatTimeAgo(snap.capturedAt);
    html += '<div class="claude-bridge-badge">' +
      '<span class="claude-bridge-dot ' + dotCls + '"></span>' +
      'Bridge ' + (data.bridgeFresh ? 'active' : 'stale') + ' · Last: ' + agoStr +
      '</div>';

    body.innerHTML = html;
  }).catch(function() {});
}

function meterColor(pct) {
  if (pct >= 80) return 'var(--red)';
  if (pct >= 50) return 'var(--amber)';
  return 'var(--green)';
}



// ════════════════════════════════════════════
//  Phase 10: SYSTEM ALERTS
// ════════════════════════════════════════════

function loadSystemAlerts() {
  fetch('/api/alerts').then(function(r) { return r.json(); })
  .then(function(data) {
    var container = document.getElementById('alert-banner-container');
    if (!container) return;
    var alerts = data.alerts || [];
    if (alerts.length === 0) {
      container.innerHTML = '';
      return;
    }
    var html = '';
    var shown = Math.min(alerts.length, 3);
    for (var i = 0; i < shown; i++) {
      var a = alerts[i];
      var icon = a.severity === 'critical' ? '🚨' : (a.severity === 'warning' ? '⚠️' : 'ℹ️');
      html += '<div class="alert-banner ' + esc(a.severity) + '">' +
        '<span class="alert-banner-icon">' + icon + '</span>' +
        '<div class="alert-banner-content">' +
        '<div class="alert-banner-title">' + esc(a.category) + '</div>' +
        '<div class="alert-banner-msg">' + esc(a.message) + '</div>' +
        '</div>' +
        '<button class="alert-banner-dismiss" onclick="dismissAlert(' + a.id + ')" title="Dismiss">&times;</button>' +
        '</div>';
    }
    if (alerts.length > 3) {
      html += '<div class="alert-more-link" onclick="switchToTab(\'overview\')">' +
        '+ ' + (alerts.length - 3) + ' more alert(s)</div>';
    }
    container.innerHTML = html;
  }).catch(function() {});
}

function dismissAlert(id) {
  fetch('/api/alerts/dismiss', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: id })
  }).then(function() {
    loadSystemAlerts();
    showToast('Alert dismissed', 'success');
  }).catch(function() {
    showToast('Failed to dismiss alert', 'error');
  });
}

// ════════════════════════════════════════════
//  PHASE 11: CODEX & SESSIONS
// ════════════════════════════════════════════

// ── Codex Settings Status ──
function loadCodexSettingsStatus() {
  var statusEl = document.getElementById('codex-status-settings');
  if (!statusEl) return;

  fetch('/api/codex/status').then(function(r) { return r.json(); })
  .then(function(data) {
    statusEl.style.display = '';
    if (!data.installed) {
      statusEl.innerHTML = '<span style="color:var(--text-muted)">⚠️ Codex CLI not detected. ' +
        'Install <a href="https://github.com/openai/codex" target="_blank" style="color:var(--accent)">Codex</a> ' +
        'and run <code>codex auth</code> to enable.</span>';
      return;
    }
    var tokenStatus = data.tokenExpired ?
      '<span style="color:var(--warning)">⚠️ Token expired — will auto-refresh on next poll</span>' :
      '<span style="color:var(--success)">✅ Token valid (expires ' + (data.tokenExpiresIn || '?') + ')</span>';
    var displayId = data.email || (data.accountId && data.accountId.length > 12 ? data.accountId.substring(0,6) + '…' + data.accountId.slice(-6) : (data.accountId || 'unknown'));
    statusEl.innerHTML =
      '🤖 Codex detected · Account: <strong>' + esc(displayId) + '</strong><br>' +
      tokenStatus;
    if (data.snapshot) {
      statusEl.innerHTML += '<br>Latest: <strong>' + data.snapshot.fiveHourPct.toFixed(1) + '%</strong> used (5h) · ' +
        '<span style="color:var(--text-muted)">' + formatTimeAgo(data.snapshot.capturedAt) + '</span>';
    }
  })
  .catch(function() {
    statusEl.style.display = 'none';
  });
}

// ── Codex Manual Snap ──
function handleCodexSnap() {
  showToast('🤖 Capturing Codex snapshot...', 'info');
  fetch('/api/codex/snap', { method: 'POST' })
  .then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.error) {
      showToast('❌ ' + data.error, 'error');
      return;
    }
    showToast('🤖 Codex snapshot captured! Plan: ' + (data.plan || 'unknown'), 'success');
    loadCodexSettingsStatus();
    loadOverview();
  })
  .catch(function() { showToast('❌ Codex snap failed', 'error'); });
}

// ── Codex Status Card (for Overview tab) ──
function renderCodexCard(container) {
  fetch('/api/codex/status').then(function(r) { return r.json(); })
  .then(function(data) {
    if (!data.installed && !data.snapshot) return; // Don't show card if no Codex at all

    var html = '<div class="overview-card codex-card">';
    html += '<div class="card-header"><h3>🤖 Codex / ChatGPT</h3>';
    html += '<button class="btn-add" onclick="handleCodexSnap()" style="padding:4px 10px;font-size:11px">📸 Snap</button>';
    html += '</div>';

    if (!data.installed) {
      html += '<div class="card-body"><span style="color:var(--text-muted)">Codex not detected</span></div>';
    } else if (!data.snapshot) {
      html += '<div class="card-body"><span style="color:var(--text-muted)">No snapshots yet — click Snap to capture</span></div>';
    } else {
      var snap = data.snapshot;
      var fivePct = snap.fiveHourPct || 0;
      var sevenPct = snap.sevenDayPct || null;
      var reviewPct = snap.codeReviewPct || null;
      var statusClass = fivePct >= 95 ? 'critical' : fivePct >= 80 ? 'danger' : fivePct >= 50 ? 'warning' : 'healthy';

      html += '<div class="card-body">';
      // 5-hour quota bar
      html += '<div class="codex-quota">';
      html += '<div class="codex-quota-label">5-Hour Window</div>';
      html += '<div class="quota-bar-track"><div class="quota-bar-fill ' + statusClass + '" style="width:' + Math.min(fivePct, 100) + '%"></div></div>';
      html += '<div class="codex-quota-value ' + statusClass + '">' + fivePct.toFixed(1) + '% used</div>';
      html += '</div>';

      // 7-day quota bar (optional)
      if (sevenPct !== null) {
        var sevenClass = sevenPct >= 95 ? 'critical' : sevenPct >= 80 ? 'danger' : sevenPct >= 50 ? 'warning' : 'healthy';
        html += '<div class="codex-quota">';
        html += '<div class="codex-quota-label">7-Day Window</div>';
        html += '<div class="quota-bar-track"><div class="quota-bar-fill ' + sevenClass + '" style="width:' + Math.min(sevenPct, 100) + '%"></div></div>';
        html += '<div class="codex-quota-value ' + sevenClass + '">' + sevenPct.toFixed(1) + '% used</div>';
        html += '</div>';
      }

      // Code review bar (optional)
      if (reviewPct !== null) {
        var revClass = reviewPct >= 95 ? 'critical' : reviewPct >= 80 ? 'danger' : reviewPct >= 50 ? 'warning' : 'healthy';
        html += '<div class="codex-quota">';
        html += '<div class="codex-quota-label">Code Review</div>';
        html += '<div class="quota-bar-track"><div class="quota-bar-fill ' + revClass + '" style="width:' + Math.min(reviewPct, 100) + '%"></div></div>';
        html += '<div class="codex-quota-value ' + revClass + '">' + reviewPct.toFixed(1) + '% used</div>';
        html += '</div>';
      }

      // Meta info
      html += '<div class="codex-meta">';
      html += '<span>Plan: <strong>' + esc(snap.planType || '—') + '</strong></span>';
      if (snap.creditsBalance !== null && snap.creditsBalance !== undefined) {
        html += '<span>Credits: <strong>' + snap.creditsBalance.toFixed(2) + '</strong></span>';
      }
      html += '<span>Captured: ' + formatTimeAgo(snap.capturedAt) + '</span>';
      html += '<span class="codex-provenance">' + esc(snap.captureMethod) + '/' + esc(snap.captureSource) + '</span>';
      html += '</div>';
      html += '</div>';
    }
    html += '</div>';

    // Insert before the calendar or at end
    var existing = container.querySelector('.codex-card');
    if (existing) {
      existing.outerHTML = html;
    } else {
      container.insertAdjacentHTML('afterbegin', html);
    }
  })
  .catch(function() {}); // Silently fail
}

// ── Sessions Timeline (for Overview tab) ──
function renderSessionsTimeline(container) {
  fetch('/api/sessions?limit=10').then(function(r) { return r.json(); })
  .then(function(data) {
    if (!data.sessions || data.sessions.length === 0) return;

    var html = '<div class="overview-card sessions-card">';
    html += '<div class="card-header"><h3>⏱️ Usage Sessions</h3>';
    html += '<span class="card-count">' + data.count + ' sessions</span>';
    html += '</div>';
    html += '<div class="card-body">';
    html += '<div class="session-timeline">';

    for (var i = 0; i < data.sessions.length; i++) {
      var sess = data.sessions[i];
      var isActive = !sess.endedAt;
      var duration = isActive ?
        formatDurationSec(Math.floor((Date.now() - new Date(sess.startedAt).getTime()) / 1000)) :
        formatDurationSec(sess.durationSec);
      var providerIcon = sess.provider === 'codex' ? '🤖' :
                         sess.provider === 'claude' ? '🔮' : '⚡';

      html += '<div class="session-item' + (isActive ? ' active' : '') + '">';
      html += '<div class="session-dot' + (isActive ? ' pulse' : '') + '"></div>';
      html += '<div class="session-content">';
      html += '<div class="session-top">';
      html += '<span class="session-provider">' + providerIcon + ' ' + esc(sess.provider) + '</span>';
      html += '<span class="session-duration">' + duration + '</span>';
      html += '</div>';
      html += '<div class="session-bottom">';
      html += '<span class="session-time">' + formatTimeAgo(sess.startedAt) + '</span>';
      html += '<span class="session-snaps">' + sess.snapCount + ' snaps</span>';
      if (isActive) html += '<span class="session-active-badge">LIVE</span>';
      html += '</div>';
      html += '</div></div>';
    }

    html += '</div></div></div>';

    // Insert after codex card or at start
    var codexCard = container.querySelector('.codex-card');
    var existing = container.querySelector('.sessions-card');
    if (existing) {
      existing.outerHTML = html;
    } else if (codexCard) {
      codexCard.insertAdjacentHTML('afterend', html);
    } else {
      container.insertAdjacentHTML('afterbegin', html);
    }
  })
  .catch(function() {});
}

// ── IIFE Safety: Expose functions used by inline onclick="..." HTML attributes ──
window.openBudgetModal = openBudgetModal;
window.dismissAlert = dismissAlert;
window.switchToTab = switchToTab;
window.calendarNav = calendarNav;
window.handleCodexSnap = handleCodexSnap;

