// Niyantra Dashboard — Shareable Spend Report (F16-UX)
// Generates a beautiful monthly AI spend summary as a downloadable PNG.
// Uses native Canvas API — zero dependencies.

interface ReportData {
  totalSpend: number;
  providerCount: number;
  accountCount: number;
  topCategory: { name: string; pct: number; spend: number } | null;
  activeDays: number;
  totalSnaps: number;
  currentStreak: number;
  longestStreak: number;
  period: string;
  dailyBurn: number[];
}

// Assemble report data from existing APIs
async function assembleReportData(): Promise<ReportData> {
  var results = await Promise.all([
    fetch('/api/overview').then(function(r) { return r.json(); }),
    fetch('/api/history/heatmap').then(function(r) { return r.json(); }),
  ]);

  var overview = results[0];
  var heatmap = results[1];

  var stats = overview.stats || {};
  var byCat = stats.byCategory || {};
  var catKeys = Object.keys(byCat);

  // Find top category
  var topCat: { name: string; pct: number; spend: number } | null = null;
  if (catKeys.length > 0) {
    catKeys.sort(function(a, b) { return (byCat[b].monthlySpend || 0) - (byCat[a].monthlySpend || 0); });
    var topName = catKeys[0];
    var topSpend = byCat[topName].monthlySpend || 0;
    var totalSpend = stats.totalMonthlySpend || 0;
    topCat = {
      name: topName,
      pct: totalSpend > 0 ? Math.round((topSpend / totalSpend) * 100) : 0,
      spend: topSpend
    };
  }

  // Build daily burn from heatmap
  var dailyBurn: number[] = [];
  if (heatmap && heatmap.days) {
    var dayKeys = Object.keys(heatmap.days).sort().slice(-30);
    for (var i = 0; i < dayKeys.length; i++) {
      dailyBurn.push(heatmap.days[dayKeys[i]] || 0);
    }
  }

  var now = new Date();
  var period = now.toLocaleString('default', { month: 'long', year: 'numeric' });

  return {
    totalSpend: stats.totalMonthlySpend || 0,
    providerCount: overview.providerCount || (overview.quotaSummary ? Object.keys(overview.quotaSummary.byProvider || {}).length : 0),
    accountCount: overview.accountCount || 0,
    topCategory: topCat,
    activeDays: heatmap ? (heatmap.activeDays || 0) : 0,
    totalSnaps: heatmap ? (heatmap.totalSnaps || 0) : 0,
    currentStreak: heatmap ? (heatmap.currentStreak || 0) : 0,
    longestStreak: heatmap ? (heatmap.longestStreak || 0) : 0,
    period: period,
    dailyBurn: dailyBurn
  };
}

// Main render pipeline
export async function generateReport(): Promise<Blob | null> {
  var data = await assembleReportData();

  var canvas = document.createElement('canvas');
  var dpr = window.devicePixelRatio || 1;
  var W = 1200;
  var H = 630;
  canvas.width = W * dpr;
  canvas.height = H * dpr;
  canvas.style.width = W + 'px';
  canvas.style.height = H + 'px';
  var ctx = canvas.getContext('2d')!;
  ctx.scale(dpr, dpr);

  // Background gradient
  var bg = ctx.createLinearGradient(0, 0, 0, H);
  bg.addColorStop(0, '#0a0f1a');
  bg.addColorStop(1, '#0f172a');
  ctx.fillStyle = bg;
  ctx.fillRect(0, 0, W, H);

  // Subtle grid pattern
  ctx.strokeStyle = 'rgba(255,255,255,0.02)';
  ctx.lineWidth = 1;
  for (var gy = 0; gy < H; gy += 40) {
    ctx.beginPath(); ctx.moveTo(0, gy); ctx.lineTo(W, gy); ctx.stroke();
  }

  drawHeader(ctx, data, W);
  drawHeroMetrics(ctx, data);
  drawStatCards(ctx, data, W);
  drawTrendBars(ctx, data.dailyBurn, W);
  drawFooter(ctx, data, W, H);

  return new Promise(function(resolve) {
    canvas.toBlob(function(blob) { resolve(blob); }, 'image/png');
  });
}

function drawHeader(ctx: CanvasRenderingContext2D, data: ReportData, W: number): void {
  // Logo
  ctx.font = '700 20px Inter, system-ui, -apple-system, sans-serif';
  ctx.fillStyle = '#6ee7b7';
  ctx.fillText('⚡ Niyantra', 40, 42);

  // Period
  ctx.font = '400 14px Inter, system-ui, sans-serif';
  ctx.fillStyle = '#64748b';
  ctx.textAlign = 'right';
  ctx.fillText(data.period + ' Report', W - 40, 42);
  ctx.textAlign = 'left';

  // Separator
  ctx.strokeStyle = 'rgba(255,255,255,0.06)';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(40, 60);
  ctx.lineTo(W - 40, 60);
  ctx.stroke();
}

function drawHeroMetrics(ctx: CanvasRenderingContext2D, data: ReportData): void {
  // Left: Total Spend
  ctx.font = '400 11px Inter, system-ui, sans-serif';
  ctx.fillStyle = '#64748b';
  ctx.letterSpacing = '1px';
  ctx.fillText('TOTAL AI SPEND', 50, 98);

  ctx.font = '700 48px Inter, system-ui, sans-serif';
  ctx.fillStyle = '#f1f5f9';
  ctx.fillText('$' + data.totalSpend.toFixed(2), 50, 155);

  // Right: Top Category
  if (data.topCategory) {
    ctx.font = '400 11px Inter, system-ui, sans-serif';
    ctx.fillStyle = '#64748b';
    ctx.fillText('TOP CATEGORY', 700, 98);

    ctx.font = '700 28px Inter, system-ui, sans-serif';
    ctx.fillStyle = '#f1f5f9';
    ctx.fillText(data.topCategory.name + ' (' + data.topCategory.pct + '%)', 700, 135);

    ctx.font = '400 16px Inter, system-ui, sans-serif';
    ctx.fillStyle = '#94a3b8';
    ctx.fillText('$' + data.topCategory.spend.toFixed(2) + '/mo', 700, 160);
  }
}

function drawStatCards(ctx: CanvasRenderingContext2D, data: ReportData, W: number): void {
  var cards = [
    { label: 'PROVIDERS', value: String(data.providerCount || '—'), color: '#6366f1' },
    { label: 'SNAPSHOTS', value: String(data.totalSnaps || '—'), color: '#10b981' },
    { label: 'ACTIVE DAYS', value: String(data.activeDays || '—'), color: '#3b82f6' },
    { label: 'BEST STREAK', value: data.longestStreak > 0 ? data.longestStreak + 'd' : '—', color: '#f59e0b' },
  ];

  var startX = 50;
  var cardW = (W - 100 - 60) / 4; // 4 cards with 20px gaps
  var gap = 20;
  var y = 200;
  var cardH = 80;

  for (var i = 0; i < cards.length; i++) {
    var x = startX + i * (cardW + gap);
    var c = cards[i];

    // Card bg
    ctx.fillStyle = '#131b2e';
    roundRect(ctx, x, y, cardW, cardH, 8);
    ctx.fill();

    // Accent bar
    ctx.fillStyle = c.color;
    roundRect(ctx, x, y, 3, cardH, 2);
    ctx.fill();

    // Label
    ctx.font = '500 10px Inter, system-ui, sans-serif';
    ctx.fillStyle = '#64748b';
    ctx.fillText(c.label, x + 16, y + 24);

    // Value
    ctx.font = '700 28px Inter, system-ui, sans-serif';
    ctx.fillStyle = '#f1f5f9';
    ctx.fillText(c.value, x + 16, y + 60);
  }
}

function drawTrendBars(ctx: CanvasRenderingContext2D, daily: number[], W: number): void {
  var chartX = 50;
  var chartY = 320;
  var chartW = W - 100;
  var chartH = 200;

  // Background
  ctx.fillStyle = '#131b2e';
  roundRect(ctx, chartX, chartY, chartW, chartH, 8);
  ctx.fill();

  if (daily.length === 0) {
    ctx.font = '400 13px Inter, system-ui, sans-serif';
    ctx.fillStyle = '#475569';
    ctx.textAlign = 'center';
    ctx.fillText('No activity data available', chartX + chartW / 2, chartY + chartH / 2);
    ctx.textAlign = 'left';
    return;
  }

  var max = Math.max.apply(null, daily);
  if (max === 0) max = 1;
  var barCount = daily.length;
  var barArea = chartW - 40;
  var barW = Math.max(4, Math.floor(barArea / barCount) - 2);
  var barStartX = chartX + 20;

  for (var i = 0; i < barCount; i++) {
    var barH = Math.max(2, (daily[i] / max) * (chartH - 50));
    var bx = barStartX + i * (barW + 2);
    var by = chartY + chartH - 20 - barH;

    // Gradient opacity
    var alpha = 0.4 + (daily[i] / max) * 0.6;

    ctx.fillStyle = 'rgba(110, 231, 183, ' + alpha + ')';
    roundRect(ctx, bx, by, barW, barH, 2);
    ctx.fill();
  }

  // Label
  ctx.font = '400 11px Inter, system-ui, sans-serif';
  ctx.fillStyle = '#64748b';
  ctx.fillText('Activity Trend (last ' + daily.length + ' days)', chartX + 20, chartY + chartH - 4);
}

function drawFooter(ctx: CanvasRenderingContext2D, data: ReportData, W: number, H: number): void {
  ctx.strokeStyle = 'rgba(255,255,255,0.06)';
  ctx.lineWidth = 1;
  ctx.beginPath();
  ctx.moveTo(40, H - 40);
  ctx.lineTo(W - 40, H - 40);
  ctx.stroke();

  ctx.font = '400 11px Inter, system-ui, sans-serif';
  ctx.fillStyle = '#475569';
  ctx.fillText('Tracked with Niyantra · niyantra.bhaskarjha.dev', 50, H - 18);

  ctx.textAlign = 'right';
  ctx.fillText(new Date().toLocaleDateString(), W - 50, H - 18);
  ctx.textAlign = 'left';
}

// Helper: draw rounded rectangle
function roundRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, r: number): void {
  ctx.beginPath();
  ctx.moveTo(x + r, y);
  ctx.lineTo(x + w - r, y);
  ctx.arcTo(x + w, y, x + w, y + r, r);
  ctx.lineTo(x + w, y + h - r);
  ctx.arcTo(x + w, y + h, x + w - r, y + h, r);
  ctx.lineTo(x + r, y + h);
  ctx.arcTo(x, y + h, x, y + h - r, r);
  ctx.lineTo(x, y + r);
  ctx.arcTo(x, y, x + r, y, r);
  ctx.closePath();
}

// Download the report as PNG
export function downloadReport(): void {
  var btn = document.getElementById('generate-report-btn');
  if (btn) {
    btn.textContent = '⏳ Generating...';
    btn.setAttribute('disabled', 'true');
  }

  generateReport().then(function(blob) {
    if (btn) {
      btn.textContent = '📊 Monthly Report';
      btn.removeAttribute('disabled');
    }
    if (!blob) return;
    var url = URL.createObjectURL(blob);
    var a = document.createElement('a');
    a.href = url;
    a.download = 'niyantra-report-' + new Date().toISOString().slice(0, 7) + '.png';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  }).catch(function(err) {
    console.error('Report generation failed:', err);
    if (btn) {
      btn.textContent = '📊 Monthly Report';
      btn.removeAttribute('disabled');
    }
  });
}
