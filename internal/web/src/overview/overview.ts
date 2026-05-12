// Niyantra Dashboard — Overview Tab Renderer
// @ts-nocheck
import { serverConfig, latestQuotaData } from '../core/state';
import { esc, formatTimeAgo, formatDurationSec } from '../core/utils';
import { fetchOverview, fetchSubscriptions, fetchUsage } from '../core/api';
import { getBudget, openBudgetModal } from './budget';
import { renderServerInsights, loadAdvisorCard } from './insights';
import { loadCostKPI } from './cost';
import { renderRenewalCalendar } from './calendar';
import { formatResetTime } from '../quotas/render';
import { renderClaudeCodeCard, loadClaudeCardData } from '../advanced/claude';
import { renderSessionsTimeline } from '../advanced/codex';

export function loadOverview(): void {
  // Fetch overview, subscriptions, and usage intelligence
  Promise.all([fetchOverview(), fetchSubscriptions('', ''), fetchUsage()]).then(function(results) {
    var data = results[0];
    var subsData = results[1];
    var usageData = results[2];
    renderOverviewEnhanced(data, subsData.subscriptions || [], usageData);
  }).catch(function(err) {
    console.error('Failed to load overview:', err);
  });
}

export function renderOverviewEnhanced(data, subs, usageData) {
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

  // ── Budget Status (from usage intelligence) — single source of truth ──
  var forecastHTML = '';
  if (usageData && usageData.budgetForecast) {
    var bf = usageData.budgetForecast;
    var forecastCls = bf.onTrack ? 'forecast-ok' : 'forecast-over';
    var forecastIcon = bf.onTrack ? '✅' : '⚠️';
    var pct = Math.round((bf.currentSpend / bf.monthlyBudget) * 100);
    var statusMsg = bf.onTrack
      ? 'On track — $' + bf.currentSpend.toFixed(2) + ' of $' + bf.monthlyBudget.toFixed(2) + ' budget (' + pct + '%)'
      : 'Over budget — $' + bf.currentSpend.toFixed(2) + ' exceeds $' + bf.monthlyBudget.toFixed(2) + ' by $' +
        (bf.currentSpend - bf.monthlyBudget).toFixed(2) + ' (' + pct + '%)';
    forecastHTML = '<div class="overview-card full-width">' +
      '<h3>Budget Status</h3>' +
      '<div class="budget-forecast ' + forecastCls + '">' +
      '<div class="forecast-header">' + forecastIcon + ' ' + statusMsg + '</div>' +
      '<div class="forecast-details">' +
      '<span class="forecast-chip">Monthly subs: $' + bf.currentSpend.toFixed(2) + '</span>' +
      '<span class="forecast-chip">Budget: $' + bf.monthlyBudget.toFixed(2) + '</span>' +
      '</div></div></div>';
  } else if (!getBudget()) {
    forecastHTML = '<div class="overview-card full-width">' +
      '<div class="budget-forecast forecast-ok">' +
      '<div class="forecast-header">💰 No monthly budget set</div>' +
      '<div class="forecast-details"><button class="btn-add-sm" onclick="openBudgetModal()">Set Budget</button></div>' +
      '</div></div>';
  }

  // ── Spend + Category merged card ──
  var cats = Object.keys(stats.byCategory);
  var spendHTML = '<div class="overview-card">' +
    '<h3>Monthly AI Spend</h3>' +
    '<div class="overview-big-number">$' + stats.totalMonthlySpend.toFixed(2) + '</div>';
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

  // ── Claude Code card ──
  var claudeHTML = '';
  if (serverConfig['claude_bridge'] === 'true') {
    claudeHTML = renderClaudeCodeCard();
  }

  // ── Renewal Calendar — only if renewals exist ──
  var calendarHTML = '';
  if (renewals.length > 0) {
    calendarHTML = '<div id="renewal-calendar-container" class="overview-card full-width"></div>';
  }

  // ── Quick Links — most recent URL per platform (no stale duplicates) ──
  var linksHTML = '';
  if (links.length > 0) {
    var platformLinks = {};
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
    var cs = latestQuotaData.codexSnapshot;
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
    var cls2 = latestQuotaData.claudeSnapshot;
    var clStatus = cls2.status === 'healthy' ? 'health-good' : 'health-bad';
    providerHTML += '<div class="provider-health-row">' +
      '<span class="ph-name">🔮 Claude Code</span>' +
      '<span class="ph-count">Bridge</span>' +
      '<span class="ph-bar"><span class="ph-fill ' + clStatus + '" style="width:' + (100 - (cls2.fiveHourPct || 0)) + '%"></span></span>' +
      '<span class="ph-stat ' + clStatus + '">' + (cls2.status || '—') + '</span>' +
      '</div>';
  }
  providerHTML += '</div></div>';

  // ── F8: Estimated Cost KPI (async — fetched from /api/cost) ──
  var costKPIHTML = '<div id="cost-kpi-container"></div>';

  // P1: Content order — advisor first (most actionable), then budget, cost KPI, provider health, spend, insights
  el.innerHTML = advisorHTML + forecastHTML + costKPIHTML + providerHTML + insightsHTML + claudeHTML + spendHTML + calendarHTML + linksHTML + exportHTML;

  // Async load Claude Code data
  if (serverConfig['claude_bridge'] === 'true') {
    loadClaudeCardData();
  }

  // Async load advisor card
  loadAdvisorCard();

  // F8: Async load estimated cost KPI
  loadCostKPI();

  // Render calendar with renewal data (only if container exists)
  if (renewals.length > 0) {
    renderRenewalCalendar(renewals, subs);
  }

  // Phase 11: Sessions timeline (Codex card removed — Provider Health covers it)
  renderSessionsTimeline(el);
}

