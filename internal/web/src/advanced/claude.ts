// Niyantra Dashboard — Claude Code Bridge
import { esc, formatTimeAgo } from '../core/utils';
import { formatResetTime } from '../quotas/render';


export function loadClaudeBridgeStatus(): void {
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

export function renderClaudeCodeCard(): string {
  return '<div class="claude-card" id="claude-code-card">' +
    '<h3>🔗 Claude Code</h3>' +
    '<div id="claude-card-body"><div class="empty-hint">Loading...</div></div>' +
    '<div id="claude-deep-usage" class="claude-deep-section"></div>' +
    '</div>';
}

export function loadClaudeCardData(): void {
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

export function meterColor(pct: number): string {
  if (pct >= 80) return 'var(--red)';
  if (pct >= 50) return 'var(--amber)';
  return 'var(--green)';
}

// ── F15d: Deep Token Usage ──────────────────────────────────────

export function loadClaudeDeepUsage(): void {
  fetch('/api/claude/usage?days=30').then(function(r) { return r.json(); }).then(function(data) {
    var container = document.getElementById('claude-deep-usage');
    if (!container) return;

    if (!data || !data.days || data.days.length === 0) {
      container.innerHTML = '<div class="empty-hint">No Claude Code session data found. Start coding with Claude Code to see token analytics.</div>';
      return;
    }

    var html = '';

    // Summary stats row
    html += '<div class="claude-deep-stats">';
    html += '<div class="claude-deep-stat">' +
      '<span class="claude-deep-value">' + formatTokens(data.totalTokens) + '</span>' +
      '<span class="claude-deep-label">tokens (30d)</span>' +
    '</div>';
    html += '<div class="claude-deep-stat">' +
      '<span class="claude-deep-value">$' + (data.totalCost || 0).toFixed(2) + '</span>' +
      '<span class="claude-deep-label">est. cost</span>' +
    '</div>';
    html += '<div class="claude-deep-stat">' +
      '<span class="claude-deep-value">' + (data.totalSessions || 0) + '</span>' +
      '<span class="claude-deep-label">sessions</span>' +
    '</div>';
    html += '<div class="claude-deep-stat">' +
      '<span class="claude-deep-value">' + ((data.cacheHitRate || 0) * 100).toFixed(0) + '%</span>' +
      '<span class="claude-deep-label">cache hit</span>' +
    '</div>';
    html += '</div>';

    // Token breakdown: input vs output
    var totalIn = data.totalInput || 0;
    var totalOut = data.totalOutput || 0;
    var totalAll = totalIn + totalOut;
    if (totalAll > 0) {
      var inPct = (totalIn / totalAll * 100).toFixed(0);
      var outPct = (totalOut / totalAll * 100).toFixed(0);
      html += '<div class="claude-token-bar">' +
        '<div class="claude-token-in" style="width:' + inPct + '%">' +
          '<span>In ' + formatTokens(totalIn) + '</span>' +
        '</div>' +
        '<div class="claude-token-out" style="width:' + outPct + '%">' +
          '<span>Out ' + formatTokens(totalOut) + '</span>' +
        '</div>' +
      '</div>';
    }

    // Top model badge
    if (data.topModel) {
      html += '<div class="claude-deep-meta">' +
        '<span class="claude-deep-chip">🏆 ' + data.topModel + '</span>' +
      '</div>';
    }

    container.innerHTML = html;
  }).catch(function() {});
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return n.toString();
}

// ════════════════════════════════════════════

