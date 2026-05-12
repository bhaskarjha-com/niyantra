// Niyantra Dashboard — Insights & Advisor
import { latestQuotaData } from '../core/state';
import { esc } from '../core/utils';

export function generateInsights(stats: any, renewals: any, subs: any): any[] {
  var chips = [];

  // Total subscriptions
  var activeSubs = (stats.byStatus && stats.byStatus.active) || 0;
  var trialSubs = (stats.byStatus && stats.byStatus.trial) || 0;
  if (activeSubs > 0) {
    chips.push({ icon: '📊', text: activeSubs + ' active subscription' + (activeSubs !== 1 ? 's' : ''), cls: 'info' });
  }
  if (trialSubs > 0) {
    chips.push({ icon: '⏳', text: trialSubs + ' trial' + (trialSubs !== 1 ? 's' : '') + ' active', cls: 'warn' });
  }

  // Highest category
  if (stats.byCategory) {
    var topCat = null, topSpend = 0;
    var cats = Object.keys(stats.byCategory);
    for (var i = 0; i < cats.length; i++) {
      if (stats.byCategory[cats[i]].monthlySpend > topSpend) {
        topSpend = stats.byCategory[cats[i]].monthlySpend;
        topCat = cats[i];
      }
    }
    if (topCat && topSpend > 0) {
      chips.push({ icon: '💰', text: 'Most spent on: ' + topCat + ' ($' + topSpend.toFixed(0) + '/mo)', cls: 'info' });
    }
  }

  // Imminent renewals
  var urgent = 0;
  for (var r = 0; r < renewals.length; r++) {
    if (renewals[r].daysUntil <= 3) urgent++;
  }
  if (urgent > 0) {
    chips.push({ icon: '🔴', text: urgent + ' renewal' + (urgent !== 1 ? 's' : '') + ' in next 3 days', cls: 'warn' });
  } else if (renewals.length > 0) {
    chips.push({ icon: '📅', text: 'Next renewal: ' + renewals[0].platform + ' in ' + renewals[0].daysUntil + ' days', cls: 'good' });
  }

  // PAYG warning
  if (subs) {
    var paygCount = 0;
    for (var s = 0; s < subs.length; s++) {
      if (subs[s].billingCycle === 'payg' && subs[s].status === 'active') paygCount++;
    }
    if (paygCount > 0) {
      chips.push({ icon: '📈', text: paygCount + ' pay-as-you-go service' + (paygCount !== 1 ? 's' : '') + ' (unbounded cost)', cls: 'warn' });
    }
  }

  // Annual savings potential
  if (stats.totalMonthlySpend > 100) {
    var annualSavings = stats.totalMonthlySpend * 12 * 0.17; // ~17% saved on annual billing
    chips.push({ icon: '💡', text: 'Could save ~$' + annualSavings.toFixed(0) + '/yr by switching monthly plans to annual (typical ~17% discount)', cls: 'good' });
  }

  return chips;
}

export function renderInsightChips(chips: any[]): string {
  if (chips.length === 0) return '';
  var html = '<div class="overview-card full-width"><h3>Insights</h3><div class="insight-chips">';
  for (var i = 0; i < chips.length; i++) {
    var c = chips[i];
    html += '<div class="insight-chip ' + c.cls + '">' +
      '<span class="insight-icon">' + c.icon + '</span>' + esc(c.text) + '</div>';
  }
  html += '</div></div>';
  return html;
}


//  Phase 10: SERVER-COMPUTED INSIGHTS
// ════════════════════════════════════════════

export function renderServerInsights(insights: any[]): string {
  if (!insights || insights.length === 0) return '';

  var html = '<div class="insight-panel"><h3>🧠 Intelligence Insights</h3><div class="insight-list">';

  var iconMap = {
    renewal_imminent: '🔴',
    trial_expiring: '⏳',
    unused_subscription: '💤',
    spending_anomaly: '📈',
    category_overlap: '🔁',
    annual_savings: '💡',
    budget_exceeded: '🚨'
  };

  for (var i = 0; i < insights.length; i++) {
    var ins = insights[i];
    var icon = (iconMap as any)[ins.type] || '💡';
    var cls = ins.severity === 'critical' ? 'critical' : (ins.severity === 'warning' ? 'warning' : 'info');
    html += '<div class="insight-item ' + cls + '">' +
      '<span class="insight-item-icon">' + icon + '</span>' +
      '<div class="insight-item-content">' +
      '<div class="insight-item-title">' + esc(ins.type.replace(/_/g, ' ')) + '</div>' +
      '<div class="insight-item-msg">' + esc(ins.message) + '</div>' +
      '</div></div>';
  }
  html += '</div></div>';
  return html;
}

// O1: Dynamic Switch Advisor — model-group aware
export var advisorGroupPref = localStorage.getItem('niyantra_advisor_group') || 'claude_gpt';

export function loadAdvisorCard(): void {
  var container = document.getElementById('advisor-card-container');
  if (!container) return;

  // Use latestQuotaData which has per-account group breakdowns
  if (!latestQuotaData || !latestQuotaData.accounts || latestQuotaData.accounts.length < 2) {
    container.innerHTML = '';
    return;
  }

  renderAdvisorWithGroup(container, advisorGroupPref);
}

export function renderAdvisorWithGroup(container: HTMLElement, groupKey: string): void {
  var accounts = latestQuotaData!.accounts;

  // Build ranked list based on selected group's remaining %
  var ranked = [];
  for (var i = 0; i < accounts.length; i++) {
    var acc = accounts[i];
    var groups = acc.groups || [];
    var pct = null;
    for (var g = 0; g < groups.length; g++) {
      if (groups[g].groupKey === groupKey) {
        pct = Math.round(groups[g].remainingPercent);
        break;
      }
    }
    // If group not found for this account, compute average of all
    if (pct === null) {
      if (groupKey === 'all') {
        var total = 0;
        for (var gx = 0; gx < groups.length; gx++) total += groups[gx].remainingPercent;
        pct = groups.length > 0 ? Math.round(total / groups.length) : 0;
      } else {
        pct = 0;
      }
    }
    var isStale = false;
    if (acc.lastSeen) {
      var ageMs = Date.now() - new Date(acc.lastSeen).getTime();
      isStale = ageMs > 6 * 3600 * 1000; // >6h
    }
    ranked.push({
      email: acc.email,
      pct: pct,
      stale: isStale,
      label: acc.stalenessLabel || ''
    });
  }

  // Sort by remaining % descending
  ranked.sort(function(a, b) { return b.pct - a.pct; });

  // Group display names
  var groupNames = {
    'claude_gpt': 'Claude + GPT',
    'gemini_pro': 'Gemini Pro',
    'gemini_flash': 'Gemini Flash',
    'all': 'All Models (avg)'
  };

  var best = ranked[0];
  var worst = ranked[ranked.length - 1];
  // Bug 4 fix: Detect when all accounts are healthy
  var allHealthy = ranked.every(function(a) { return a.pct > 80; });
  var actionIcon = allHealthy ? '✅' : (best.pct > 20 ? '⚡' : '⏳');
  var actionLabel = allHealthy ? 'ALL READY' : (best.pct > 20 ? 'SWITCH' : 'WAIT');
  var bestLabel = best.email.split('@')[0] + '@...';

  var html = '<div class="advisor-card">' +
    '<h3>⚡ Antigravity Account Advisor</h3>' +
    '<div class="advisor-group-select">' +
    '<label>Optimize for:</label>' +
    '<select id="advisor-group-filter" class="filter-select" style="margin-left:8px;font-size:12px">' +
    '<option value="claude_gpt"' + (groupKey === 'claude_gpt' ? ' selected' : '') + '>Claude + GPT</option>' +
    '<option value="gemini_pro"' + (groupKey === 'gemini_pro' ? ' selected' : '') + '>Gemini Pro</option>' +
    '<option value="gemini_flash"' + (groupKey === 'gemini_flash' ? ' selected' : '') + '>Gemini Flash</option>' +
    '<option value="all"' + (groupKey === 'all' ? ' selected' : '') + '>All Models (avg)</option>' +
    '</select></div>';

  var actionCls = allHealthy ? 'stay' : (best.pct > 20 ? 'switch' : 'wait');
  html += '<div class="advisor-action ' + actionCls + '">' +
    actionIcon + ' ' + actionLabel + '</div>' +
    '<div class="advisor-reason">' + (allHealthy ? 'All accounts have healthy quotas — no switch needed' :
    'Best: ' + esc(best.email) + ' (' + best.pct + '% ' +
    esc((groupNames as any)[groupKey] || groupKey) + ' remaining)') +
    (best.stale ? ' ⚠️ stale data' : '') + '</div>';

  // Score bars — show top 5, toggle for rest
  html += '<div class="advisor-scores">';
  var initialShow = Math.min(ranked.length, 5);
  for (var s = 0; s < ranked.length; s++) {
    var acct = ranked[s];
    var isBest = s === 0;
    var barCls = acct.pct > 50 ? 'good' : (acct.pct > 20 ? 'ok' : 'low');
    var staleIcon = acct.stale ? ' <span class="stale-icon" title="Data ' + esc(acct.label) + '">⚠</span>' : '';
    var hidden = s >= initialShow ? ' style="display:none" data-advisor-extra' : '';
    html += '<div class="advisor-score-row' + (isBest ? ' best' : '') + '"' + hidden + '>' +
      '<span class="advisor-score-email" title="' + esc(acct.email) + '">' + esc(acct.email) + '</span>' +
      '<div class="advisor-score-bar"><div class="advisor-score-fill ' + barCls + '" style="width:' + acct.pct + '%"></div></div>' +
      '<span class="advisor-score-val">' + acct.pct + '%' + staleIcon + '</span>' +
      '</div>';
  }
  if (ranked.length > initialShow) {
    html += '<button class="advisor-show-all" id="advisor-toggle-all">Show all ' + ranked.length + ' accounts</button>';
  }
  html += '</div></div>';
  container.innerHTML = html;

  // Wire up group selector
  var sel = document.getElementById('advisor-group-filter');
  if (sel) {
    sel.addEventListener('change', function() {
      advisorGroupPref = (sel as HTMLSelectElement).value;
      localStorage.setItem('niyantra_advisor_group', advisorGroupPref);
      renderAdvisorWithGroup(container, advisorGroupPref);
    });
  }

  // Wire up show-all toggle
  var toggleBtn = document.getElementById('advisor-toggle-all');
  if (toggleBtn) {
    toggleBtn.addEventListener('click', function() {
      var extras = container.querySelectorAll('[data-advisor-extra]');
      var showing = toggleBtn!.textContent.indexOf('Hide') >= 0;
      extras.forEach(function(el) { (el as HTMLElement).style.display = showing ? 'none' : ''; });
      toggleBtn!.textContent = showing ? 'Show all ' + ranked.length + ' accounts' : 'Hide extras';
    });
  }
}


