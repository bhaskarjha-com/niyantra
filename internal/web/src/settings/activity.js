// Niyantra Dashboard — Activity Log
import { esc } from '../core/utils.js';


export function loadActivityLog() {
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

export function formatActivityDetail(entry) {
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
