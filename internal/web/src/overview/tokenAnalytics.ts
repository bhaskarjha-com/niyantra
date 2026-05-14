// Niyantra Dashboard — Token Usage Analytics (F13)
// Renders KPI cards, model distribution donut, and daily burn chart.

export function loadTokenAnalytics(): void {
  var container = document.getElementById('token-analytics-container');
  if (!container) return;

  // Get time range from selector, default to 30 days
  var rangeSelector = document.getElementById('token-range-selector') as HTMLSelectElement | null;
  var days = 30;
  if (rangeSelector) {
    days = parseInt(rangeSelector.value) || 30;
  }

  fetch('/api/token-usage?days=' + days).then(function(res) {
    return res.json();
  }).then(function(data: any) {
    renderTokenAnalytics(container!, data, days);
  }).catch(function(err) {
    console.error('Token analytics fetch failed:', err);
    container!.innerHTML = '<div class="token-analytics-empty">Failed to load token analytics</div>';
  });
}

function renderTokenAnalytics(container: HTMLElement, data: any, days: number): void {
  if (!data || !data.totals || data.totals.totalTokens === 0) {
    container.innerHTML = '<div class="overview-card full-width token-analytics-card">' +
      '<h3>🔥 Token Usage Analytics</h3>' +
      '<div class="token-analytics-empty">' +
      '<p>No token usage data available yet.</p>' +
      '<p style="font-size:12px;color:var(--text-secondary)">Use Claude Code to generate token usage data. ' +
      'Session files are parsed from <code>~/.claude/projects/</code>.</p>' +
      '</div></div>';
    return;
  }

  var totals = data.totals;
  var kpis = data.kpis || {};
  var models = data.byModel || [];
  var dailyData = data.byDay || [];

  // ── Time Range Selector ──
  var rangeOptions = [
    { value: '7', label: '7d' },
    { value: '30', label: '30d' },
    { value: '90', label: '90d' },
    { value: '365', label: '1y' },
  ];
  var rangeHTML = '<div class="token-range-bar">';
  for (var i = 0; i < rangeOptions.length; i++) {
    var opt = rangeOptions[i];
    var activeClass = String(days) === opt.value ? ' token-range-active' : '';
    rangeHTML += '<button class="token-range-btn' + activeClass + '" data-days="' + opt.value + '">' + opt.label + '</button>';
  }
  rangeHTML += '</div>';

  // ── KPI Cards ──
  var kpiHTML = '<div class="token-kpi-row">';
  kpiHTML += buildKpiCard('Total Tokens', formatTokens(totals.totalTokens), '📊');
  kpiHTML += buildKpiCard('Est. Cost', '$' + (totals.estimatedCostUSD || 0).toFixed(2), '💰');
  kpiHTML += buildKpiCard('Active Days', String(kpis.daysActive || 0), '📅');
  kpiHTML += buildKpiCard('Avg/Day', formatTokens(kpis.avgTokensPerDay || 0), '📈');
  kpiHTML += buildKpiCard('Cache Rate', Math.round((kpis.cacheHitRate || 0) * 100) + '%', '⚡');
  kpiHTML += '</div>';

  // ── Token Breakdown Chips ──
  var chipsHTML = '<div class="token-breakdown-chips">';
  chipsHTML += '<span class="token-chip token-chip-input">Input: ' + formatTokens(totals.inputTokens) + '</span>';
  chipsHTML += '<span class="token-chip token-chip-output">Output: ' + formatTokens(totals.outputTokens) + '</span>';
  chipsHTML += '<span class="token-chip token-chip-cache">Cache: ' + formatTokens(totals.cacheTokens) + '</span>';
  if (totals.sessions > 0) {
    chipsHTML += '<span class="token-chip token-chip-sessions">Sessions: ' + totals.sessions + '</span>';
  }
  chipsHTML += '</div>';

  // ── Model Distribution (donut-like visualization) ──
  var modelHTML = '';
  if (models.length > 0) {
    modelHTML = '<div class="token-section">';
    modelHTML += '<h4>Model Distribution</h4>';
    modelHTML += '<div class="token-model-bars">';
    // Color palette for models
    var colors = ['#6366f1', '#8b5cf6', '#ec4899', '#f59e0b', '#10b981', '#3b82f6', '#ef4444'];
    var topModels = models.slice(0, 7); // Max 7 models
    for (var mi = 0; mi < topModels.length; mi++) {
      var model = topModels[mi];
      var color = colors[mi % colors.length];
      var pct = model.percentage || 0;
      var costLabel = model.costUSD > 0 ? ' · $' + model.costUSD.toFixed(2) : '';
      modelHTML += '<div class="token-model-row">' +
        '<div class="token-model-header">' +
        '<span class="token-model-name" style="color:' + color + '">' + escapeHtml(model.model) + '</span>' +
        '<span class="token-model-stats">' + formatTokens(model.totalTokens) + ' (' + pct.toFixed(1) + '%)' + costLabel + '</span>' +
        '</div>' +
        '<div class="token-model-bar-track">' +
        '<div class="token-model-bar-fill" style="width:' + pct + '%;background:' + color + '"></div>' +
        '</div></div>';
    }
    modelHTML += '</div></div>';
  }

  // ── Daily Burn Chart (sparkline bars) ──
  var chartHTML = '';
  if (dailyData.length > 0) {
    chartHTML = '<div class="token-section">';
    chartHTML += '<h4>Daily Token Burn</h4>';
    chartHTML += '<div class="token-daily-chart">';

    // Find max for scaling
    var maxTokens = 0;
    for (var di = 0; di < dailyData.length; di++) {
      if (dailyData[di].totalTokens > maxTokens) maxTokens = dailyData[di].totalTokens;
    }

    // Limit to last N days for display
    var displayDays = dailyData;
    if (displayDays.length > 60) {
      displayDays = displayDays.slice(displayDays.length - 60);
    }

    for (var dj = 0; dj < displayDays.length; dj++) {
      var day = displayDays[dj];
      var barHeight = maxTokens > 0 ? Math.max(2, (day.totalTokens / maxTokens) * 100) : 2;
      var inputPct = day.totalTokens > 0 ? (day.inputTokens / day.totalTokens) * barHeight : 0;
      var outputPct = barHeight - inputPct;
      var dayLabel = day.date.substring(5); // MM-DD

      chartHTML += '<div class="token-bar-col" title="' + day.date + ': ' + formatTokens(day.totalTokens) + ' tokens, $' + (day.costUSD || 0).toFixed(2) + '">' +
        '<div class="token-bar-stack" style="height:' + barHeight + '%">' +
        '<div class="token-bar-output" style="height:' + outputPct + '%"></div>' +
        '<div class="token-bar-input" style="height:' + inputPct + '%"></div>' +
        '</div>' +
        '<span class="token-bar-label">' + dayLabel + '</span>' +
        '</div>';
    }

    chartHTML += '</div>';
    chartHTML += '<div class="token-chart-legend">' +
      '<span class="token-legend-item"><span class="token-legend-dot" style="background:var(--token-input-color)"></span>Input</span>' +
      '<span class="token-legend-item"><span class="token-legend-dot" style="background:var(--token-output-color)"></span>Output</span>' +
      '</div>';
    chartHTML += '</div>';
  }

  // ── Peak Day badge ──
  var peakHTML = '';
  if (kpis.peakDay) {
    peakHTML = '<div class="token-peak-badge">🔥 Peak: ' + kpis.peakDay + ' — ' + formatTokens(kpis.peakDayTokens) + ' tokens</div>';
  }

  // ── Assemble ──
  container.innerHTML = '<div class="overview-card full-width token-analytics-card">' +
    '<div class="token-analytics-header">' +
    '<h3>🔥 Token Usage Analytics</h3>' +
    rangeHTML +
    '</div>' +
    kpiHTML + chipsHTML + peakHTML + modelHTML + chartHTML +
    '</div>';

  // Wire up range selector buttons
  var rangeBtns = container.querySelectorAll('.token-range-btn');
  for (var bi = 0; bi < rangeBtns.length; bi++) {
    rangeBtns[bi].addEventListener('click', function(this: HTMLElement) {
      var newDays = this.getAttribute('data-days') || '30';
      // Update active state
      var allBtns = container.querySelectorAll('.token-range-btn');
      for (var k = 0; k < allBtns.length; k++) allBtns[k].classList.remove('token-range-active');
      this.classList.add('token-range-active');
      // Re-fetch with new range
      fetch('/api/token-usage?days=' + newDays).then(function(res) { return res.json(); }).then(function(d: any) {
        renderTokenAnalytics(container, d, parseInt(newDays));
      });
    });
  }
}

function buildKpiCard(label: string, value: string, icon: string): string {
  return '<div class="token-kpi-card">' +
    '<div class="token-kpi-icon">' + icon + '</div>' +
    '<div class="token-kpi-value">' + value + '</div>' +
    '<div class="token-kpi-label">' + label + '</div>' +
    '</div>';
}

function formatTokens(n: number): string {
  if (n >= 1_000_000_000) return (n / 1_000_000_000).toFixed(1) + 'B';
  if (n >= 1_000_000) return (n / 1_000_000).toFixed(1) + 'M';
  if (n >= 1_000) return (n / 1_000).toFixed(1) + 'K';
  return String(n);
}

function escapeHtml(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;');
}
