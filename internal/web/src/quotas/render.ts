// Niyantra Dashboard — Quota Grid Rendering
// Sort, filter, tag strip, and account grid rendering.

import {
  GROUP_ORDER, GROUP_LABELS, GROUP_COLORS, GROUP_NAMES,
  expandedAccounts, collapsedProviders,
  quotaSortState, latestQuotaData, setLatestQuotaData,
  activeTagFilter, setActiveTagFilter,
  usageDataCache,
} from '../core/state';
import { esc, formatSeconds, formatCredits, formatTimeAgo } from '../core/utils';
import type { StatusResponse, AccountReadiness } from '../types/api';
import { renderPinnedBadge, renderAccountTags, renderAccountNote, renderCreditRenewal } from './features';
export function getGroupPct(acc: any, groupKey: string): number {
  if (!acc.groups) return -1;
  for (var i = 0; i < acc.groups.length; i++) {
    if (acc.groups[i].groupKey === groupKey) return acc.groups[i].remainingPercent;
  }
  return -1;
}

export function getAICredits(acc: any): number {
  if (acc.aiCredits && acc.aiCredits.length > 0) return acc.aiCredits[0].creditAmount;
  return -1;
}

export function allExhausted(acc: any): boolean {
  var grps = acc.groups || [];
  if (grps.length === 0) return false;
  for (var i = 0; i < grps.length; i++) {
    if (!grps[i].isExhausted && grps[i].remainingPercent > 0) return false;
  }
  return true;
}

// Bug 6: Determine status of Codex/Claude snapshot for filtering
export function getCodexClaudeStatus(snap: any): string {
  var fiveUsed = snap.fiveHourPct || 0;
  var sevenUsed = snap.sevenDayPct || 0;
  var fiveRem = Math.max(0, 100 - fiveUsed);
  var sevenRem = Math.max(0, 100 - sevenUsed);
  if (fiveRem === 0 && sevenRem === 0) return 'empty';
  if (fiveUsed >= 80 || sevenUsed >= 80) return 'low';
  return 'ready';
}

export function sortAccountsArray(accounts: any[]): any[] {
  var col = quotaSortState.column;
  var dir = quotaSortState.direction;
  return accounts.slice().sort(function(a, b) {
    var va, vb;
    switch (col) {
      case 'account': va = a.email; vb = b.email; break;
      case 'claude_gpt':
      case 'gemini_pro':
      case 'gemini_flash':
        va = getGroupPct(a, col); vb = getGroupPct(b, col); break;
      case 'credits':
        va = getAICredits(a); vb = getAICredits(b); break;
      case 'lastsnap':
        va = a.lastSeen ? new Date(a.lastSeen).getTime() : 0;
        vb = b.lastSeen ? new Date(b.lastSeen).getTime() : 0; break;
      case 'status':
        va = a.isReady ? 1 : 0; vb = b.isReady ? 1 : 0; break;
      default: va = a.email; vb = b.email; break;
    }
    if (va === vb) return 0;
    var res = va > vb ? 1 : -1;
    return dir === 'asc' ? res : -res;
  });
}

export function filterAccountsArray(accounts: any[]): any[] {
  var searchInput = document.getElementById('quota-search');
  var statusFilter = document.getElementById('quota-filter-status');
  var query = searchInput ? (searchInput as HTMLInputElement).value.toLowerCase() : '';
  var status = statusFilter ? (statusFilter as HTMLSelectElement).value : 'all';

  return accounts.filter(function(acc) {
    var matchesSearch = !query ||
      acc.email.toLowerCase().includes(query) ||
      (acc.planName || '').toLowerCase().includes(query);

    var matchesStatus = true;
    if (status === 'ready') matchesStatus = acc.isReady;
    else if (status === 'low') matchesStatus = !acc.isReady && !allExhausted(acc);
    else if (status === 'empty') matchesStatus = allExhausted(acc);

    // F4: Tag-based filtering
    var matchesTag = true;
    if (activeTagFilter) {
      var accTags = (acc.tags || '').split(',').map(function(t: any) { return t.trim().toLowerCase(); });
      matchesTag = accTags.indexOf(activeTagFilter) >= 0;
    }

    return matchesSearch && matchesStatus && matchesTag;
  });
}

export function updateSortHeaders(): void {
  document.querySelectorAll('.grid-header .sortable').forEach(function(el) {
    el.classList.remove('sort-active');
    var span = el.querySelector('.sort-indicator');
    if (span) span.textContent = '';
    if ((el as HTMLElement).dataset.sort === quotaSortState.column) {
      el.classList.add('sort-active');
      if (span) span.textContent = quotaSortState.direction === 'asc' ? '▾' : '▴';
    }
  });
}

// ════════════════════════════════════════════
//  F4: TAG-BASED FILTERING
// ════════════════════════════════════════════

export function getUniqueTagsFromData(data: any): Record<string, number> {
  var tagCounts: Record<string, number> = {};
  var accounts = data.accounts || [];
  for (var i = 0; i < accounts.length; i++) {
    var tags = (accounts[i].tags || '').split(',');
    for (var j = 0; j < tags.length; j++) {
      var t = tags[j].trim().toLowerCase();
      if (t) {
        tagCounts[t] = (tagCounts[t] || 0) + 1;
      }
    }
  }
  return tagCounts;
}

export function renderTagFilterStrip(data: any): void {
  var strip = document.getElementById('tag-filter-strip');
  if (!strip) return;

  var tagCounts = getUniqueTagsFromData(data);
  var tagNames = Object.keys(tagCounts).sort();

  // Only show strip if there are tags to filter by
  if (tagNames.length === 0) {
    // Bug fix: if active filter was set to a now-deleted tag, reset to show all
    if (activeTagFilter) {
      setActiveTagFilter(null);
    }
    strip.innerHTML = '';
    return;
  }

  // Bug fix: if the active tag no longer exists in the data, reset to show all
  if (activeTagFilter && tagNames.indexOf(activeTagFilter) < 0) {
    setActiveTagFilter(null);
  }

  var html = '<span class="tag-filter-label">🏷️ Filter:</span>';

  // "All" chip
  var allActive = !activeTagFilter ? ' active' : '';
  var totalAccounts = (data.accounts || []).length;
  html += '<button class="tag-filter-chip' + allActive + '" data-tag-filter="">' +
    'All <span class="tag-filter-count">' + totalAccounts + '</span></button>';

  // Tag chips
  for (var i = 0; i < tagNames.length; i++) {
    var tag = tagNames[i];
    var isActive = activeTagFilter === tag ? ' active' : '';
    html += '<button class="tag-filter-chip' + isActive + '" data-tag-filter="' + esc(tag) + '">' +
      esc(tag) + ' <span class="tag-filter-count">' + tagCounts[tag] + '</span></button>';
  }

  strip.innerHTML = html;
}

export function handleTagFilterClick(e: Event): void {
  var chip = (e.target as HTMLElement).closest('.tag-filter-chip');
  if (!chip) return;

  var tag = chip.getAttribute('data-tag-filter');
  setActiveTagFilter(tag || null);

  // Re-render with filter applied
  if (latestQuotaData) {
    renderTagFilterStrip(latestQuotaData);
    renderAccounts(latestQuotaData);
  }
}

export function renderAccounts(data: any): void {
  setLatestQuotaData(data);
  var grid = document.getElementById('account-grid');
  var countBadge = document.getElementById('account-count');
  var snapCount = document.getElementById('snap-count');
  if (!grid) return;

  // F4: Update tag filter strip on data refresh
  renderTagFilterStrip(data);

  var acctCount = (data.accounts || []).length;
  var parts = [];
  if (acctCount > 0) parts.push(acctCount + ' Antigravity');
  if (data.codexSnapshot) parts.push('1 Codex');
  if (data.claudeSnapshot) parts.push('1 Claude');
  if (countBadge) countBadge.textContent = parts.join(' · ') || '0 accounts';
  if (snapCount) snapCount.textContent = data.snapshotCount ? (data.snapshotCount + ' snapshots') : '';

  if (acctCount === 0 && !data.codexSnapshot && !data.claudeSnapshot) {
    grid.innerHTML = '<div class="empty-state">' +
      '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.4"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/><path d="M12 2v4M12 18v4M2 12h4M18 12h4"/></svg>' +
      '<p>No accounts tracked yet</p>' +
      '<p class="empty-hint">Click <strong>Snap Now</strong> to capture your first snapshot</p>' +
      '</div>';
    return;
  }

  var providerFilter = document.getElementById('quota-filter-provider');
  var pf = providerFilter ? (providerFilter as HTMLSelectElement).value : 'all';
  var html = '';
  if (acctCount > 0 && (pf === 'all' || pf === 'antigravity')) {
  var filtered = filterAccountsArray(data.accounts);
  var sorted = sortAccountsArray(filtered);
  var agCollapseClass = collapsedProviders.has('section-antigravity') ? ' collapsed' : '';
  var agChevron = collapsedProviders.has('section-antigravity') ? '▸' : '▾';
  html += '<div class="provider-section" data-provider="antigravity"><div class="provider-header" data-toggle-provider="section-antigravity">' +
    '<div class="provider-header-left"><span class="provider-chevron" id="pchev-section-antigravity">' + agChevron + '</span>' +
    '<span class="provider-name">Antigravity</span>' +
    '<span class="provider-count">' + acctCount + ' account' + (acctCount !== 1 ? 's' : '') + '</span></div></div>' +
    '<div class="provider-body' + agCollapseClass + '" id="section-antigravity">';
  // Dynamic Antigravity grid header
  html += '<div class="grid-header">' +
    '<div class="grid-col-account sortable" data-sort="account">Account <span class="sort-indicator"></span></div>';
  for (var gh = 0; gh < GROUP_ORDER.length; gh++) {
    html += '<div class="grid-col-group sortable" data-sort="' + GROUP_ORDER[gh] + '">' + (GROUP_LABELS[gh] || GROUP_ORDER[gh]) + ' <span class="sort-indicator"></span></div>';
  }
  html += '<div class="grid-col-credits sortable" data-sort="credits">AI Credits <span class="sort-indicator"></span></div>' +
    '<div class="grid-col-snap sortable" data-sort="lastsnap">Last Snap <span class="sort-indicator"></span></div>' +
    '<div class="grid-col-status sortable" data-sort="status">Status <span class="sort-indicator"></span></div></div>';
  for (var i = 0; i < sorted.length; i++) {
    var acc = sorted[i];
    var accId = 'acc-' + acc.accountId;
    var isExpanded = expandedAccounts.has(accId as any);

    var groupCells = '';
    // Pre-index models by group for Quick Adjust
    var modelsByGroup: Record<string, string[]> = {};
    if (acc.models) {
      for (var mi2 = 0; mi2 < acc.models.length; mi2++) {
        var mm = acc.models[mi2];
        var gk = mm.groupKey || 'claude_gpt';
        if (!modelsByGroup[gk]) modelsByGroup[gk] = [];
        modelsByGroup[gk].push(mm.label || mm.modelId);
      }
    }

    // F3: Determine pinned group for this account (default to first group)
    var pinnedKey = acc.pinnedGroup || (acc.groups && acc.groups.length > 0 ? acc.groups[0].groupKey : 'claude_gpt');
    var pinnedGroupData = null;
    var groups = acc.groups || [];
    for (var pg = 0; pg < groups.length; pg++) {
      if (groups[pg].groupKey === pinnedKey) { pinnedGroupData = groups[pg]; break; }
    }

    for (var gi = 0; gi < GROUP_ORDER.length; gi++) {
      var key = GROUP_ORDER[gi];
      var g = null;
      var groups = acc.groups || [];
      for (var gj = 0; gj < groups.length; gj++) {
        if (groups[gj].groupKey === key) { g = groups[gj]; break; }
      }
      if (!g) {
        groupCells += '<div class="quota-cell"><span class="quota-pct">—</span></div>';
        continue;
      }
      var pct = Math.round(g.remainingPercent);
      var cls = 'good';
      if (g.isExhausted || pct === 0) cls = 'exhausted';
      else if (pct < 20) cls = 'warning';
      else if (pct < 50) cls = 'ok';
      var reset = '';
      if (g.timeUntilResetSec > 0) {
        reset = '<span class="quota-reset">↻ ' + formatSeconds(g.timeUntilResetSec) + '</span>';
      }
      // Q4: Mini progress bar under percentage
      var barCls = cls;

      // Group-level Quick Adjust — ±5 buttons, appear on hover
      var groupLabels = (modelsByGroup[key] || []).join('|||');
      var groupAdjust = '<span class="group-adjust" data-snap-id="' + acc.latestSnapshotId +
        '" data-group-key="' + key +
        '" data-group-labels="' + esc(groupLabels) +
        '" data-current-pct="' + pct + '">' +
        '<button class="gadj-btn" data-delta="-5" title="−5% all models in group">−5</button>' +
        '<button class="gadj-btn" data-delta="5" title="+5% all models in group">+5</button>' +
        '</span>';

      // F7: TTX badge — shows "~Xh" time-to-exhaustion from forecast data
      var ttxBadge = '';
      if (data.forecasts && data.forecasts[acc.accountId]) {
        var acctForecasts = data.forecasts[acc.accountId];
        for (var fi = 0; fi < acctForecasts.length; fi++) {
          if (acctForecasts[fi].groupKey === key && acctForecasts[fi].ttxLabel) {
            var ttxSev = acctForecasts[fi].severity || 'safe';
            var ttxLabel = acctForecasts[fi].ttxLabel;
            if (ttxLabel && ttxLabel !== '' && ttxSev !== 'none') {
              ttxBadge = '<span class="ttx-badge ttx-' + ttxSev + '" title="Time to exhaustion at current burn rate">' + esc(ttxLabel) + '</span>';
            }
            break;
          }
        }
      }

      // F8: Cost badge — shows estimated $ cost per group
      // Only shown when: (a) group has consumed quota (pct < 95), (b) cost > $0.01
      var costBadge = '';
      if (pct < 95 && data.estimatedCosts && data.estimatedCosts[acc.accountId]) {
        var acctCosts = data.estimatedCosts[acc.accountId];
        if (acctCosts.groups) {
          for (var ci = 0; ci < acctCosts.groups.length; ci++) {
            if (acctCosts.groups[ci].groupKey === key && acctCosts.groups[ci].hasData) {
              var costVal = acctCosts.groups[ci].estimatedCost || 0;
              if (costVal < 0.01) break; // Skip negligible costs
              var costLabel = acctCosts.groups[ci].costLabel || '—';
              var costCls = 'cost-low';
              if (costVal >= 10) costCls = 'cost-high';
              else if (costVal >= 3) costCls = 'cost-medium';
              var costTitle = 'Estimated cost this cycle';
              if (acctCosts.groups[ci].hourlyLabel) {
                costTitle += ' (' + acctCosts.groups[ci].hourlyLabel + ')';
              }
              costBadge = '<span class="cost-badge ' + costCls + '" title="' + costTitle + '">' + esc(costLabel) + '</span>';
              break;
            }
          }
        }
      }

      groupCells += '<div class="quota-cell">' +
        '<span class="quota-pct ' + cls + '">' + pct + '%</span>' +
        '<div class="quota-minibar"><div class="quota-minibar-fill ' + barCls + '" style="width:' + pct + '%"></div></div>' +
        groupAdjust +
        reset + ttxBadge + costBadge + '</div>';
    }


    // Q5: Health dots — visual status
    var dotCls = 'dot-ready';
    var badgeText = 'Ready';
    if (allExhausted(acc)) { dotCls = 'dot-empty'; badgeText = 'Empty'; }
    else if (!acc.isReady) { dotCls = 'dot-low'; badgeText = 'Low'; }

    var creditsCell = '<div class="credits-cell" style="position:relative">';
    if (acc.aiCredits && acc.aiCredits.length > 0) {
      var credits = acc.aiCredits[0].creditAmount;
      var creditCls = credits > 500 ? 'good' : credits > 100 ? 'ok' : 'warning';
      creditsCell += '<span class="credit-amount ' + creditCls + '" title="AI Credits">✦ ' +
        formatCredits(credits) + '</span>';
      // Credit renewal countdown
      creditsCell += renderCreditRenewal(acc.accountId, acc.creditRenewalDay);
    } else {
      creditsCell += '<span class="credit-amount muted">—</span>';
    }
    creditsCell += '</div>';

    var modelsHTML = '';
    if (acc.models && acc.models.length > 0) {
      // F3: Build group headers with pin stars in expanded view
      var groupedModels: Record<string, any[]> = {};
      for (var mi = 0; mi < acc.models.length; mi++) {
        var m = acc.models[mi];
        var gk2 = m.groupKey || 'claude_gpt';
        if (!groupedModels[gk2]) groupedModels[gk2] = [];
        groupedModels[gk2].push(m);
      }

      var modelRows = '';
      for (var goi = 0; goi < GROUP_ORDER.length; goi++) {
        var groupKey2 = GROUP_ORDER[goi];
        var groupModels = groupedModels[groupKey2];
        if (!groupModels || groupModels.length === 0) continue;

        // F3: Group header with pin star
        var isPinned = pinnedKey === groupKey2;
        var starCls = isPinned ? 'pin-star pinned' : 'pin-star';
        var starTitle = isPinned ? 'Pinned — click to unpin' : 'Click to pin this group';
        var starChar = isPinned ? '★' : '☆';
        modelRows += '<div class="model-group-header">' +
          '<button class="' + starCls + '" data-pin-group="' + groupKey2 + '" data-pin-account="' + acc.accountId + '" title="' + starTitle + '">' + starChar + '</button>' +
          '<span class="model-group-name" style="color:' + (GROUP_COLORS[groupKey2] || 'var(--text-secondary)') + '">' + (GROUP_NAMES[groupKey2] || groupKey2) + '</span>' +
          '</div>';

        for (var mi3 = 0; mi3 < groupModels.length; mi3++) {
          var m = groupModels[mi3];
        var mpct = Math.round(m.remainingPercent);
        var mcls = 'good';
        if (m.isExhausted || mpct === 0) mcls = 'exhausted';
        else if (mpct < 20) mcls = 'warning';
        else if (mpct < 50) mcls = 'ok';
        var color = GROUP_COLORS[m.groupKey] || '#94a3b8';
        var resetStr = m.resetSeconds > 0 ? ('↻ ' + formatSeconds(m.resetSeconds)) : '';

        // Intelligence badges from usage data
        var intellBadges = '';
        if (usageDataCache && usageDataCache.models) {
          for (var ui = 0; ui < usageDataCache.models.length; ui++) {
            var um = usageDataCache.models[ui];
            if (um.modelId === m.modelId && um.hasIntelligence) {
              var rateStr = (um.currentRate * 100).toFixed(1) + '%/hr';
              intellBadges += '<span class="rate-badge" title="Current consumption rate">' + rateStr + '</span>';
              if (um.projectedUsage > 0) {
                var projPct = Math.round(um.projectedUsage * 100);
                var projCls = projPct > 95 ? 'proj-danger' : (projPct > 80 ? 'proj-warn' : 'proj-ok');
                intellBadges += '<span class="proj-badge ' + projCls + '" title="Projected usage at reset">→' + projPct + '%</span>';
              }
              if (um.projectedExhaustion) {
                var exhaust = new Date(um.projectedExhaustion);
                var minsLeft = Math.round((exhaust.getTime() - Date.now()) / 60000);
                if (minsLeft > 0) {
                  intellBadges += '<span class="exhaust-badge" title="Projected exhaustion time">⚠ ' + (minsLeft > 60 ? Math.round(minsLeft/60) + 'h' : minsLeft + 'm') + '</span>';
                }
              }
              break;
            }
          }
        }

        // Quick Adjust controls — visible on hover
        var adjustBtns = '<span class="adjust-controls" data-snap-id="' + acc.latestSnapshotId + '" data-model-label="' + esc(m.label || m.modelId) + '" data-current-pct="' + mpct + '">' +
          '<button class="adj-btn" data-delta="-10" title="−10%">−10</button>' +
          '<button class="adj-btn" data-delta="-5" title="−5%">−5</button>' +
          '<button class="adj-btn" data-delta="5" title="+5%">+5</button>' +
          '<button class="adj-btn" data-delta="10" title="+10%">+10</button>' +
          '</span>';

        modelRows += '<div class="model-row">' +
          '<div class="model-indicator" style="background:' + color + '"></div>' +
          '<span class="model-label">' + esc(m.label || m.modelId) + '</span>' +
          '<div class="model-bar-track"><div class="model-bar-fill ' + mcls + '" style="width:' + mpct + '%"></div></div>' +
          '<span class="model-pct ' + mcls + '">' + mpct + '%</span>' +
          adjustBtns +
          '<span class="model-reset">' + resetStr + '</span>' +
          intellBadges +
          '</div>';
        }
      }
      var expandedCls = isExpanded ? ' is-expanded' : '';
      modelsHTML = '<div class="model-details' + expandedCls + '" id="' + accId + '">' + modelRows +
        '<div class="account-actions">' +
        '<button class="btn-clear-snaps" data-clear-account="' + acc.accountId + '" data-clear-email="' + esc(acc.email) + '" title="Delete all snapshots for this account">Clear Snapshots</button>' +
        '<button class="btn-delete-account" data-delete-account="' + acc.accountId + '" data-delete-email="' + esc(acc.email) + '" title="Remove account and all its data">Remove Account</button>' +
        '</div></div>';
    }

    var chevronCls = isExpanded ? 'chevron expanded' : 'chevron';
    // Bug 5 fix: Dim based on quota readiness, not snap age.
    // Accounts with any depleted group get dimmed; fully ready = bright.
    var staleStyle = '';
    if (!acc.isReady) {
      staleStyle = ' style="opacity:0.6"';
    }
    html += '<div class="account-card"' + staleStyle + '>' +
      '<div class="account-row" data-toggle="' + accId + '">' +
      '<div class="account-info">' +
      '<div class="account-email"><span class="' + chevronCls + '" id="chev-' + accId + '">▸</span> ' + esc(acc.email) +
      renderPinnedBadge(pinnedGroupData, pinnedKey) + '</div>' +
      '<div class="account-meta" style="position:relative">' +
      (acc.planName ? '<span class="plan-badge">' + esc(acc.planName) + '</span>' : '') +
      renderAccountTags(acc) +
      renderAccountNote(acc) +
      '</div></div>' +
      groupCells +
      creditsCell +
      '<div class="snap-cell"><span class="snap-ago">' + esc(acc.stalenessLabel) + '</span></div>' +
      '<div style="text-align:center"><span class="health-dot ' + dotCls + '">● ' + badgeText + '</span></div>' +
      '</div>' +
      modelsHTML +
      '</div>';
  }

  html += '</div></div>'; // close provider-body + provider-section
  } // end if acctCount > 0

  // Bug 6 fix: Apply status filter to Codex/Claude sections too
  var sf = document.getElementById('quota-filter-status');
  var statusVal = sf ? (sf as HTMLSelectElement).value : 'all';
  if (data.codexSnapshot && (pf === 'all' || pf === 'codex')) {
    var cxStatus = getCodexClaudeStatus(data.codexSnapshot);
    if (statusVal === 'all' || cxStatus === statusVal) {
      html += renderCodexProviderSection(data.codexSnapshot);
    }
  }
  if (data.claudeSnapshot && (pf === 'all' || pf === 'claude')) {
    var clStatus = getCodexClaudeStatus(data.claudeSnapshot);
    if (statusVal === 'all' || clStatus === statusVal) {
      html += renderClaudeProviderSection(data.claudeSnapshot);
    }
  }

  // V3: Empty states when a specific provider is selected but has no data
  if (pf === 'antigravity' && acctCount === 0) {
    html += '<div class="provider-empty-state" data-provider="antigravity">' +
      '<span class="provider-empty-icon">⚡</span>' +
      '<p>No Antigravity accounts detected</p>' +
      '<p class="empty-hint">Open Windsurf and log in to start tracking quotas</p></div>';
  }
  if (pf === 'codex' && !data.codexSnapshot) {
    html += '<div class="provider-empty-state" data-provider="codex">' +
      '<span class="provider-empty-icon">🤖</span>' +
      '<p>No Codex snapshots yet</p>' +
      '<p class="empty-hint">Install Codex CLI and click <strong>Snap Now</strong> to capture</p></div>';
  }
  if (pf === 'claude' && !data.claudeSnapshot) {
    html += '<div class="provider-empty-state" data-provider="claude">' +
      '<span class="provider-empty-icon">🔮</span>' +
      '<p>No Claude Code data yet</p>' +
      '<p class="empty-hint">Enable the Claude bridge in <strong>Settings</strong></p></div>';
  }

  grid.innerHTML = html;

  // Wire up provider section collapse (state already baked into HTML)
  grid.querySelectorAll('.provider-header[data-toggle-provider]').forEach(function(hdr) {
    (hdr as HTMLElement).addEventListener('click', function() {
      var targetId = (hdr as HTMLElement).dataset.toggleProvider;
      var body = document.getElementById(targetId!);
      var chev = document.getElementById('pchev-' + targetId!);
      if (!body) return;
      var collapsed = body.classList.toggle('collapsed');
      if (chev) chev.textContent = collapsed ? '▸' : '▾';
      if (collapsed) {
        collapsedProviders.add(targetId!);
      } else {
        collapsedProviders.delete(targetId!);
      }
    });
  });
}

export function renderCodexProviderSection(cs: any): string {
  var fiveUsed = cs.fiveHourPct || 0;
  var fiveRem = Math.max(0, 100 - fiveUsed);
  var fiveCls = fiveRem > 50 ? 'good' : fiveRem > 20 ? 'ok' : fiveRem > 0 ? 'warning' : 'exhausted';
  var fiveReset = cs.fiveHourReset ? formatResetTime(cs.fiveHourReset) : '';
  var sevenUsed = cs.sevenDayPct ? cs.sevenDayPct : 0;
  var sevenRem = Math.max(0, 100 - sevenUsed);
  var sevenCls = sevenRem > 50 ? 'good' : sevenRem > 20 ? 'ok' : sevenRem > 0 ? 'warning' : 'exhausted';
  var sevenReset = cs.sevenDayReset ? formatResetTime(cs.sevenDayReset) : '';
  var capturedAgo = cs.capturedAt ? formatTimeAgo(cs.capturedAt) : 'unknown';
  var dotCls = (fiveUsed >= 80 || sevenUsed >= 80) ? 'dot-low' : 'dot-ready';
  var dotText = dotCls === 'dot-ready' ? 'Ready' : 'Low';
  var displayName = cs.email || (cs.accountId && cs.accountId.length > 12 ? cs.accountId.substring(0,6) + '..' + cs.accountId.slice(-6) : (cs.accountId || 'Codex'));
  var creditsStr = cs.creditsBalance !== null && cs.creditsBalance !== undefined ? cs.creditsBalance.toFixed(2) : String.fromCharCode(8212);
  var cxCollapseClass = collapsedProviders.has('section-codex') ? ' collapsed' : '';
  var cxChevron = collapsedProviders.has('section-codex') ? '▸' : '▾';
  return '<div class="provider-section" data-provider="codex">' +
    '<div class="provider-header" data-toggle-provider="section-codex">' +
    '<div class="provider-header-left">' +
    '<span class="provider-chevron" id="pchev-section-codex">' + cxChevron + '</span>' +
    '<span class="provider-name">\ud83e\udd16 Codex / ChatGPT</span>' +
    '<span class="provider-count">1 account</span>' +
    '</div></div>' +
    '<div class="provider-body' + cxCollapseClass + '" id="section-codex">' +
    '<div class="grid-header grid-codex">' +
    '<div>Account</div><div>Plan</div><div>5-Hour</div><div>7-Day</div><div>Credits</div><div>Last Snap</div><div>Status</div>' +
    '</div>' +
    '<div class="account-card"><div class="account-row grid-codex">' +
    '<div class="account-info"><div class="account-email">' + esc(displayName) + '</div></div>' +
    '<div>' + (cs.planType ? '<span class="plan-badge">' + esc(cs.planType) + '</span>' : String.fromCharCode(8212)) + '</div>' +
    '<div class="quota-cell"><span class="quota-pct ' + fiveCls + '">' + fiveRem.toFixed(0) + '%</span>' +
    '<div class="quota-minibar"><div class="quota-minibar-fill ' + fiveCls + '" style="width:' + fiveRem + '%"></div></div>' +
    (fiveReset ? '<span class="quota-reset">\u21bb ' + fiveReset + '</span>' : '') + '</div>' +
    '<div class="quota-cell"><span class="quota-pct ' + sevenCls + '">' + sevenRem.toFixed(0) + '%</span>' +
    '<div class="quota-minibar"><div class="quota-minibar-fill ' + sevenCls + '" style="width:' + sevenRem + '%"></div></div>' +
    (sevenReset ? '<span class="quota-reset">\u21bb ' + sevenReset + '</span>' : '') + '</div>' +
    '<div class="credits-cell"><span class="credit-amount">' + creditsStr + '</span></div>' +
    '<div class="snap-cell"><span class="snap-ago">' + capturedAgo + '</span></div>' +
    '<div style="text-align:center"><span class="health-dot ' + dotCls + '">\u25cf ' + dotText + '</span></div>' +
    '</div></div></div></div>';
}

export function renderClaudeProviderSection(cl: any): string {
  var clFive = cl.fiveHourPct || 0;
  var clFiveRem = Math.max(0, 100 - clFive);
  var clFiveCls = clFiveRem > 50 ? 'good' : clFiveRem > 20 ? 'ok' : clFiveRem > 0 ? 'warning' : 'exhausted';
  var clSeven = cl.sevenDayPct ? cl.sevenDayPct : 0;
  var clSevenRem = Math.max(0, 100 - clSeven);
  var clSevenCls = clSevenRem > 50 ? 'good' : clSevenRem > 20 ? 'ok' : clSevenRem > 0 ? 'warning' : 'exhausted';
  var clAgo = cl.capturedAt ? formatTimeAgo(cl.capturedAt) : 'unknown';
  var dotCls = (clFive >= 80 || clSeven >= 80) ? 'dot-low' : 'dot-ready';
  var dotText = dotCls === 'dot-ready' ? 'Ready' : 'Low';
  var clCollapseClass = collapsedProviders.has('section-claude') ? ' collapsed' : '';
  var clChevron = collapsedProviders.has('section-claude') ? '▸' : '▾';
  return '<div class="provider-section" data-provider="claude">' +
    '<div class="provider-header" data-toggle-provider="section-claude">' +
    '<div class="provider-header-left">' +
    '<span class="provider-chevron" id="pchev-section-claude">' + clChevron + '</span>' +
    '<span class="provider-name">\ud83d\udd17 Claude Code</span>' +
    '<span class="provider-count">1 account \u00b7 Bridge</span>' +
    '</div></div>' +
    '<div class="provider-body' + clCollapseClass + '" id="section-claude">' +
    '<div class="grid-header grid-claude">' +
    '<div>Source</div><div>5-Hour</div><div>7-Day</div><div>Last Snap</div><div>Status</div>' +
    '</div>' +
    '<div class="account-card"><div class="account-row grid-claude">' +
    '<div class="account-info"><div class="account-email">' + esc(cl.source || 'statusline') + '</div></div>' +
    '<div class="quota-cell"><span class="quota-pct ' + clFiveCls + '">' + clFiveRem.toFixed(0) + '%</span>' +
    '<div class="quota-minibar"><div class="quota-minibar-fill ' + clFiveCls + '" style="width:' + clFiveRem + '%"></div></div></div>' +
    '<div class="quota-cell"><span class="quota-pct ' + clSevenCls + '">' + clSevenRem.toFixed(0) + '%</span>' +
    '<div class="quota-minibar"><div class="quota-minibar-fill ' + clSevenCls + '" style="width:' + clSevenRem + '%"></div></div></div>' +
    '<div class="snap-cell"><span class="snap-ago">' + clAgo + '</span></div>' +
    '<div style="text-align:center"><span class="health-dot ' + dotCls + '">\u25cf ' + dotText + '</span></div>' +
    '</div></div></div></div>';
}



export function formatResetTime(isoString: string | null): string {
  if (!isoString) return '';
  var reset = new Date(isoString);
  var now = new Date();
  var diffSec = (reset.getTime() - now.getTime()) / 1000;
  if (diffSec <= 0) return 'now';
  return formatSeconds(diffSec);
}
