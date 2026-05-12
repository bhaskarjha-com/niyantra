// Niyantra Dashboard — Claude Code Bridge
import { esc, formatTimeAgo } from '../core/utils.js';
import { formatResetTime } from '../quotas/render.js';


export function loadClaudeBridgeStatus() {
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

export function renderClaudeCodeCard() {
  return '<div class="claude-card" id="claude-code-card">' +
    '<h3>🔗 Claude Code</h3>' +
    '<div id="claude-card-body"><div class="empty-hint">Loading...</div></div>' +
    '</div>';
}

export function loadClaudeCardData() {
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

export function meterColor(pct) {
  if (pct >= 80) return 'var(--red)';
  if (pct >= 50) return 'var(--amber)';
  return 'var(--green)';
}



// ════════════════════════════════════════════
