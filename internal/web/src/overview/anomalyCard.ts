// Niyantra Dashboard — Anomaly Detection Card (F5-UX)
// Renders cost anomaly alerts with dismiss functionality.
// CSP-safe — no inline event handlers.

export function loadAnomalies(): void {
  var container = document.getElementById('anomaly-card-container');
  if (!container) return;

  fetch('/api/anomalies').then(function(res) { return res.json(); }).then(function(data: any) {
    if (!data || !data.anomalies || data.anomalies.length === 0) {
      container!.innerHTML = '';
      return;
    }

    var anomalies = data.anomalies as any[];

    // Check dismissed anomalies in localStorage
    var dismissedKey = 'niyantra_dismissed_anomalies';
    var dismissed: string[] = [];
    try { dismissed = JSON.parse(localStorage.getItem(dismissedKey) || '[]'); } catch(e) {}

    // Filter out dismissed
    anomalies = anomalies.filter(function(a: any) {
      return dismissed.indexOf(a.provider + '_' + new Date().toISOString().slice(0, 10)) < 0;
    });

    if (anomalies.length === 0) {
      container!.innerHTML = '';
      return;
    }

    var severityClass = anomalies[0].severity === 'critical' ? 'critical' : 'warning';

    var html = '<div class="anomaly-card ' + severityClass + '">' +
      '<div class="anomaly-header">' +
        '<span class="anomaly-title">' +
          (severityClass === 'critical' ? '🚨' : '⚠️') + ' Cost Anomaly Detected' +
        '</span>' +
        '<button class="anomaly-dismiss" id="anomaly-dismiss-btn" title="Dismiss for today">✕</button>' +
      '</div>';

    for (var i = 0; i < anomalies.length; i++) {
      var a = anomalies[i];
      html += '<div class="anomaly-item">' +
        '<div class="anomaly-provider">' +
          (a.severity === 'critical' ? '🔴' : '🟡') + ' ' + escHtml(a.message) +
        '</div>' +
        '<div class="anomaly-detail">Z-score: ' + a.zScore + 'σ · ' +
          '$' + a.currentValue.toFixed(2) + ' today vs $' + a.mean30d.toFixed(2) + ' avg (30d)</div>';

      if (a.projectedImpact > 0) {
        html += '<div class="anomaly-impact">📊 At this rate: budget exceeded by $' +
          a.projectedImpact.toFixed(2) + '/mo</div>';
      }
      html += '</div>';
    }

    html += '</div>';
    container!.innerHTML = html;

    // Wire dismiss button (CSP-safe)
    var dismissBtn = document.getElementById('anomaly-dismiss-btn');
    if (dismissBtn) {
      dismissBtn.addEventListener('click', function() {
        var today = new Date().toISOString().slice(0, 10);
        var keys = anomalies.map(function(a: any) { return a.provider + '_' + today; });
        var current: string[] = [];
        try { current = JSON.parse(localStorage.getItem(dismissedKey) || '[]'); } catch(e) {}
        for (var k = 0; k < keys.length; k++) {
          if (current.indexOf(keys[k]) < 0) current.push(keys[k]);
        }
        // Keep only last 7 days of dismissals
        if (current.length > 50) current = current.slice(-50);
        localStorage.setItem(dismissedKey, JSON.stringify(current));
        container!.innerHTML = '';
      });
    }
  }).catch(function(err) {
    console.error('Anomaly detection failed:', err);
  });
}

function escHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}
