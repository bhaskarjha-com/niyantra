// Niyantra Dashboard — Quota History Chart
// @ts-nocheck
var Chart = (window as any).Chart;


export var historyChart: any = null;

// M2: Update chart colors in-place on theme toggle (avoids destroy+rebuild flash)
export function updateChartTheme(theme: string): void {
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

export function loadHistoryChart(): void {
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

export function renderHistoryChart(snapshots: any[]): void {
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

export function populateChartAccountSelect(data: any): void {
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
