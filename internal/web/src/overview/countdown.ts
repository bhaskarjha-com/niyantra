// Niyantra Dashboard — Reset Countdown Timers (F6-UX)
// Shows countdown chips for providers whose quota resets within 24h.

export function renderCountdowns(quotaData: any): string {
  if (!quotaData) return '';

  var items: { provider: string; label: string; resetMs: number }[] = [];

  // Antigravity accounts: use resetTime from readiness data
  if (quotaData.accounts) {
    for (var i = 0; i < quotaData.accounts.length; i++) {
      var acc = quotaData.accounts[i];
      if (acc.resetTime) {
        var resetDate = new Date(acc.resetTime);
        var ms = resetDate.getTime() - Date.now();
        if (ms > 0 && ms < 86400000) {
          items.push({
            provider: '⚡ Antigravity',
            label: acc.email ? acc.email.split('@')[0] : 'account',
            resetMs: ms,
          });
        }
      }
    }
  }

  // Claude: 5h window reset
  if (quotaData.claudeSnapshot) {
    var cs = quotaData.claudeSnapshot;
    if (cs.capturedAt && (cs.fiveHourPct || 0) > 50) {
      var fiveHReset = new Date(cs.capturedAt).getTime() + 5 * 3600000;
      var msLeft = fiveHReset - Date.now();
      if (msLeft > 0) {
        items.push({ provider: '🔮 Claude', label: '5h window', resetMs: msLeft });
      }
    }
  }

  // Codex: 7-day window
  if (quotaData.codexSnapshot) {
    var cx = quotaData.codexSnapshot;
    if (cx.capturedAt && (cx.sevenDayPct || 0) > 50) {
      var sevenDReset = new Date(cx.capturedAt).getTime() + 7 * 86400000;
      var cxMs = sevenDReset - Date.now();
      if (cxMs > 0 && cxMs < 86400000 * 2) {
        items.push({ provider: '🤖 Codex', label: '7d window', resetMs: cxMs });
      }
    }
  }

  if (items.length === 0) return '';

  // Sort by soonest reset
  items.sort(function(a, b) { return a.resetMs - b.resetMs; });

  var html = '<div class="countdown-strip">' +
    '<span class="countdown-title">⏱ Resets:</span>';
  for (var c = 0; c < Math.min(items.length, 4); c++) {
    var item = items[c];
    var h = Math.floor(item.resetMs / 3600000);
    var m = Math.floor((item.resetMs % 3600000) / 60000);
    var timeStr = h > 0 ? h + 'h ' + m + 'm' : m + 'm';
    html += '<div class="countdown-chip">' +
      '<span class="countdown-provider">' + item.provider + '</span>' +
      '<span class="countdown-time">' + timeStr + '</span>' +
      '</div>';
  }
  html += '</div>';
  return html;
}

// Live countdown refresh — call every 60s to update timers client-side
var countdownInterval: ReturnType<typeof setInterval> | null = null;

export function startCountdownRefresh(quotaData: any): void {
  if (countdownInterval) clearInterval(countdownInterval);
  countdownInterval = setInterval(function() {
    var container = document.getElementById('countdown-container');
    if (container) {
      container.innerHTML = renderCountdowns(quotaData);
    }
  }, 60000);
}
