// Niyantra Dashboard — Settings Panel
import { serverConfig, presetsData } from '../core/state.js';
import { showToast, formatTimeAgo } from '../core/utils.js';
import { fetchStatus } from '../core/api.js';
import { renderAccounts } from '../quotas/render.js';
import { updateConfig, loadConfig, setBudget, getBudget } from '../overview/budget.js';
import { loadOverview } from '../overview/overview.js';
import { loadMode } from './mode.js';
import { loadDataSources } from './data.js';
import { loadActivityLog } from './activity.js';
import { loadModelPricing, addPricingRow, resetPricingDefaults } from './pricing.js';
import { loadClaudeBridgeStatus } from '../advanced/claude.js';
import { loadCodexSettingsStatus } from '../advanced/codex.js';
import { loadSubscriptions } from '../subscriptions.js';
import { updateChartTheme } from '../charts/history.js';


export function initSettings() {
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
export function migrateLocalStorage(cfg) {
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
