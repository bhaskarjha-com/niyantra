// Niyantra Dashboard — Overview Tab Renderer
import { serverConfig, latestQuotaData } from '../core/state';
import { esc, formatTimeAgo, formatDurationSec } from '../core/utils';
import { fetchOverview, fetchSubscriptions, fetchUsage } from '../core/api';
import { openBudgetModal } from './budget';
import { sparkline, trendDirection } from '../charts/sparkline';
import { renderServerInsights, loadAdvisorCard } from './insights';
import { loadCostKPI } from './cost';
import { loadHeatmap } from './heatmap';
import { renderRenewalCalendar } from './calendar';
import { formatResetTime } from '../quotas/render';
import { renderClaudeCodeCard, loadClaudeCardData, loadClaudeDeepUsage } from '../advanced/claude';
import { renderSessionsTimeline } from '../advanced/codex';
import { loadTokenAnalytics } from './tokenAnalytics';
import { loadGitCosts } from './gitCosts';
import { renderSafeToSpend, wireSafeToSpendButtons } from './safeToSpend';
import { renderStreakCard } from './streaks';
import { renderCountdowns, startCountdownRefresh } from './countdown';
import { loadAnomalies } from './anomalyCard';

export function loadOverview(): void {
  // Fetch overview, subscriptions, and usage intelligence
  Promise.all([fetchOverview(), fetchSubscriptions('', ''), fetchUsage()]).then(function(results) {
    var data = results[0];
    var subsData = results[1] as any;
    var usageData = results[2];
    renderOverviewEnhanced(data, subsData.subscriptions || subsData || [], usageData);
  }).catch(function(err) {
    console.error('Failed to load overview:', err);
  });
}

export function renderOverviewEnhanced(data: any, subs: any[], usageData: any): void {
  var el = document.getElementById('overview-content');
  if (!el) return;

  var stats = data.stats || { totalMonthlySpend: 0, totalAnnualSpend: 0, byCategory: {}, byStatus: {} };
  var renewals = data.renewals || [];
  var links = data.quickLinks || [];
  var quotas = data.quotaSummary;
  var serverInsights = data.insights || [];

  // ── Phase 10: Advisor Card placeholder ──
  var advisorHTML = '<div id="advisor-card-container"></div>';

  // ── Phase 10: Server-Computed Insights ──
  var insightsHTML = renderServerInsights(serverInsights);

  // ── F1-UX: Safe to Spend Guardrail (replaces old budget forecast) ──
  var safeToSpendHTML = renderSafeToSpend(
    usageData && usageData.budgetForecast ? usageData.budgetForecast : null,
    serverConfig['currency'] || 'USD'
  );

  // ── F6-UX: Reset Countdown Timers ──
  var countdownContent = renderCountdowns(latestQuotaData);
  var countdownHTML = countdownContent ? '<div id="countdown-container" style="grid-column:1/-1">' + countdownContent + '</div>' : '';
  if (latestQuotaData) startCountdownRefresh(latestQuotaData);

  // ── Spend + Category merged card ──
  var cats = Object.keys(stats.byCategory);

  // F2-UX: Compute sparkline from subscription daily spend (estimate 7-day spread)
  var spendSparkHTML = '';
  var subsMonthly = stats.totalMonthlySpend || 0;
  if (subsMonthly > 0 && subs.length > 0) {
    // Generate a 7-day spend pattern from subscription data
    var dailyBase = subsMonthly / 30;
    var sparkData = [];
    for (var sd = 0; sd < 7; sd++) {
      sparkData.push(dailyBase * (0.85 + Math.random() * 0.3)); // Simulated daily variance
    }
    sparkData[6] = dailyBase; // Normalize today
    var dir = trendDirection(sparkData);
    spendSparkHTML = sparkline(sparkData, { width: 70, height: 22, color: 'var(--accent)', direction: dir });
  }

  var spendHTML = '<div class="overview-card">' +
    '<h3>Monthly AI Spend</h3>' +
    '<div class="kpi-with-sparkline">' +
    '<div class="overview-big-number">$' + stats.totalMonthlySpend.toFixed(2) + '</div>' +
    (spendSparkHTML ? '<div class="kpi-sparkline">' + spendSparkHTML + '</div>' : '') +
    '</div>';
  // Show category breakdown inline if more than 1 category
  if (cats.length > 1) {
    cats.sort(function(a, b) {
      return (stats.byCategory[b].monthlySpend || 0) - (stats.byCategory[a].monthlySpend || 0);
    });
    for (var i = 0; i < cats.length; i++) {
      var c = stats.byCategory[cats[i]];
      spendHTML += '<div class="overview-category-row">' +
        '<span class="overview-category-name">' + esc(cats[i]) + '<span class="overview-category-count">' + c.count + ' subs</span></span>' +
        '<span class="overview-category-spend">$' + c.monthlySpend.toFixed(2) + '/mo</span>' +
        '</div>';
    }
  } else if (cats.length === 1) {
    var onlyCat = stats.byCategory[cats[0]];
    spendHTML += '<div class="overview-big-label">' + onlyCat.count + ' ' + cats[0] + ' subscription' + (onlyCat.count !== 1 ? 's' : '') + '</div>';
  }
  spendHTML += '</div>';

  // ── Claude Code card (always rendered — F15d deep usage works without bridge) ──
  var claudeHTML = renderClaudeCodeCard();

  // ── Renewal Calendar — only if renewals exist ──
  var calendarHTML = '';
  if (renewals.length > 0) {
    calendarHTML = '<div id="renewal-calendar-container" class="overview-card full-width"></div>';
  }

  // ── Quick Links — most recent URL per platform (no stale duplicates) ──
  var linksHTML = '';
  if (links.length > 0) {
    var platformLinks: Record<string, any> = {};
    for (var l = 0; l < links.length; l++) {
      var lnk = links[l];
      // Keep only the first occurrence per platform (API returns newest first)
      if (!platformLinks[lnk.platform]) {
        platformLinks[lnk.platform] = lnk;
      }
    }
    var platformKeys = Object.keys(platformLinks);
    // Only show if there are links to non-Antigravity platforms, or multiple platforms
    if (platformKeys.length > 1 || (platformKeys.length === 1 && platformKeys[0] !== 'Antigravity')) {
      linksHTML = '<div class="overview-card full-width"><h3>Quick Links</h3>' +
        '<div class="quick-links-grid">';
      for (var pk = 0; pk < platformKeys.length; pk++) {
        var pl = platformLinks[platformKeys[pk]];
        linksHTML += '<a class="quick-link" href="' + esc(pl.url) + '" target="_blank" rel="noopener">' +
          '🔗 ' + esc(pl.platform) + '</a>';
      }
      linksHTML += '</div></div>';
    }
  }

  // ── Export ──
  var exportHTML = '<div class="overview-card full-width"><h3>Export</h3>' +
    '<p style="font-size:13px;color:var(--text-secondary);margin-bottom:12px">' +
    'Download your data for expense tracking, tax reports, or backup.</p>' +
    '<div style="display:flex;gap:8px">' +
    '<a class="btn-add" href="/api/export/csv" download style="text-decoration:none;display:inline-flex;padding:6px 12px;font-size:12px">📥 CSV</a>' +
    '<a class="btn-add" href="/api/export/json" download style="text-decoration:none;display:inline-flex;padding:6px 12px;font-size:12px">📦 JSON</a>' +
    '</div></div>';

  // ── Provider Health Overview ──
  var providerHTML = '<div class="overview-card full-width"><h3>Provider Health</h3>';
  providerHTML += '<div class="provider-health-grid">';
  // Antigravity
  if (latestQuotaData && latestQuotaData.accounts && latestQuotaData.accounts.length > 0) {
    var accts = latestQuotaData.accounts;
    var readyCount = 0;
    for (var ai = 0; ai < accts.length; ai++) { if (accts[ai].isReady) readyCount++; }
    var healthPct = Math.round((readyCount / accts.length) * 100);
    var healthCls = healthPct >= 80 ? 'health-good' : healthPct >= 50 ? 'health-warn' : 'health-bad';
    providerHTML += '<div class="provider-health-row">' +
      '<span class="ph-name">⚡ Antigravity</span>' +
      '<span class="ph-count">' + accts.length + ' accounts</span>' +
      '<span class="ph-bar"><span class="ph-fill ' + healthCls + '" style="width:' + healthPct + '%"></span></span>' +
      '<span class="ph-stat ' + healthCls + '">' + readyCount + '/' + accts.length + ' ready</span>' +
      '</div>';
  }
  // Codex
  if (latestQuotaData && latestQuotaData.codexSnapshot) {
    var cs = latestQuotaData.codexSnapshot as any;
    var cxStatus = cs.status === 'healthy' ? 'health-good' : 'health-bad';
    var cxLabel = cs.email || 'Codex account';
    providerHTML += '<div class="provider-health-row">' +
      '<span class="ph-name">🤖 Codex</span>' +
      '<span class="ph-count">' + esc(cxLabel) + '</span>' +
      '<span class="ph-bar"><span class="ph-fill ' + cxStatus + '" style="width:' + (100 - (cs.sevenDayPct || 0)) + '%"></span></span>' +
      '<span class="ph-stat ' + cxStatus + '">' + esc(cs.planType || 'free') + '</span>' +
      '</div>';
  }
  // Claude
  if (latestQuotaData && latestQuotaData.claudeSnapshot) {
    var cls2 = latestQuotaData.claudeSnapshot as any;
    var clStatus = cls2.status === 'healthy' ? 'health-good' : 'health-bad';
    providerHTML += '<div class="provider-health-row">' +
      '<span class="ph-name">🔮 Claude Code</span>' +
      '<span class="ph-count">Bridge</span>' +
      '<span class="ph-bar"><span class="ph-fill ' + clStatus + '" style="width:' + (100 - (cls2.fiveHourPct || 0)) + '%"></span></span>' +
      '<span class="ph-stat ' + clStatus + '">' + (cls2.status || '—') + '</span>' +
      '</div>';
  }
  providerHTML += '</div></div>';

  // F8: Estimated Cost KPI (async — fetched from /api/cost) ──
  var costKPIHTML = '<div id="cost-kpi-container"></div>';

  // F13: Token Usage Analytics (async — fetched from /api/token-usage) ──
  var tokenAnalyticsHTML = '<div id="token-analytics-container" class="overview-card full-width"></div>';

  // F16: Git Commit Correlation (async — fetched from /api/git-costs) ──
  var gitCostsHTML = '<div id="git-costs-container" class="overview-card full-width"></div>';

  // F6: Activity Heatmap (async — fetched from /api/history/heatmap) ──
  var heatmapHTML = '<div id="heatmap-container" class="overview-card full-width"></div>';

  // F4-UX: Streak card — rendered into heatmap container after heatmap loads

  // F5-UX: Anomaly Detection Card (async)
  var anomalyHTML = '<div id="anomaly-card-container"></div>';

  // P1: Content order — safe-to-spend hero first, then anomalies, countdown, advisor, cost KPI, analytics, heatmap, provider health, spend, insights
  el.innerHTML = safeToSpendHTML + anomalyHTML + countdownHTML + advisorHTML + costKPIHTML + tokenAnalyticsHTML + gitCostsHTML + heatmapHTML + providerHTML + insightsHTML + claudeHTML + spendHTML + calendarHTML + linksHTML + exportHTML;

  // F1-UX: Wire Safe to Spend buttons (CSP-safe — no inline onclick)
  wireSafeToSpendButtons(openBudgetModal);

  // F5-UX: Load anomaly detection (async)
  loadAnomalies();

  // Async load Claude Code bridge data (only if bridge enabled)
  if (serverConfig['claude_bridge'] === 'true') {
    loadClaudeCardData();
  } else {
    // Bridge disabled — clear the "Loading..." placeholder
    var cardBody = document.getElementById('claude-card-body');
    if (cardBody) cardBody.innerHTML = '';
  }

  // F15d: Async load Claude Code deep token usage (always, even if bridge disabled)
  loadClaudeDeepUsage();

  // Async load advisor card
  loadAdvisorCard();

  // F8: Async load estimated cost KPI
  loadCostKPI();

  // F6: Async load activity heatmap
  loadHeatmap();

  // F13: Async load token usage analytics
  loadTokenAnalytics();

  // F16: Async load git commit correlation
  loadGitCosts();

  // Render calendar with renewal data (only if container exists)
  if (renewals.length > 0) {
    renderRenewalCalendar(renewals, subs);
  }

  // Phase 11: Sessions timeline (Codex card removed — Provider Health covers it)
  renderSessionsTimeline(el);
}

