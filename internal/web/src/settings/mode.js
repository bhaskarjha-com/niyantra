// Niyantra Dashboard — Mode Badge
import { presetsData } from '../core/state.js';
import { formatTimeAgo, formatPollInterval } from '../core/utils.js';
import { loadSystemAlerts } from '../advanced/alerts.js';
import { loadActivityLog } from './activity.js';


export var modeRefreshTimer = null;

export function loadMode() {
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
