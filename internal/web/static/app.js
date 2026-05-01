// Niyantra Dashboard JS — Unified Quota + Subscription Manager

const GROUP_ORDER = ['claude_gpt', 'gemini_pro', 'gemini_flash'];
const GROUP_COLORS = { claude_gpt: '#D97757', gemini_pro: '#10B981', gemini_flash: '#3B82F6' };

// Track which accounts are expanded (survives re-renders)
const expandedAccounts = new Set();

// Platform presets (loaded from API)
var presetsData = [];

// ════════════════════════════════════════════
//  THEME
// ════════════════════════════════════════════

function initTheme() {
  var saved = localStorage.getItem('niyantra-theme');
  if (saved) {
    document.documentElement.setAttribute('data-theme', saved);
  } else if (window.matchMedia('(prefers-color-scheme: light)').matches) {
    document.documentElement.setAttribute('data-theme', 'light');
  }

  document.getElementById('theme-btn').addEventListener('click', function() {
    var current = document.documentElement.getAttribute('data-theme');
    var next = current === 'light' ? 'dark' : 'light';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('niyantra-theme', next);
    // M2: Update chart colors in-place to avoid flash
    updateChartTheme(next);
  });
}

// ════════════════════════════════════════════
//  TABS
// ════════════════════════════════════════════

function initTabs() {
  var btns = document.querySelectorAll('.tab-btn');
  btns.forEach(function(btn) {
    btn.addEventListener('click', function() {
      var tab = btn.getAttribute('data-tab');
      btns.forEach(function(b) { b.classList.remove('active'); });
      btn.classList.add('active');

      document.querySelectorAll('.tab-panel').forEach(function(p) {
        p.classList.remove('active');
      });
      document.getElementById('panel-' + tab).classList.add('active');

      // Load tab data on activation
      if (tab === 'subscriptions') loadSubscriptions();
      if (tab === 'overview') loadOverview();
      if (tab === 'settings') { loadActivityLog(); loadMode(); loadDataSources(); }
    });
  });
}

// ════════════════════════════════════════════
//  API — Quotas (existing auto-tracked)
// ════════════════════════════════════════════

function fetchStatus() {
  return fetch('/api/status').then(function(res) {
    if (!res.ok) throw new Error('Failed to fetch status');
    return res.json();
  });
}

function triggerSnap() {
  return fetch('/api/snap', { method: 'POST' }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Snap failed');
      return data;
    });
  });
}

// ════════════════════════════════════════════
//  API — Subscriptions
// ════════════════════════════════════════════

function fetchSubscriptions(status, category) {
  var params = new URLSearchParams();
  if (status) params.set('status', status);
  if (category) params.set('category', category);
  var url = '/api/subscriptions' + (params.toString() ? '?' + params : '');
  return fetch(url).then(function(res) { return res.json(); });
}

function createSubscription(sub) {
  return fetch('/api/subscriptions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub),
  }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Create failed');
      return data;
    });
  });
}

function updateSubscription(id, sub) {
  return fetch('/api/subscriptions/' + id, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub),
  }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Update failed');
      return data;
    });
  });
}

function deleteSubscription(id) {
  return fetch('/api/subscriptions/' + id, { method: 'DELETE' }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Delete failed');
      return data;
    });
  });
}

function fetchOverview() {
  return fetch('/api/overview').then(function(res) { return res.json(); });
}

function fetchPresets() {
  return fetch('/api/presets').then(function(res) { return res.json(); });
}

// Usage intelligence cache (populated by fetchUsage)
var usageDataCache = null;

function fetchUsage(accountId) {
  var url = '/api/usage';
  if (accountId) url += '?account=' + accountId;
  return fetch(url).then(function(res) { return res.json(); }).then(function(data) {
    usageDataCache = data;
    return data;
  });
}

// ════════════════════════════════════════════
//  RENDER — Quotas Tab
// ════════════════════════════════════════════

var quotaSortState = { column: 'account', direction: 'asc' };
var latestQuotaData = null;

function getGroupPct(acc, groupKey) {
  if (!acc.groups) return -1;
  for (var i = 0; i < acc.groups.length; i++) {
    if (acc.groups[i].groupKey === groupKey) return acc.groups[i].remainingPercent;
  }
  return -1;
}

function getAICredits(acc) {
  if (acc.aiCredits && acc.aiCredits.length > 0) return acc.aiCredits[0].creditAmount;
  return -1;
}

function allExhausted(acc) {
  var grps = acc.groups || [];
  if (grps.length === 0) return false;
  for (var i = 0; i < grps.length; i++) {
    if (!grps[i].isExhausted && grps[i].remainingPercent > 0) return false;
  }
  return true;
}

function sortAccountsArray(accounts) {
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

function filterAccountsArray(accounts) {
  var searchInput = document.getElementById('quota-search');
  var statusFilter = document.getElementById('quota-filter-status');
  var query = searchInput ? searchInput.value.toLowerCase() : '';
  var status = statusFilter ? statusFilter.value : 'all';

  return accounts.filter(function(acc) {
    var matchesSearch = !query ||
      acc.email.toLowerCase().includes(query) ||
      (acc.planName || '').toLowerCase().includes(query);

    var matchesStatus = true;
    if (status === 'ready') matchesStatus = acc.isReady;
    else if (status === 'low') matchesStatus = !acc.isReady && !allExhausted(acc);
    else if (status === 'empty') matchesStatus = allExhausted(acc);

    return matchesSearch && matchesStatus;
  });
}

function updateSortHeaders() {
  document.querySelectorAll('.grid-header .sortable').forEach(function(el) {
    el.classList.remove('sort-active');
    var span = el.querySelector('.sort-indicator');
    if (span) span.textContent = '';
    if (el.dataset.sort === quotaSortState.column) {
      el.classList.add('sort-active');
      if (span) span.textContent = quotaSortState.direction === 'asc' ? '▾' : '▴';
    }
  });
}

function renderAccounts(data) {
  latestQuotaData = data;
  var grid = document.getElementById('account-grid');
  var countBadge = document.getElementById('account-count');
  var snapCount = document.getElementById('snap-count');
  if (!grid) return;

  var acctCount = (data.accounts || []).length;
  var parts = [];
  if (acctCount > 0) parts.push(acctCount + ' Antigravity');
  if (data.codexSnapshot) parts.push('1 Codex');
  if (data.claudeSnapshot) parts.push('1 Claude');
  countBadge.textContent = parts.join(' · ') || '0 accounts';
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
  var pf = providerFilter ? providerFilter.value : 'all';
  var html = '';
  if (acctCount > 0 && (pf === 'all' || pf === 'antigravity')) {
  var filtered = filterAccountsArray(data.accounts);
  var sorted = sortAccountsArray(filtered);
  html += '<div class="provider-section"><div class="provider-header" data-toggle-provider="section-antigravity">' +
    '<div class="provider-header-left"><span class="provider-chevron" id="pchev-section-antigravity">▾</span>' +
    '<span class="provider-name">Antigravity</span>' +
    '<span class="provider-count">' + acctCount + ' account' + (acctCount !== 1 ? 's' : '') + '</span></div></div>' +
    '<div class="provider-body" id="section-antigravity">';
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
    var isExpanded = expandedAccounts.has(accId);

    var groupCells = '';
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
      groupCells += '<div class="quota-cell">' +
        '<span class="quota-pct ' + cls + '">' + pct + '%</span>' +
        '<div class="quota-minibar"><div class="quota-minibar-fill ' + barCls + '" style="width:' + pct + '%"></div></div>' +
        reset + '</div>';
    }


    // Q5: Health dots — visual status
    var dotCls = 'dot-ready';
    var badgeText = 'Ready';
    if (allExhausted(acc)) { dotCls = 'dot-empty'; badgeText = 'Empty'; }
    else if (!acc.isReady) { dotCls = 'dot-low'; badgeText = 'Low'; }

    var creditsCell = '<div class="credits-cell">';
    if (acc.aiCredits && acc.aiCredits.length > 0) {
      var credits = acc.aiCredits[0].creditAmount;
      var creditCls = credits > 500 ? 'good' : credits > 100 ? 'ok' : 'warning';
      creditsCell += '<span class="credit-amount ' + creditCls + '" title="AI Credits">✦ ' +
        formatCredits(credits) + '</span>';
    } else {
      creditsCell += '<span class="credit-amount muted">—</span>';
    }
    creditsCell += '</div>';

    var modelsHTML = '';
    if (acc.models && acc.models.length > 0) {
      var modelRows = '';
      for (var mi = 0; mi < acc.models.length; mi++) {
        var m = acc.models[mi];
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
                var minsLeft = Math.round((exhaust - Date.now()) / 60000);
                if (minsLeft > 0) {
                  intellBadges += '<span class="exhaust-badge" title="Projected exhaustion time">⚠ ' + (minsLeft > 60 ? Math.round(minsLeft/60) + 'h' : minsLeft + 'm') + '</span>';
                }
              }
              break;
            }
          }
        }

        modelRows += '<div class="model-row">' +
          '<div class="model-indicator" style="background:' + color + '"></div>' +
          '<span class="model-label">' + esc(m.label || m.modelId) + '</span>' +
          '<div class="model-bar-track"><div class="model-bar-fill ' + mcls + '" style="width:' + mpct + '%"></div></div>' +
          '<span class="model-pct ' + mcls + '">' + mpct + '%</span>' +
          '<span class="model-reset">' + resetStr + '</span>' +
          intellBadges +
          '</div>';
      }
      var expandedCls = isExpanded ? ' is-expanded' : '';
      modelsHTML = '<div class="model-details' + expandedCls + '" id="' + accId + '">' + modelRows +
        '<div class="account-actions">' +
        '<button class="btn-clear-snaps" data-clear-account="' + acc.accountId + '" data-clear-email="' + esc(acc.email) + '" title="Delete all snapshots for this account">Clear Snapshots</button>' +
        '<button class="btn-delete-account" data-delete-account="' + acc.accountId + '" data-delete-email="' + esc(acc.email) + '" title="Remove account and all its data">Remove Account</button>' +
        '</div></div>';
    }

    var chevronCls = isExpanded ? 'chevron expanded' : 'chevron';
    // Q3: Dim rows older than 24h
    var staleStyle = '';
    if (acc.lastSeen) {
      var ageMs = Date.now() - new Date(acc.lastSeen).getTime();
      if (ageMs > 86400000) staleStyle = ' style="opacity:0.65"';
    }
    html += '<div class="account-card"' + staleStyle + '>' +
      '<div class="account-row" data-toggle="' + accId + '">' +
      '<div class="account-info">' +
      '<div class="account-email"><span class="' + chevronCls + '" id="chev-' + accId + '">▸</span> ' + esc(acc.email) + '</div>' +
      '<div class="account-meta">' +
      (acc.planName ? '<span class="plan-badge">' + esc(acc.planName) + '</span>' : '') +
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

  if (data.codexSnapshot && (pf === 'all' || pf === 'codex')) html += renderCodexProviderSection(data.codexSnapshot);
  if (data.claudeSnapshot && (pf === 'all' || pf === 'claude')) html += renderClaudeProviderSection(data.claudeSnapshot);

  grid.innerHTML = html;

  // Wire up provider section collapse
  grid.querySelectorAll('.provider-header[data-toggle-provider]').forEach(function(hdr) {
    hdr.addEventListener('click', function() {
      var targetId = hdr.dataset.toggleProvider;
      var body = document.getElementById(targetId);
      var chev = document.getElementById('pchev-' + targetId);
      if (!body) return;
      var collapsed = body.classList.toggle('collapsed');
      if (chev) chev.textContent = collapsed ? '▸' : '▾';
    });
  });
}

function renderCodexProviderSection(cs) {
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
  return '<div class="provider-section">' +
    '<div class="provider-header" data-toggle-provider="section-codex">' +
    '<div class="provider-header-left">' +
    '<span class="provider-chevron" id="pchev-section-codex">▾</span>' +
    '<span class="provider-name">\ud83e\udd16 Codex / ChatGPT</span>' +
    '<span class="provider-count">1 account</span>' +
    '</div></div>' +
    '<div class="provider-body" id="section-codex">' +
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

function renderClaudeProviderSection(cl) {
  var clFive = cl.fiveHourPct || 0;
  var clFiveRem = Math.max(0, 100 - clFive);
  var clFiveCls = clFiveRem > 50 ? 'good' : clFiveRem > 20 ? 'ok' : clFiveRem > 0 ? 'warning' : 'exhausted';
  var clSeven = cl.sevenDayPct ? cl.sevenDayPct : 0;
  var clSevenRem = Math.max(0, 100 - clSeven);
  var clSevenCls = clSevenRem > 50 ? 'good' : clSevenRem > 20 ? 'ok' : clSevenRem > 0 ? 'warning' : 'exhausted';
  var clAgo = cl.capturedAt ? formatTimeAgo(cl.capturedAt) : 'unknown';
  var dotCls = (clFive >= 80 || clSeven >= 80) ? 'dot-low' : 'dot-ready';
  var dotText = dotCls === 'dot-ready' ? 'Ready' : 'Low';
  return '<div class="provider-section">' +
    '<div class="provider-header" data-toggle-provider="section-claude">' +
    '<div class="provider-header-left">' +
    '<span class="provider-chevron" id="pchev-section-claude">▾</span>' +
    '<span class="provider-name">\ud83d\udd17 Claude Code</span>' +
    '<span class="provider-count">1 account \u00b7 Bridge</span>' +
    '</div></div>' +
    '<div class="provider-body" id="section-claude">' +
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



function formatResetTime(isoString) {
  if (!isoString) return '';
  var reset = new Date(isoString);
  var now = new Date();
  var diffSec = (reset - now) / 1000;
  if (diffSec <= 0) return 'now';
  return formatSeconds(diffSec);
}

// ════════════════════════════════════════════
//  RENDER — Subscriptions Tab
// ════════════════════════════════════════════

function loadSubscriptions() {
  var status = document.getElementById('filter-status').value;
  var category = document.getElementById('filter-category').value;

  fetchSubscriptions(status, category).then(function(data) {
    renderSubscriptions(data);
  }).catch(function(err) {
    console.error('Failed to load subscriptions:', err);
  });
}

function renderSubscriptions(data) {
  var grid = document.getElementById('subs-grid');
  var summary = document.getElementById('subs-summary');
  if (!grid) return;

  var subs = data.subscriptions || [];
  summary.textContent = subs.length + ' subscription' + (subs.length !== 1 ? 's' : '');

  if (subs.length === 0) {
    grid.innerHTML = '<div class="empty-state">' +
      '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.4"><rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/></svg>' +
      '<p>No subscriptions tracked yet</p>' +
      '<p class="empty-hint">Click <strong>+ Add</strong> to add your first AI subscription</p>' +
      '</div>';
    return;
  }

  // S2: Split subs into auto-tracked provider groups vs manual
  var providerGroups = {}; // platform -> array of auto-tracked subs
  var manualSubs = [];
  for (var i = 0; i < subs.length; i++) {
    var s = subs[i];
    if (s.autoTracked) {
      var pkey = s.platform || 'Unknown';
      if (!providerGroups[pkey]) providerGroups[pkey] = [];
      providerGroups[pkey].push(s);
    } else {
      manualSubs.push(s);
    }
  }

  var html = '';

  // Render auto-tracked provider sections as compact collapsible tables
  var providerKeys = Object.keys(providerGroups);
  for (var pi = 0; pi < providerKeys.length; pi++) {
    var provider = providerKeys[pi];
    var items = providerGroups[provider];
    var totalMonthly = 0;
    for (var ti = 0; ti < items.length; ti++) {
      if (items[ti].billingCycle === 'monthly') totalMonthly += items[ti].costAmount;
      else if (items[ti].billingCycle === 'yearly') totalMonthly += items[ti].costAmount / 12;
    }

    var sectionId = 'provider-section-' + provider.replace(/\s+/g, '-').toLowerCase();
    html += '<div class="provider-section">' +
      '<div class="provider-header" data-toggle-provider="' + sectionId + '">' +
      '<div class="provider-header-left">' +
      '<span class="provider-chevron" id="pchev-' + sectionId + '">▾</span> ' +
      '<span class="provider-name">' + esc(provider) + '</span>' +
      '<span class="provider-count">' + items.length + ' account' + (items.length !== 1 ? 's' : '') + '</span>' +
      '</div>' +
      '<span class="provider-spend">$' + totalMonthly.toFixed(2) + '/mo</span>' +
      '</div>' +
      '<div class="provider-body" id="' + sectionId + '">' +
      '<table class="provider-table">' +
      '<thead><tr><th>Account</th><th>Plan</th><th>Cost</th><th>Status</th><th></th></tr></thead>' +
      '<tbody>';

    for (var si = 0; si < items.length; si++) {
      var sub = items[si];
      var sym = currencySymbol(sub.costCurrency);
      var costStr = sub.costAmount > 0 ? (sym + sub.costAmount.toFixed(2) + '/' + (sub.billingCycle || 'mo')) : '—';
      var statusCls = sub.status === 'active' ? 'ready' : (sub.status === 'trial' ? 'partial' : 'exhausted');
      html += '<tr>' +
        '<td class="provider-row-email">' + esc(sub.email || '—') + '</td>' +
        '<td>' + esc(sub.planName || '—') + '</td>' +
        '<td class="provider-row-cost">' + costStr + '</td>' +
        '<td><span class="status-badge ' + statusCls + '">' + esc(sub.status) + '</span></td>' +
        '<td class="provider-row-actions">' +
        '<button class="btn-delete-card btn-delete-sm" data-delete-id="' + sub.id + '" data-delete-name="' + esc(sub.platform) + '" title="Delete">×</button>' +
        '</td>' +
        '</tr>';
    }

    html += '</tbody></table></div></div>';
  }

  // Render manual subs with full card layout, grouped by category
  if (manualSubs.length > 0) {
    var grouped = {};
    for (var mi = 0; mi < manualSubs.length; mi++) {
      var cat = manualSubs[mi].category || 'other';
      if (!grouped[cat]) grouped[cat] = [];
      grouped[cat].push(manualSubs[mi]);
    }
    var catOrder = ['coding', 'chat', 'api', 'image', 'audio', 'productivity', 'other'];
    if (providerKeys.length > 0) {
      html += '<div class="sub-category-label" style="margin-top:24px">Manual Subscriptions (' + manualSubs.length + ')</div>';
    }
    for (var ci = 0; ci < catOrder.length; ci++) {
      var catItems = grouped[catOrder[ci]];
      if (!catItems || catItems.length === 0) continue;
      if (manualSubs.length > 5) {
        html += '<div class="sub-category-label">' + esc(catOrder[ci]) + ' (' + catItems.length + ')</div>';
      }
      for (var csi = 0; csi < catItems.length; csi++) {
        html += renderSubCard(catItems[csi]);
      }
    }
  } else if (providerKeys.length > 0) {
    // Show helpful empty state for manual subs
    html += '<div class="manual-empty">' +
      '<p>No manual subscriptions tracked.</p>' +
      '<p class="empty-hint">Click <strong>+ Add</strong> to track Claude, Cursor, or other AI services.</p>' +
      '</div>';
  }

  grid.innerHTML = html;

  // Wire up provider section collapse/expand
  grid.querySelectorAll('.provider-header').forEach(function(hdr) {
    hdr.addEventListener('click', function() {
      var targetId = hdr.dataset.toggleProvider;
      var body = document.getElementById(targetId);
      var chev = document.getElementById('pchev-' + targetId);
      if (!body) return;
      var collapsed = body.classList.toggle('collapsed');
      if (chev) chev.textContent = collapsed ? '▸' : '▾';
    });
  });
}

function renderSubCard(sub) {
  // Cost display
  var costHTML = '';
  if (sub.costAmount > 0) {
    var sym = currencySymbol(sub.costCurrency);
    costHTML = '<div class="sub-card-cost">' + sym + sub.costAmount.toFixed(2) +
      ' <span class="cycle">/' + esc(sub.billingCycle) + '</span></div>';
  } else if (sub.billingCycle === 'payg') {
    costHTML = '<div class="sub-card-cost">Pay-as-you-go</div>';
  }

  // Limits chips
  var limitsHTML = '';
  var chips = [];
  if (sub.tokenLimit > 0) chips.push(formatNumber(sub.tokenLimit) + ' tokens/' + esc(sub.limitPeriod));
  if (sub.creditLimit > 0) chips.push(formatNumber(sub.creditLimit) + ' credits/' + esc(sub.limitPeriod));
  if (sub.requestLimit > 0) chips.push(formatNumber(sub.requestLimit) + ' requests/' + esc(sub.limitPeriod));
  if (chips.length > 0) {
    limitsHTML = '<div class="sub-card-limits">';
    for (var c = 0; c < chips.length; c++) {
      limitsHTML += '<span class="sub-limit-chip">' + chips[c] + '</span>';
    }
    limitsHTML += '</div>';
  }

  // Badges
  var badgesHTML = '<span class="sub-status-badge ' + esc(sub.status) + '">' + esc(sub.status) + '</span>';
  badgesHTML += '<span class="sub-cat-badge">' + esc(sub.category) + '</span>';
  if (sub.autoTracked) badgesHTML += '<span class="sub-auto-badge">AUTO</span>';

  // Trial countdown
  var trialHTML = '';
  if (sub.daysUntilTrialEnd !== undefined && sub.daysUntilTrialEnd !== null) {
    if (sub.daysUntilTrialEnd <= 0) {
      trialHTML = '<span class="trial-countdown">Trial expired!</span>';
    } else if (sub.daysUntilTrialEnd <= 7) {
      trialHTML = '<span class="trial-countdown">Trial ends in ' + sub.daysUntilTrialEnd + 'd</span>';
    }
  }

  // Renewal
  var renewalHTML = '';
  if (sub.nextRenewal && sub.daysUntilRenewal !== undefined) {
    var rCls = sub.daysUntilRenewal <= 7 ? 'soon' : '';
    if (sub.daysUntilRenewal < 0) rCls = 'overdue';
    renewalHTML = '<span class="sub-renewal-tag ' + rCls + '">Renews: ' + sub.nextRenewal +
      ' (' + sub.daysUntilRenewal + 'd)</span>';
  }

  // Links
  var linksHTML = '';
  if (sub.url || sub.statusPageUrl) {
    linksHTML = '<div class="sub-card-links">';
    if (sub.url) linksHTML += '<a href="' + esc(sub.url) + '" target="_blank" rel="noopener">🔗 Dashboard</a>';
    if (sub.statusPageUrl) linksHTML += '<a href="' + esc(sub.statusPageUrl) + '" target="_blank" rel="noopener">🟢 Status</a>';
    linksHTML += '</div>';
  }

  // Notes
  var notesHTML = '';
  if (sub.notes) {
    notesHTML = '<div class="sub-card-notes">' + esc(sub.notes) + '</div>';
  }

  // Meta line
  var metaParts = [];
  if (sub.email) metaParts.push(esc(sub.email));
  if (sub.planName) metaParts.push(esc(sub.planName));
  var metaHTML = metaParts.length > 0
    ? '<div class="sub-card-meta">' + metaParts.join(' · ') + '</div>'
    : '';

  // S1: Auto-tracked subs show email as title (the differentiator), platform as subtitle
  var cardTitle, cardSubtitle;
  if (sub.autoTracked && sub.email) {
    cardTitle = esc(sub.email);
    cardSubtitle = '<span class="sub-card-platform-badge">' + esc(sub.platform) + (sub.planName ? ' · ' + esc(sub.planName) : '') + '</span>';
  } else {
    cardTitle = esc(sub.platform);
    cardSubtitle = '';
  }

  // S3: Remove AUTO badge from badgesHTML for auto-tracked (context is implicit)
  if (sub.autoTracked) {
    badgesHTML = badgesHTML.replace(/<span[^>]*>AUTO<\/span>/i, '');
  }

  // M1: Generate a unique accent color from platform+email for visual differentiation
  var colorSeed = (sub.platform || '') + (sub.email || '') + sub.id;
  var hue = 0;
  for (var ci = 0; ci < colorSeed.length; ci++) {
    hue = (hue + colorSeed.charCodeAt(ci) * 31) % 360;
  }
  var accentStyle = 'border-left: 3px solid hsl(' + hue + ', 60%, 55%)';

  return '<div class="sub-card" data-sub-id="' + sub.id + '" style="' + accentStyle + '">' +
    '<div class="sub-card-header">' +
    '<div class="sub-card-title">' + cardTitle + '</div>' +
    '<div class="sub-card-badges">' + trialHTML + badgesHTML + '</div>' +
    '</div>' +
    (cardSubtitle ? '<div class="sub-card-subtitle">' + cardSubtitle + '</div>' : '') +
    metaHTML +
    costHTML +
    limitsHTML +
    notesHTML +
    linksHTML +
    renewalHTML +
    '<div class="sub-card-actions">' +
    '<button class="btn-edit-card" data-edit-id="' + sub.id + '">Edit</button>' +
    '<button class="btn-delete-card" data-delete-id="' + sub.id + '" data-delete-name="' + esc(sub.platform) + '">Delete</button>' +
    '</div>' +
    '</div>';
}




// ════════════════════════════════════════════
//  TOGGLE — Quotas expand/collapse
// ════════════════════════════════════════════

function setupToggle() {
  var grid = document.getElementById('account-grid');
  if (!grid) return;

  grid.addEventListener('click', function(e) {
    // Handle clear snapshots button
    var clearBtn = e.target.closest('[data-clear-account]');
    if (clearBtn) {
      e.stopPropagation();
      var accountId = clearBtn.getAttribute('data-clear-account');
      var email = clearBtn.getAttribute('data-clear-email');
      if (confirm('Clear all snapshots for ' + email + '?\n\nThe account will remain but all quota history will be deleted. This cannot be undone.')) {
        fetch('/api/accounts/' + accountId + '/snapshots', { method: 'DELETE' })
          .then(function(res) { return res.json(); })
          .then(function(data) {
            showToast('✅ Cleared ' + (data.snapshotsDeleted || 0) + ' snapshots for ' + email, 'success');
            fetchStatus().then(renderAccounts);
            loadHistoryChart();
          })
          .catch(function(err) { showToast('❌ ' + err.message, 'error'); });
      }
      return;
    }

    // Handle delete account button
    var deleteBtn = e.target.closest('[data-delete-account]');
    if (deleteBtn) {
      e.stopPropagation();
      var accountId2 = deleteBtn.getAttribute('data-delete-account');
      var email2 = deleteBtn.getAttribute('data-delete-email');
      if (confirm('Remove account ' + email2 + '?\n\nThis will permanently delete the account and ALL associated data (snapshots, cycles, codex data). This cannot be undone.')) {
        fetch('/api/accounts/' + accountId2, { method: 'DELETE' })
          .then(function(res) { return res.json(); })
          .then(function(data) {
            showToast('✅ Removed ' + email2 + ' (' + (data.totalDeleted || 0) + ' records deleted)', 'success');
            expandedAccounts.delete('acc-' + accountId2);
            fetchStatus().then(renderAccounts);
            loadHistoryChart();
          })
          .catch(function(err) { showToast('❌ ' + err.message, 'error'); });
      }
      return;
    }

    // Handle row toggle (existing)
    var row = e.target.closest('.account-row[data-toggle]');
    if (!row) return;
    var id = row.getAttribute('data-toggle');
    var el = document.getElementById(id);
    var chev = document.getElementById('chev-' + id);
    if (!el) return;
    var willExpand = !el.classList.contains('is-expanded');
    el.classList.toggle('is-expanded', willExpand);
    if (willExpand) expandedAccounts.add(id);
    else expandedAccounts.delete(id);
    if (chev) chev.classList.toggle('expanded', willExpand);
  });
}

// ════════════════════════════════════════════
//  MODAL — Add/Edit Subscription
// ════════════════════════════════════════════

function initModal() {
  var overlay = document.getElementById('modal-overlay');
  var closeBtn = document.getElementById('modal-close');
  var cancelBtn = document.getElementById('modal-cancel');
  var saveBtn = document.getElementById('modal-save');

  // Open modal buttons
  document.getElementById('add-sub-btn').addEventListener('click', function() { openModal(); });
  document.getElementById('add-sub-btn-2').addEventListener('click', function() { openModal(); });

  closeBtn.addEventListener('click', closeModal);
  cancelBtn.addEventListener('click', closeModal);
  overlay.addEventListener('click', function(e) {
    if (e.target === overlay) closeModal();
  });

  saveBtn.addEventListener('click', handleSave);

  // Preset autofill
  document.getElementById('f-platform').addEventListener('input', function() {
    var val = this.value;
    for (var i = 0; i < presetsData.length; i++) {
      if (presetsData[i].platform === val) {
        fillFromPreset(presetsData[i]);
        break;
      }
    }
  });

  // Subscription card actions (delegation)
  document.getElementById('subs-grid').addEventListener('click', function(e) {
    var editBtn = e.target.closest('[data-edit-id]');
    if (editBtn) {
      var id = parseInt(editBtn.getAttribute('data-edit-id'));
      openEditModal(id);
      return;
    }
    var deleteBtn = e.target.closest('[data-delete-id]');
    if (deleteBtn) {
      var deleteId = parseInt(deleteBtn.getAttribute('data-delete-id'));
      var deleteName = deleteBtn.getAttribute('data-delete-name');
      openDeleteConfirm(deleteId, deleteName);
    }
  });

  // Delete confirmation
  document.getElementById('delete-close').addEventListener('click', closeDelete);
  document.getElementById('delete-cancel').addEventListener('click', closeDelete);
  document.getElementById('delete-overlay').addEventListener('click', function(e) {
    if (e.target.id === 'delete-overlay') closeDelete();
  });

  // Filters
  document.getElementById('filter-status').addEventListener('change', loadSubscriptions);
  document.getElementById('filter-category').addEventListener('change', loadSubscriptions);
}

function openModal(sub) {
  var overlay = document.getElementById('modal-overlay');
  var title = document.getElementById('modal-title');

  if (sub) {
    title.textContent = 'Edit Subscription';
    document.getElementById('f-id').value = sub.id || '';
    document.getElementById('f-platform').value = sub.platform || '';
    document.getElementById('f-category').value = sub.category || 'other';
    document.getElementById('f-status').value = sub.status || 'active';
    document.getElementById('f-email').value = sub.email || '';
    document.getElementById('f-plan').value = sub.planName || '';
    document.getElementById('f-cost').value = sub.costAmount || '';
    document.getElementById('f-currency').value = sub.costCurrency || 'USD';
    document.getElementById('f-cycle').value = sub.billingCycle || 'monthly';
    document.getElementById('f-token-limit').value = sub.tokenLimit || '';
    document.getElementById('f-credit-limit').value = sub.creditLimit || '';
    document.getElementById('f-request-limit').value = sub.requestLimit || '';
    document.getElementById('f-limit-period').value = sub.limitPeriod || 'monthly';
    document.getElementById('f-renewal').value = sub.nextRenewal || '';
    document.getElementById('f-trial-ends').value = sub.trialEndsAt || '';
    document.getElementById('f-url').value = sub.url || '';
    document.getElementById('f-notes').value = sub.notes || '';
    document.getElementById('f-status-page-url').value = sub.statusPageUrl || '';
    document.getElementById('f-auto-tracked').value = sub.autoTracked ? '1' : '0';
    document.getElementById('f-account-id').value = sub.accountId || '0';
  } else {
    title.textContent = 'Add Subscription';
    document.getElementById('sub-modal').querySelectorAll('input, select, textarea').forEach(function(el) {
      if (el.type === 'hidden') { el.value = ''; return; }
      if (el.tagName === 'SELECT') { el.selectedIndex = 0; return; }
      el.value = '';
    });
    document.getElementById('f-currency').value = 'USD';
    document.getElementById('f-cycle').value = 'monthly';
    document.getElementById('f-category').value = 'coding';
    document.getElementById('f-limit-period').value = 'monthly';
  }

  overlay.hidden = false;
  document.getElementById('f-platform').focus();
}

function closeModal() {
  document.getElementById('modal-overlay').hidden = true;
}

function fillFromPreset(preset) {
  document.getElementById('f-category').value = preset.category || 'other';
  document.getElementById('f-cost').value = preset.costAmount || '';
  document.getElementById('f-cycle').value = preset.billingCycle || 'monthly';
  document.getElementById('f-token-limit').value = preset.tokenLimit || '';
  document.getElementById('f-credit-limit').value = preset.creditLimit || '';
  document.getElementById('f-request-limit').value = preset.requestLimit || '';
  document.getElementById('f-limit-period').value = preset.limitPeriod || 'monthly';
  document.getElementById('f-url').value = preset.url || '';
  document.getElementById('f-notes').value = preset.notes || '';
  document.getElementById('f-status-page-url').value = preset.statusPageUrl || '';
}

function openEditModal(id) {
  fetch('/api/subscriptions/' + id).then(function(res) {
    return res.json();
  }).then(function(sub) {
    openModal(sub);
  }).catch(function(err) {
    showToast('❌ ' + err.message, 'error');
  });
}

function handleSave() {
  var id = document.getElementById('f-id').value;
  var sub = {
    platform: document.getElementById('f-platform').value.trim(),
    category: document.getElementById('f-category').value,
    status: document.getElementById('f-status').value,
    email: document.getElementById('f-email').value.trim(),
    planName: document.getElementById('f-plan').value.trim(),
    costAmount: parseFloat(document.getElementById('f-cost').value) || 0,
    costCurrency: document.getElementById('f-currency').value,
    billingCycle: document.getElementById('f-cycle').value,
    tokenLimit: parseInt(document.getElementById('f-token-limit').value) || 0,
    creditLimit: parseInt(document.getElementById('f-credit-limit').value) || 0,
    requestLimit: parseInt(document.getElementById('f-request-limit').value) || 0,
    limitPeriod: document.getElementById('f-limit-period').value,
    nextRenewal: document.getElementById('f-renewal').value,
    trialEndsAt: document.getElementById('f-trial-ends').value,
    url: document.getElementById('f-url').value.trim(),
    notes: document.getElementById('f-notes').value.trim(),
    statusPageUrl: document.getElementById('f-status-page-url').value,
    autoTracked: document.getElementById('f-auto-tracked').value === '1',
    accountId: parseInt(document.getElementById('f-account-id').value) || 0,
  };

  if (!sub.platform) {
    showToast('❌ Platform name is required', 'error');
    return;
  }

  var saveBtn = document.getElementById('modal-save');
  saveBtn.disabled = true;
  saveBtn.textContent = 'Saving...';

  var promise = id
    ? updateSubscription(parseInt(id), sub)
    : createSubscription(sub);

  promise.then(function(data) {
    showToast('✅ ' + (id ? 'Updated' : 'Created') + ': ' + sub.platform, 'success');
    closeModal();
    loadSubscriptions();
  }).catch(function(err) {
    showToast('❌ ' + err.message, 'error');
  }).finally(function() {
    saveBtn.disabled = false;
    saveBtn.textContent = 'Save Subscription';
  });
}

// ════════════════════════════════════════════
//  DELETE CONFIRMATION
// ════════════════════════════════════════════

var pendingDeleteId = null;

function openDeleteConfirm(id, name) {
  pendingDeleteId = id;
  document.getElementById('delete-name').textContent = name;
  document.getElementById('delete-overlay').hidden = false;

  document.getElementById('delete-confirm').onclick = function() {
    deleteSubscription(pendingDeleteId).then(function() {
      showToast('✅ Deleted: ' + name, 'success');
      closeDelete();
      loadSubscriptions();
    }).catch(function(err) {
      showToast('❌ ' + err.message, 'error');
    });
  };
}

function closeDelete() {
  document.getElementById('delete-overlay').hidden = true;
  pendingDeleteId = null;
}

// ════════════════════════════════════════════
//  SNAP HANDLER
// ════════════════════════════════════════════

var snapInProgress = false;

// H3: Split-button snap — source-aware snapping
var snapDefault = localStorage.getItem('niyantra_snap_default') || 'antigravity';

function initSnapDropdown() {
  var caret = document.getElementById('snap-caret');
  var dropdown = document.getElementById('snap-dropdown');
  if (!caret || !dropdown) return;

  // Toggle dropdown
  caret.addEventListener('click', function(e) {
    e.stopPropagation();
    dropdown.classList.toggle('open');
  });

  // Close on outside click
  document.addEventListener('click', function() {
    dropdown.classList.remove('open');
  });

  // Option clicks
  dropdown.querySelectorAll('.snap-option').forEach(function(opt) {
    opt.addEventListener('click', function(e) {
      e.stopPropagation();
      var source = opt.dataset.source;
      dropdown.classList.remove('open');
      if (source === 'all') {
        snapSource('all');
      } else {
        // Set as new default + snap it
        snapDefault = source;
        localStorage.setItem('niyantra_snap_default', source);
        updateSnapDropdownIndicators();
        snapSource(source);
      }
    });
  });

  updateSnapDropdownIndicators();
}

function updateSnapDropdownIndicators() {
  var dropdown = document.getElementById('snap-dropdown');
  if (!dropdown) return;
  dropdown.querySelectorAll('.snap-option').forEach(function(opt) {
    if (opt.dataset.source === 'all') return; // divider option
    var isActive = opt.dataset.source === snapDefault;
    opt.textContent = (isActive ? '◉ ' : '○ ') + opt.textContent.replace(/^[◉○] /, '');
    opt.classList.toggle('active', isActive);
  });
}

function handleSnap() {
  snapSource(snapDefault);
}

function snapSource(source) {
  var btn = document.getElementById('snap-btn');
  if (!btn || btn.disabled || snapInProgress) return;

  snapInProgress = true;
  btn.disabled = true;
  btn.classList.add('snapping');
  var orig = btn.innerHTML;
  btn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/></svg> Capturing...';

  var promises = [];

  if (source === 'antigravity' || source === 'all') {
    promises.push(
      triggerSnap().then(function(data) {
        return { source: 'Antigravity', data: data, label: data.email || 'Antigravity' };
      }).catch(function(err) {
        return { source: 'Antigravity', error: err.message };
      })
    );
  }

  if (source === 'codex' || source === 'all') {
    promises.push(
      fetch('/api/codex/snap', { method: 'POST' }).then(function(r) { return r.json(); })
      .then(function(d) {
        var label = d.plan ? ('Codex · ' + d.plan) : 'Codex';
        return { source: 'Codex', data: d, label: label };
      })
      .catch(function() { return { source: 'Codex', error: 'capture failed' }; })
    );
  }

  if (promises.length === 0) {
    btn.innerHTML = orig;
    btn.disabled = false;
    snapInProgress = false;
    showToast('No snap source selected', 'warning');
    return;
  }

  Promise.all(promises).then(function(results) {
    var msgs = [];
    var antigravityData = null;
    for (var i = 0; i < results.length; i++) {
      var r = results[i];
      if (r.error) {
        msgs.push('❌ ' + r.source + ': ' + r.error);
      } else {
        msgs.push('✅ ' + r.label);
        if (r.source === 'Antigravity') antigravityData = r.data;
      }
    }
    showToast(msgs.join(' · '), msgs.some(function(m) { return m.startsWith('❌'); }) ? 'warning' : 'success');
    if (antigravityData) {
      renderAccounts(antigravityData);
      updateTimestamp();
    }
  }).finally(function() {
    btn.innerHTML = orig;
    btn.disabled = false;
    btn.classList.remove('snapping');
    snapInProgress = false;
  });
}

// ════════════════════════════════════════════
//  HELPERS
// ════════════════════════════════════════════

function formatSeconds(seconds) {
  seconds = Math.floor(seconds);
  if (seconds <= 0) return 'now';
  var h = Math.floor(seconds / 3600);
  var m = Math.floor((seconds % 3600) / 60);
  if (h >= 24) return Math.floor(h / 24) + 'd ' + (h % 24) + 'h';
  if (h > 0) return h + 'h ' + m + 'm';
  if (m === 0) return '<1m';
  return m + 'm';
}

function formatCredits(n) {
  if (n >= 1000) return (n / 1000).toFixed(n % 1000 === 0 ? 0 : 1) + 'k';
  return Math.round(n).toString();
}

function formatNumber(n) {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
  if (n >= 1000) return (n / 1000).toFixed(n % 1000 === 0 ? 0 : 1) + 'k';
  return n.toString();
}

function currencySymbol(code) {
  var map = { USD: '$', EUR: '€', GBP: '£', INR: '₹', CAD: 'C$', AUD: 'A$' };
  return map[code] || code + ' ';
}

function esc(s) {
  if (!s) return '';
  var d = document.createElement('div');
  d.textContent = s;
  // Also escape quotes for safe use in HTML attributes (M11)
  return d.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function showToast(msg, type) {
  var el = document.getElementById('toast');
  if (!el) return;
  el.textContent = msg;
  el.className = 'toast ' + type + ' visible';
  el.hidden = false;
  setTimeout(function() {
    el.classList.remove('visible');
    setTimeout(function() { el.hidden = true; }, 300);
  }, 3000);
}

// H2: Track last update time for relative display
var lastUpdateTime = null;
function updateTimestamp() {
  lastUpdateTime = new Date();
  refreshTimestampDisplay();
}
function refreshTimestampDisplay() {
  var el = document.getElementById('last-updated');
  if (!el || !lastUpdateTime) return;
  var sec = Math.floor((new Date() - lastUpdateTime) / 1000);
  var label;
  if (sec < 10) label = 'just now';
  else if (sec < 60) label = sec + 's ago';
  else if (sec < 3600) label = Math.floor(sec / 60) + 'm ago';
  else label = Math.floor(sec / 3600) + 'h ago';
  el.textContent = 'Updated ' + label;
  el.title = lastUpdateTime.toLocaleTimeString(); // absolute on hover
}

// ════════════════════════════════════════════
//  CHART — Quota History
// ════════════════════════════════════════════

var historyChart = null;

// M2: Update chart colors in-place on theme toggle (avoids destroy+rebuild flash)
function updateChartTheme(theme) {
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

function loadHistoryChart() {
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

function renderHistoryChart(snapshots) {
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
    datasets.push({
      label: groupNames[key] || key,
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

function populateChartAccountSelect(data) {
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
//  BUDGET THRESHOLD
// ════════════════════════════════════════════

// Server config cache (loaded from /api/config)
var serverConfig = {};

function getBudget() {
  return parseFloat(serverConfig['budget_monthly'] || '0');
}

function setBudget(amount) {
  serverConfig['budget_monthly'] = amount.toString();
  updateConfig('budget_monthly', amount.toString());
}

function getCurrency() {
  return serverConfig['currency'] || 'USD';
}

function updateConfig(key, value) {
  return fetch('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key: key, value: value })
  }).then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.config) {
      data.config.forEach(function(c) { serverConfig[c.key] = c.value; });
    }
  }).catch(function(err) { console.error('Config update failed:', err); });
}

function loadConfig() {
  return fetch('/api/config').then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.config) {
      data.config.forEach(function(c) { serverConfig[c.key] = c.value; });
    }
    return serverConfig;
  });
}

function initBudget() {
  document.getElementById('budget-close').addEventListener('click', closeBudget);
  document.getElementById('budget-cancel').addEventListener('click', closeBudget);
  document.getElementById('budget-overlay').addEventListener('click', function(e) {
    if (e.target.id === 'budget-overlay') closeBudget();
  });
  document.getElementById('budget-save').addEventListener('click', function() {
    var val = parseFloat(document.getElementById('f-budget').value) || 0;
    setBudget(val);
    closeBudget();
    showToast('✅ Budget set to $' + val.toFixed(0) + '/mo', 'success');
    // Refresh overview if visible
    var overviewPanel = document.getElementById('panel-overview');
    if (overviewPanel && overviewPanel.classList.contains('active')) loadOverview();
  });
}

function openBudgetModal() {
  document.getElementById('f-budget').value = getBudget() || '';
  document.getElementById('budget-overlay').hidden = false;
}

function closeBudget() {
  document.getElementById('budget-overlay').hidden = true;
}

function renderBudgetAlert(totalMonthly) {
  var budget = getBudget();
  if (budget <= 0) return '';

  var pct = Math.round((totalMonthly / budget) * 100);
  var cls, icon, msg;

  if (pct >= 100) {
    cls = 'danger';
    icon = '🚨';
    msg = 'Over budget! Spending $' + totalMonthly.toFixed(2) + ' of $' + budget.toFixed(0) + '/mo (' + pct + '%)';
  } else if (pct >= 80) {
    cls = 'warning';
    icon = '⚠️';
    msg = 'Approaching budget: $' + totalMonthly.toFixed(2) + ' of $' + budget.toFixed(0) + '/mo (' + pct + '%)';
  } else {
    cls = 'ok';
    icon = '✅';
    msg = 'Within budget: $' + totalMonthly.toFixed(2) + ' of $' + budget.toFixed(0) + '/mo (' + pct + '%)';
  }

  return '<div class="budget-alert ' + cls + '">' +
    '<span class="budget-icon">' + icon + '</span>' +
    '<span class="budget-msg">' + msg + '</span>' +
    '<button class="budget-btn" onclick="openBudgetModal()">Edit</button>' +
    '</div>';
}

// ════════════════════════════════════════════
//  INSIGHTS — Smart analysis chips
// ════════════════════════════════════════════

function generateInsights(stats, renewals, subs) {
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

function renderInsightChips(chips) {
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

// ════════════════════════════════════════════
//  RENDER — Overview Tab (with budget + insights)
// ════════════════════════════════════════════

function loadOverview() {
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

function renderOverviewEnhanced(data, subs, usageData) {
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

  // ── Quick Links — deduplicated (1 per unique URL) ──
  var linksHTML = '';
  if (links.length > 0) {
    var seenUrls = {};
    var uniqueLinks = [];
    for (var l = 0; l < links.length; l++) {
      if (!seenUrls[links[l].url]) {
        seenUrls[links[l].url] = true;
        uniqueLinks.push(links[l]);
      }
    }
    // Only show if there are genuinely different links (not 15 identical ones)
    if (uniqueLinks.length > 1 || (uniqueLinks.length === 1 && uniqueLinks[0].platform !== 'Antigravity')) {
      linksHTML = '<div class="overview-card full-width"><h3>Quick Links</h3>' +
        '<div class="quick-links-grid">';
      for (var ul = 0; ul < uniqueLinks.length; ul++) {
        linksHTML += '<a class="quick-link" href="' + esc(uniqueLinks[ul].url) + '" target="_blank" rel="noopener">' +
          '🔗 ' + esc(uniqueLinks[ul].platform) + '</a>';
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

  // P1: Content order — advisor first (most actionable), then budget, spend, insights
  el.innerHTML = advisorHTML + forecastHTML + insightsHTML + claudeHTML + spendHTML + calendarHTML + linksHTML + exportHTML;

  // Async load Claude Code data
  if (serverConfig['claude_bridge'] === 'true') {
    loadClaudeCardData();
  }

  // Async load advisor card
  loadAdvisorCard();

  // Render calendar with renewal data (only if container exists)
  if (renewals.length > 0) {
    renderRenewalCalendar(renewals, subs);
  }

  // Phase 11: Codex card + Sessions timeline (async)
  renderCodexCard(el);
  renderSessionsTimeline(el);
}

// ════════════════════════════════════════════
//  SETTINGS
// ════════════════════════════════════════════

function initSettings() {
  var themeEl = document.getElementById('s-theme');

  // Theme stays in localStorage (visual-only)
  var savedTheme = localStorage.getItem('niyantra-theme') || 'dark';
  themeEl.value = savedTheme;

  themeEl.addEventListener('change', function() {
    var val = themeEl.value;
    if (val === 'system') {
      localStorage.removeItem('niyantra-theme');
      var prefer = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', prefer);
    } else {
      localStorage.setItem('niyantra-theme', val);
      document.documentElement.setAttribute('data-theme', val);
    }
    // M2: Update chart theme in-place instead of destroy+rebuild
    var applied = document.documentElement.getAttribute('data-theme');
    updateChartTheme(applied);
  });

  // Load server config and populate settings UI
  loadConfig().then(function(cfg) {
    // Migrate localStorage budget/currency to server config (one-time)
    migrateLocalStorage(cfg);

    // Populate fields from server config
    var budgetEl = document.getElementById('s-budget');
    var currencyEl = document.getElementById('s-currency');
    var autoCaptureEl = document.getElementById('s-auto-capture');
    var autoLinkEl = document.getElementById('s-auto-link');
    var pollEl = document.getElementById('s-poll-interval');
    var retentionEl = document.getElementById('s-retention');

    budgetEl.value = parseFloat(cfg['budget_monthly'] || '0') || '';
    currencyEl.value = cfg['currency'] || 'USD';
    autoCaptureEl.checked = cfg['auto_capture'] === 'true';
    autoLinkEl.checked = cfg['auto_link_subs'] !== 'false';
    pollEl.value = cfg['poll_interval'] || '300';
    retentionEl.value = cfg['retention_days'] || '365';

    // Show/hide poll interval based on auto_capture
    document.getElementById('poll-interval-row').style.display =
      autoCaptureEl.checked ? '' : 'none';

    // Auto-save handlers
    budgetEl.addEventListener('change', function() {
      var val = parseFloat(budgetEl.value) || 0;
      setBudget(val);
      if (val > 0) showToast('✅ Budget: $' + val.toFixed(0) + '/mo', 'success');
    });

    currencyEl.addEventListener('change', function() {
      updateConfig('currency', currencyEl.value);
      showToast('✅ Currency: ' + currencyEl.value, 'success');
    });

    autoCaptureEl.addEventListener('change', function() {
      var val = autoCaptureEl.checked ? 'true' : 'false';
      updateConfig('auto_capture', val).then(function() {
        loadMode();
        showToast(autoCaptureEl.checked ? '🟢 Auto-capture started' : '⏸️ Auto-capture stopped', 'success');
      });
      document.getElementById('poll-interval-row').style.display =
        autoCaptureEl.checked ? '' : 'none';
    });

    autoLinkEl.addEventListener('change', function() {
      updateConfig('auto_link_subs', autoLinkEl.checked ? 'true' : 'false');
    });

    pollEl.addEventListener('change', function() {
      var v = parseInt(pollEl.value);
      if (v >= 30 && v <= 3600) updateConfig('poll_interval', v.toString());
    });

    retentionEl.addEventListener('change', function() {
      var v = parseInt(retentionEl.value);
      if (v >= 30 && v <= 3650) updateConfig('retention_days', v.toString());
    });

    // ── Phase 9: Claude Code Bridge ──
    var claudeBridgeEl = document.getElementById('s-claude-bridge');
    if (claudeBridgeEl) {
      claudeBridgeEl.checked = cfg['claude_bridge'] === 'true';
      claudeBridgeEl.addEventListener('change', function() {
        var val = claudeBridgeEl.checked ? 'true' : 'false';
        updateConfig('claude_bridge', val).then(function() {
          showToast(claudeBridgeEl.checked ? '🔗 Claude Code bridge enabled' : '🔗 Bridge disabled', 'success');
          loadClaudeBridgeStatus();
        });
      });
      loadClaudeBridgeStatus();
    }

    // ── Phase 9: Notifications ──
    var notifyEl = document.getElementById('s-notify-enabled');
    var thresholdEl = document.getElementById('s-notify-threshold');
    var thresholdRow = document.getElementById('notify-threshold-row');
    var testRow = document.getElementById('notify-test-row');
    if (notifyEl) {
      notifyEl.checked = cfg['notify_enabled'] === 'true';
      thresholdEl.value = cfg['notify_threshold'] || '10';
      thresholdRow.style.display = notifyEl.checked ? '' : 'none';
      testRow.style.display = notifyEl.checked ? '' : 'none';

      notifyEl.addEventListener('change', function() {
        var val = notifyEl.checked ? 'true' : 'false';
        updateConfig('notify_enabled', val).then(function() {
          showToast(notifyEl.checked ? '🔔 Notifications enabled' : '🔕 Notifications disabled', 'success');
        });
        thresholdRow.style.display = notifyEl.checked ? '' : 'none';
        testRow.style.display = notifyEl.checked ? '' : 'none';
      });

      thresholdEl.addEventListener('change', function() {
        var v = parseInt(thresholdEl.value);
        if (v >= 5 && v <= 50) {
          updateConfig('notify_threshold', v.toString());
          showToast('🔔 Threshold: ' + v + '%', 'success');
        }
      });

      document.getElementById('notify-test-btn').addEventListener('click', function() {
        fetch('/api/notify/test', { method: 'POST' })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) showToast('❌ ' + data.error, 'error');
            else showToast('🔔 Test notification sent!', 'success');
          })
          .catch(function() { showToast('❌ Failed to send test', 'error'); });
      });
    }

    // ── Phase 11: Codex Capture Toggle ──
    var codexCaptureEl = document.getElementById('s-codex-capture');
    if (codexCaptureEl) {
      codexCaptureEl.checked = cfg['codex_capture'] === 'true';
      codexCaptureEl.addEventListener('change', function() {
        var val = codexCaptureEl.checked ? 'true' : 'false';
        updateConfig('codex_capture', val).then(function() {
          showToast(codexCaptureEl.checked ? '🤖 Codex capture enabled' : '🤖 Codex capture disabled', 'success');
          loadCodexSettingsStatus();
          // T1: Refresh data sources list to reflect new enabled state
          loadDataSources();
        });
      });
      loadCodexSettingsStatus();
    }

    // ── Phase 11: JSON Import ──
    var importBtn = document.getElementById('import-json-btn');
    var importFile = document.getElementById('import-file');
    if (importBtn && importFile) {
      importBtn.addEventListener('click', function() { importFile.click(); });
      importFile.addEventListener('change', function() {
        if (!importFile.files || !importFile.files[0]) return;
        var file = importFile.files[0];
        showToast('📥 Importing ' + file.name + '...', 'info');
        var reader = new FileReader();
        reader.onload = function(e) {
          fetch('/api/import/json', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: e.target.result
          })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) {
              showToast('❌ Import failed: ' + data.error, 'error');
              return;
            }
            var msg = '✅ Imported: ' + (data.accountsCreated || 0) + ' accounts, ' +
                      (data.subsCreated || 0) + ' subs, ' +
                      (data.snapshotsImported || 0) + ' snapshots';
            showToast(msg, 'success');
            var resultEl = document.getElementById('import-result');
            if (resultEl) {
              resultEl.style.display = '';
              resultEl.innerHTML = '<span style="color:var(--accent)">' + msg + '</span>' +
                (data.accountsSkipped ? '<br>Accounts skipped (existing): ' + data.accountsSkipped : '') +
                (data.subsSkipped ? '<br>Subs skipped (existing): ' + data.subsSkipped : '') +
                (data.snapshotsDuped ? '<br>Snapshots deduped: ' + data.snapshotsDuped : '') +
                (data.errors && data.errors.length ? '<br>⚠️ Errors: ' + data.errors.length : '');
            }
            // Refresh data
            fetchStatus().then(renderAccounts);
            loadSubscriptions();
          })
          .catch(function() { showToast('❌ Import failed', 'error'); });
        };
        reader.readAsText(file);
        importFile.value = ''; // reset for re-import
      });
    }
  });

  // Load mode badge
  loadMode();

  // Load data sources
  loadDataSources();

  // Activity log controls
  document.getElementById('activity-refresh').addEventListener('click', loadActivityLog);
  document.getElementById('activity-filter').addEventListener('change', loadActivityLog);
  loadActivityLog();
}

// One-time migration of localStorage budget/currency to server config
function migrateLocalStorage(cfg) {
  var lsBudget = localStorage.getItem('niyantra-budget');
  var lsCurrency = localStorage.getItem('niyantra-currency');

  if (lsBudget && (!cfg['budget_monthly'] || cfg['budget_monthly'] === '0')) {
    updateConfig('budget_monthly', lsBudget);
    serverConfig['budget_monthly'] = lsBudget;
    localStorage.removeItem('niyantra-budget');
  }
  if (lsCurrency && cfg['currency'] === 'USD') {
    updateConfig('currency', lsCurrency);
    serverConfig['currency'] = lsCurrency;
    localStorage.removeItem('niyantra-currency');
  }
}

// ════════════════════════════════════════════
//  MODE BADGE
// ════════════════════════════════════════════

var modeRefreshTimer = null;

function loadMode() {
  fetch('/api/mode').then(function(r) { return r.json(); })
  .then(function(data) {
    var badge = document.getElementById('mode-badge');
    var label = document.getElementById('mode-label');
    if (data.mode === 'auto') {
      badge.className = 'mode-badge mode-auto';
      label.textContent = 'Auto';
    } else {
      badge.className = 'mode-badge mode-manual';
      label.textContent = 'Manual';
    }

    // Show polling status indicator
    var statusEl = document.getElementById('polling-status');
    if (statusEl) {
      if (data.isPolling) {
        var lastMsg = '';
        if (data.lastPoll) {
          lastMsg = 'Last: ' + formatTimeAgo(data.lastPoll);
          if (data.lastPollOK === false) lastMsg += ' (failed)';
        } else {
          lastMsg = 'Starting...';
        }
        statusEl.innerHTML = '<span class="polling-dot"></span> Polling every ' +
          data.pollInterval + 's · ' + lastMsg;
        statusEl.style.display = '';
      } else {
        statusEl.style.display = 'none';
      }
    }

    // Update about section
    var aboutEl = document.getElementById('s-about-info');
    if (aboutEl) {
      var srcCount = (data.sources || []).filter(function(s) { return s.enabled; }).length;
      var schemaV = data.schemaVersion ? ('Schema v' + data.schemaVersion) : 'Schema';
      var presetCount = presetsData.length || 0;
      aboutEl.textContent = schemaV + ' · ' + presetCount + ' presets · Mode: ' +
        (data.mode === 'auto' ? 'Auto' : 'Manual') +
        (data.isPolling ? ' (polling)' : '') +
        ' · ' + srcCount + ' active source' + (srcCount !== 1 ? 's' : '');
    }

    // Auto-refresh mode badge every 30s when auto-capture is active
    if (modeRefreshTimer) { clearInterval(modeRefreshTimer); modeRefreshTimer = null; }
    if (data.isPolling) {
      modeRefreshTimer = setInterval(function() {
        loadMode();
        // Also refresh activity log if settings tab is active
        var activeTab = document.querySelector('.tab-btn.active');
        if (activeTab && activeTab.getAttribute('data-tab') === 'settings') {
          loadActivityLog();
        }
      }, 30000);
    }
  }).catch(function() {});
}

// ════════════════════════════════════════════
//  DATA SOURCES
// ════════════════════════════════════════════

function loadDataSources() {
  fetch('/api/mode').then(function(r) { return r.json(); })
  .then(function(data) {
    var container = document.getElementById('data-sources-list');
    if (!data.sources || data.sources.length === 0) {
      container.innerHTML = '';
      return;
    }
    var html = '<div style="font-size:12px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;margin-top:4px">Data Sources</div>';
    data.sources.forEach(function(src) {
      var meta = src.captureCount + ' captures';
      if (src.lastCapture) {
        meta += ' · Last: ' + formatTimeAgo(src.lastCapture);
      }
      html += '<div class="data-source-item">' +
        '<div class="data-source-info">' +
          '<span class="data-source-name">' + esc(src.name) + '</span>' +
          '<span class="data-source-meta">' + esc(src.sourceType) + ' · ' + meta + '</span>' +
        '</div>' +
        '<span class="data-source-status ' + (src.enabled ? 'enabled' : 'disabled') + '">' +
          (src.enabled ? '● Active' : '○ Disabled') +
        '</span>' +
      '</div>';
    });
    container.innerHTML = html;
  }).catch(function() {});
}

function formatTimeAgo(isoStr) {
  if (!isoStr) return 'never';
  var d = new Date(isoStr);
  var now = new Date();
  var sec = Math.floor((now - d) / 1000);
  if (sec < 60) return 'just now';
  if (sec < 3600) return Math.floor(sec / 60) + 'm ago';
  if (sec < 86400) return Math.floor(sec / 3600) + 'h ago';
  return Math.floor(sec / 86400) + 'd ago';
}

// ════════════════════════════════════════════
//  ACTIVITY LOG
// ════════════════════════════════════════════

function loadActivityLog() {
  var filter = document.getElementById('activity-filter').value;
  var url = '/api/activity?limit=50';
  if (filter) url += '&type=' + filter;

  fetch(url).then(function(r) { return r.json(); })
  .then(function(data) {
    var container = document.getElementById('activity-log');
    if (!data.entries || data.entries.length === 0) {
      container.innerHTML = '<div class="activity-empty">No activity' +
        (filter ? ' for "' + filter + '"' : '') + ' yet</div>';
      return;
    }

    var html = '';
    data.entries.forEach(function(entry) {
      var time = entry.timestamp ? entry.timestamp.replace('T', ' ').substring(5, 16) : '';
      var detail = formatActivityDetail(entry);
      html += '<div class="activity-entry">' +
        '<span class="activity-time">' + time + '</span>' +
        '<span class="activity-type ' + esc(entry.eventType) + '">' +
          esc(entry.eventType.replace(/_/g, ' ')) +
        '</span>' +
        '<span class="activity-detail">' + detail + '</span>' +
      '</div>';
    });
    container.innerHTML = html;
  }).catch(function() {
    document.getElementById('activity-log').innerHTML =
      '<div class="activity-empty">Failed to load activity log</div>';
  });
}

function formatActivityDetail(entry) {
  try {
    var d = JSON.parse(entry.details || '{}');
    switch (entry.eventType) {
      case 'snap':
        return esc(entry.accountEmail || '') +
          (d.method ? ' · ' + d.method : '') +
          (d.source ? ' via ' + d.source : '');
      case 'snap_failed':
        return esc(d.error || 'Unknown error');
      case 'config_change':
        return esc(d.key || '') + ': ' + esc(d.from || '""') + ' → ' + esc(d.to || '""');
      case 'server_start':
        return 'Port ' + (d.port || '?') + ' · ' + esc(d.mode || 'manual') + ' mode';
      case 'sub_created':
      case 'sub_deleted':
        return esc(d.platform || '');
      case 'auto_link':
        return esc(entry.accountEmail || '') + ' → ' + esc(d.platform || '');
      case 'codex_snap':
        // T2: Truncate UUID for readability
        var acctId = entry.accountEmail || '';
        if (acctId.length > 20) acctId = acctId.substring(0, 6) + '..' + acctId.slice(-6);
        return esc(acctId) + (d.plan ? ' (' + esc(d.plan) + ')' : '');
      case 'model_reset':
        return esc(entry.accountEmail || '');
      default:
        return entry.accountEmail ? esc(entry.accountEmail) : '';
    }
  } catch(e) {
    return '';
  }
}

// ════════════════════════════════════════════
//  SEARCH — Subscriptions
// ════════════════════════════════════════════

function initSearch() {
  var searchEl = document.getElementById('search-subs');
  if (!searchEl) return;

  searchEl.addEventListener('input', function() {
    var query = searchEl.value.toLowerCase().trim();
    var cards = document.querySelectorAll('.sub-card');
    var labels = document.querySelectorAll('.sub-category-label');
    cards.forEach(function(card) {
      var text = card.textContent.toLowerCase();
      card.style.display = text.indexOf(query) >= 0 ? '' : 'none';
    });
    // Hide empty category labels
    labels.forEach(function(label) {
      var next = label.nextElementSibling;
      var anyVisible = false;
      while (next && !next.classList.contains('sub-category-label')) {
        if (next.classList.contains('sub-card') && next.style.display !== 'none') {
          anyVisible = true;
        }
        next = next.nextElementSibling;
      }
      label.style.display = anyVisible ? '' : 'none';
    });
  });
}

// ════════════════════════════════════════════
//  KEYBOARD SHORTCUTS
// ════════════════════════════════════════════

function initKeyboardShortcuts() {
  document.addEventListener('keydown', function(e) {
    // Skip if user is typing in an input/textarea/select
    var tag = document.activeElement.tagName;
    if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') {
      if (e.key === 'Escape') {
        document.activeElement.blur();
        closeModal();
        closeDelete();
        closeBudget();
      }
      return;
    }

    // Don't fire shortcuts when modals are open (except Escape)
    var anyModal = !document.getElementById('modal-overlay').hidden ||
                   !document.getElementById('delete-overlay').hidden ||
                   !document.getElementById('budget-overlay').hidden;

    if (e.key === 'Escape') {
      closeModal();
      closeDelete();
      closeBudget();
      return;
    }

    if (anyModal) return;

    switch (e.key) {
      case '1': switchToTab('quotas'); break;
      case '2': switchToTab('subscriptions'); break;
      case '3': switchToTab('overview'); break;
      case '4': switchToTab('settings'); break;
      case 'n': case 'N': openModal(); e.preventDefault(); break;
      case 's': case 'S': handleSnap(); e.preventDefault(); break;
      case '/':
        e.preventDefault();
        switchToTab('subscriptions');
        setTimeout(function() {
          var search = document.getElementById('search-subs');
          if (search) search.focus();
        }, 100);
        break;
    }
  });

  // Ctrl+K / Cmd+K for command palette
  document.addEventListener('keydown', function(e) {
    if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
      e.preventDefault();
      toggleCommandPalette();
    }
  });
}

function switchToTab(tabName) {
  var btns = document.querySelectorAll('.tab-btn');
  btns.forEach(function(b) { b.classList.remove('active'); });
  var target = document.querySelector('.tab-btn[data-tab="' + tabName + '"]');
  if (target) target.classList.add('active');

  document.querySelectorAll('.tab-panel').forEach(function(p) {
    p.classList.remove('active');
  });
  var panel = document.getElementById('panel-' + tabName);
  if (panel) panel.classList.add('active');

  if (tabName === 'subscriptions') loadSubscriptions();
  if (tabName === 'overview') loadOverview();
  if (tabName === 'settings') { loadActivityLog(); loadMode(); loadDataSources(); }
}

// ════════════════════════════════════════════
//  INIT
// ════════════════════════════════════════════

function initQuotas() {
  var qSearch = document.getElementById('quota-search');
  var qStatus = document.getElementById('quota-filter-status');
  if (qSearch) {
    qSearch.addEventListener('input', function() {
      if (latestQuotaData) renderAccounts(latestQuotaData);
    });
  }
  if (qStatus) {
    qStatus.addEventListener('change', function() {
      if (latestQuotaData) renderAccounts(latestQuotaData);
    });
  }

  var qProvider = document.getElementById('quota-filter-provider');
  if (qProvider) {
    qProvider.addEventListener('change', function() {
      if (latestQuotaData) renderAccounts(latestQuotaData);
    });
  }

  // Sort headers are now dynamic â€” use delegation on account-grid
  var gridEl = document.getElementById('account-grid');
  if (gridEl) {
    gridEl.addEventListener('click', function(e) {
      var el = e.target.closest('.sortable');
      if (!el) return;
      var col = el.dataset.sort;
      if (quotaSortState.column === col) {
        quotaSortState.direction = quotaSortState.direction === 'asc' ? 'desc' : 'asc';
      } else {
        quotaSortState.column = col;
        quotaSortState.direction = 'asc';
      }
      if (latestQuotaData) renderAccounts(latestQuotaData);
    });
  }
}

document.addEventListener('DOMContentLoaded', function() {
  initTheme();
  initTabs();
  initQuotas();
  setupToggle();
  initModal();
  initBudget();
  initSettings();
  initSearch();
  initKeyboardShortcuts();

  document.getElementById('snap-btn').addEventListener('click', handleSnap);
  initSnapDropdown();

  // Chart controls
  document.getElementById('chart-account').addEventListener('change', loadHistoryChart);
  document.getElementById('chart-range').addEventListener('change', loadHistoryChart);

  // Load quotas and usage intelligence
  Promise.all([fetchStatus(), fetchUsage()]).then(function(results) {
    var data = results[0];
    renderAccounts(data);
    updateTimestamp();
    populateChartAccountSelect(data);
    loadHistoryChart();
  }).catch(function(err) {
    console.error('Failed to load status:', err);
  });

  // Load presets for the datalist
  fetchPresets().then(function(data) {
    presetsData = data.presets || [];
    var list = document.getElementById('preset-list');
    for (var i = 0; i < presetsData.length; i++) {
      var opt = document.createElement('option');
      opt.value = presetsData[i].platform;
      list.appendChild(opt);
    }
  });

  // Load mode badge in header (manual/auto status)
  loadMode();

  // Init command palette
  initCommandPalette();

  // Phase 10: Load system alerts
  loadSystemAlerts();

  // Auto-capture polling is handled server-side by the agent.
  // Manual data refreshes on snap or page reload.

  // H2: Refresh relative timestamp every 30s
  setInterval(refreshTimestampDisplay, 30000);
});

// ════════════════════════════════════════════
//  COMMAND PALETTE (Phase 9)
// ════════════════════════════════════════════

var PALETTE_COMMANDS = [
  { name: 'Snap Now',            key: 'S',    icon: '📸', action: function() { handleSnap(); } },
  { name: 'Show Quotas',         key: '1',    icon: '📊', action: function() { switchToTab('quotas'); } },
  { name: 'Show Subscriptions',  key: '2',    icon: '💳', action: function() { switchToTab('subscriptions'); } },
  { name: 'Show Overview',       key: '3',    icon: '📋', action: function() { switchToTab('overview'); } },
  { name: 'Show Settings',       key: '4',    icon: '⚙️', action: function() { switchToTab('settings'); } },
  { name: 'New Subscription',    key: 'N',    icon: '➕', action: function() { openModal(); } },
  { name: 'Toggle Auto-Capture',              icon: '🔄', action: function() {
    var el = document.getElementById('s-auto-capture');
    if (el) { el.checked = !el.checked; el.dispatchEvent(new Event('change')); }
  }},
  { name: 'Export CSV',                       icon: '📥', action: function() { window.location.href = '/api/export/csv'; } },
  { name: 'Export JSON',                      icon: '📦', action: function() { window.location.href = '/api/export/json'; } },
  { name: 'Download Backup',                  icon: '💾', action: function() { window.location.href = '/api/backup'; } },
  { name: 'Search Subscriptions', key: '/',   icon: '🔍', action: function() {
    switchToTab('subscriptions');
    setTimeout(function() { var s = document.getElementById('search-subs'); if (s) s.focus(); }, 100);
  }},
  { name: 'Set Budget',                       icon: '💰', action: function() { openBudgetModal(); } },
  { name: 'Toggle Theme',                     icon: '🌓', action: function() {
    var cur = document.documentElement.getAttribute('data-theme');
    var next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('niyantra-theme', next);
    var themeEl = document.getElementById('s-theme');
    if (themeEl) themeEl.value = next;
    updateChartTheme(next);
  }},
  { name: 'Codex Snap',                        icon: '🤖', action: function() { handleCodexSnap(); } },
  { name: 'Import JSON',                       icon: '📥', action: function() {
    var f = document.getElementById('import-file');
    if (f) f.click();
  }},
];

var paletteSelectedIndex = 0;
var paletteFilteredCommands = PALETTE_COMMANDS;

function initCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  var search = document.getElementById('command-palette-search');
  if (!overlay || !search) return;

  overlay.addEventListener('click', function(e) {
    if (e.target === overlay) closeCommandPalette();
  });

  search.addEventListener('input', function() {
    var query = search.value.toLowerCase().trim();
    paletteFilteredCommands = PALETTE_COMMANDS.filter(function(cmd) {
      return cmd.name.toLowerCase().indexOf(query) >= 0;
    });
    paletteSelectedIndex = 0;
    renderPaletteList();
  });

  search.addEventListener('keydown', function(e) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      paletteSelectedIndex = Math.min(paletteSelectedIndex + 1, paletteFilteredCommands.length - 1);
      renderPaletteList();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      paletteSelectedIndex = Math.max(paletteSelectedIndex - 1, 0);
      renderPaletteList();
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (paletteFilteredCommands[paletteSelectedIndex]) {
        closeCommandPalette();
        paletteFilteredCommands[paletteSelectedIndex].action();
      }
    } else if (e.key === 'Escape') {
      closeCommandPalette();
    }
  });
}

function toggleCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  if (overlay.hidden) {
    openCommandPalette();
  } else {
    closeCommandPalette();
  }
}

function openCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  var search = document.getElementById('command-palette-search');
  overlay.hidden = false;
  search.value = '';
  paletteFilteredCommands = PALETTE_COMMANDS;
  paletteSelectedIndex = 0;
  renderPaletteList();
  setTimeout(function() { search.focus(); }, 50);
}

function closeCommandPalette() {
  document.getElementById('command-palette-overlay').hidden = true;
}

function renderPaletteList() {
  var list = document.getElementById('command-palette-list');
  if (paletteFilteredCommands.length === 0) {
    list.innerHTML = '<div class="command-palette-empty">No matching commands</div>';
    return;
  }
  var html = '';
  for (var i = 0; i < paletteFilteredCommands.length; i++) {
    var cmd = paletteFilteredCommands[i];
    var sel = i === paletteSelectedIndex ? ' selected' : '';
    html += '<div class="command-palette-item' + sel + '" data-idx="' + i + '">' +
      '<span class="cp-icon">' + cmd.icon + '</span>' +
      '<span class="cp-name">' + esc(cmd.name) + '</span>' +
      (cmd.key ? '<span class="cp-shortcut">' + cmd.key + '</span>' : '') +
      '</div>';
  }
  list.innerHTML = html;

  // Click handlers
  list.querySelectorAll('.command-palette-item').forEach(function(el) {
    el.addEventListener('click', function() {
      var idx = parseInt(el.getAttribute('data-idx'));
      closeCommandPalette();
      paletteFilteredCommands[idx].action();
    });
  });

  // Scroll selected into view
  var selected = list.querySelector('.selected');
  if (selected) selected.scrollIntoView({ block: 'nearest' });
}

// ════════════════════════════════════════════
//  CLAUDE CODE BRIDGE (Phase 9)
// ════════════════════════════════════════════

function loadClaudeBridgeStatus() {
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

function renderClaudeCodeCard() {
  return '<div class="claude-card" id="claude-code-card">' +
    '<h3>🔗 Claude Code</h3>' +
    '<div id="claude-card-body"><div class="empty-hint">Loading...</div></div>' +
    '</div>';
}

function loadClaudeCardData() {
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

function meterColor(pct) {
  if (pct >= 80) return 'var(--red)';
  if (pct >= 50) return 'var(--amber)';
  return 'var(--green)';
}

function formatResetTime(isoStr) {
  if (!isoStr) return '';
  var d = new Date(isoStr);
  var now = new Date();
  var diffMs = d - now;
  if (diffMs <= 0) return 'now';
  var hours = Math.floor(diffMs / 3600000);
  var mins = Math.floor((diffMs % 3600000) / 60000);
  if (hours >= 24) return Math.floor(hours / 24) + 'd';
  if (hours > 0) return hours + 'h';
  return mins + 'm';
}

// ════════════════════════════════════════════
//  Phase 10: SYSTEM ALERTS
// ════════════════════════════════════════════

function loadSystemAlerts() {
  fetch('/api/alerts').then(function(r) { return r.json(); })
  .then(function(data) {
    var container = document.getElementById('alert-banner-container');
    if (!container) return;
    var alerts = data.alerts || [];
    if (alerts.length === 0) {
      container.innerHTML = '';
      return;
    }
    var html = '';
    var shown = Math.min(alerts.length, 3);
    for (var i = 0; i < shown; i++) {
      var a = alerts[i];
      var icon = a.severity === 'critical' ? '🚨' : (a.severity === 'warning' ? '⚠️' : 'ℹ️');
      html += '<div class="alert-banner ' + esc(a.severity) + '">' +
        '<span class="alert-banner-icon">' + icon + '</span>' +
        '<div class="alert-banner-content">' +
        '<div class="alert-banner-title">' + esc(a.category) + '</div>' +
        '<div class="alert-banner-msg">' + esc(a.message) + '</div>' +
        '</div>' +
        '<button class="alert-banner-dismiss" onclick="dismissAlert(' + a.id + ')" title="Dismiss">&times;</button>' +
        '</div>';
    }
    if (alerts.length > 3) {
      html += '<div class="alert-more-link" onclick="switchToTab(\'overview\')">' +
        '+ ' + (alerts.length - 3) + ' more alert(s)</div>';
    }
    container.innerHTML = html;
  }).catch(function() {});
}

function dismissAlert(id) {
  fetch('/api/alerts/dismiss', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ id: id })
  }).then(function() {
    loadSystemAlerts();
    showToast('Alert dismissed', 'success');
  }).catch(function() {
    showToast('Failed to dismiss alert', 'error');
  });
}

// ════════════════════════════════════════════
//  Phase 10: SERVER-COMPUTED INSIGHTS
// ════════════════════════════════════════════

function renderServerInsights(insights) {
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
    var icon = iconMap[ins.type] || '💡';
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
var advisorGroupPref = localStorage.getItem('niyantra_advisor_group') || 'claude_gpt';

function loadAdvisorCard() {
  var container = document.getElementById('advisor-card-container');
  if (!container) return;

  // Use latestQuotaData which has per-account group breakdowns
  if (!latestQuotaData || !latestQuotaData.accounts || latestQuotaData.accounts.length < 2) {
    container.innerHTML = '';
    return;
  }

  renderAdvisorWithGroup(container, advisorGroupPref);
}

function renderAdvisorWithGroup(container, groupKey) {
  var accounts = latestQuotaData.accounts;

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
  var actionIcon = best.pct > 20 ? '⚡' : '⏳';
  var actionLabel = best.pct > 20 ? 'SWITCH' : 'WAIT';
  var bestLabel = best.email.split('@')[0] + '@...';

  var html = '<div class="advisor-card">' +
    '<h3>🎯 Switch Advisor</h3>' +
    '<div class="advisor-group-select">' +
    '<label>Optimize for:</label>' +
    '<select id="advisor-group-filter" class="filter-select" style="margin-left:8px;font-size:12px">' +
    '<option value="claude_gpt"' + (groupKey === 'claude_gpt' ? ' selected' : '') + '>Claude + GPT</option>' +
    '<option value="gemini_pro"' + (groupKey === 'gemini_pro' ? ' selected' : '') + '>Gemini Pro</option>' +
    '<option value="gemini_flash"' + (groupKey === 'gemini_flash' ? ' selected' : '') + '>Gemini Flash</option>' +
    '<option value="all"' + (groupKey === 'all' ? ' selected' : '') + '>All Models (avg)</option>' +
    '</select></div>';

  html += '<div class="advisor-action ' + (best.pct > 20 ? 'switch' : 'wait') + '">' +
    actionIcon + ' ' + actionLabel + '</div>' +
    '<div class="advisor-reason">Best: ' + esc(best.email) + ' (' + best.pct + '% ' +
    esc(groupNames[groupKey] || groupKey) + ' remaining)' +
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
      advisorGroupPref = sel.value;
      localStorage.setItem('niyantra_advisor_group', advisorGroupPref);
      renderAdvisorWithGroup(container, advisorGroupPref);
    });
  }

  // Wire up show-all toggle
  var toggleBtn = document.getElementById('advisor-toggle-all');
  if (toggleBtn) {
    toggleBtn.addEventListener('click', function() {
      var extras = container.querySelectorAll('[data-advisor-extra]');
      var showing = toggleBtn.textContent.indexOf('Hide') >= 0;
      extras.forEach(function(el) { el.style.display = showing ? 'none' : ''; });
      toggleBtn.textContent = showing ? 'Show all ' + ranked.length + ' accounts' : 'Hide extras';
    });
  }
}


// ════════════════════════════════════════════
//  Phase 10: RENEWAL CALENDAR
// ════════════════════════════════════════════

var calendarViewDate = new Date();

function renderRenewalCalendar(renewals, subs) {
  var container = document.getElementById('renewal-calendar-container');
  if (!container) return;

  // Build a map of date -> [{ platform, category }]
  var renewalMap = {};
  if (renewals) {
    for (var i = 0; i < renewals.length; i++) {
      var r = renewals[i];
      var dateKey = r.nextRenewal; // "YYYY-MM-DD"
      if (!renewalMap[dateKey]) renewalMap[dateKey] = [];
      // Find category for this platform
      var cat = 'other';
      if (subs) {
        for (var s = 0; s < subs.length; s++) {
          if (subs[s].platform === r.platform && subs[s].category) {
            cat = subs[s].category;
            break;
          }
        }
      }
      renewalMap[dateKey].push({ platform: r.platform, category: cat, daysUntil: r.daysUntil });
    }
  }

  var year = calendarViewDate.getFullYear();
  var month = calendarViewDate.getMonth();
  var today = new Date();
  var todayKey = today.getFullYear() + '-' + String(today.getMonth() + 1).padStart(2, '0') + '-' + String(today.getDate()).padStart(2, '0');

  var monthNames = ['January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December'];
  var dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

  var firstDay = new Date(year, month, 1).getDay();
  var daysInMonth = new Date(year, month + 1, 0).getDate();
  var prevDays = new Date(year, month, 0).getDate();

  var html = '<div class="calendar-container">' +
    '<div class="calendar-header">' +
    '<h3>📅 Renewal Calendar</h3>' +
    '<div class="calendar-nav">' +
    '<button class="calendar-nav-btn" onclick="calendarNav(-1)">‹</button>' +
    '<span class="calendar-month-label">' + monthNames[month] + ' ' + year + '</span>' +
    '<button class="calendar-nav-btn" onclick="calendarNav(1)">›</button>' +
    '</div></div>';

  // Weekday headers
  html += '<div class="calendar-weekdays">';
  for (var d = 0; d < 7; d++) {
    html += '<div class="calendar-weekday">' + dayNames[d] + '</div>';
  }
  html += '</div>';

  // Calendar grid
  html += '<div class="calendar-grid">';

  // Previous month's trailing days
  for (var p = firstDay - 1; p >= 0; p--) {
    html += '<div class="calendar-day other-month"><span class="calendar-day-num">' + (prevDays - p) + '</span></div>';
  }

  // Current month days
  for (var day = 1; day <= daysInMonth; day++) {
    var dateKey = year + '-' + String(month + 1).padStart(2, '0') + '-' + String(day).padStart(2, '0');
    var isToday = dateKey === todayKey;
    var dayClass = isToday ? 'calendar-day today' : 'calendar-day';
    var events = renewalMap[dateKey];

    html += '<div class="' + dayClass + '"';

    // Tooltip
    if (events && events.length > 0) {
      var tooltipText = events.map(function(e) { return e.platform; }).join(', ');
      html += ' title="' + esc(tooltipText) + '"';
    }
    html += '>';

    html += '<span class="calendar-day-num">' + day + '</span>';

    // Renewal pins
    if (events && events.length > 0) {
      html += '<div class="calendar-pins">';
      for (var e = 0; e < Math.min(events.length, 4); e++) {
        html += '<span class="calendar-pin ' + esc(events[e].category) + '"></span>';
      }
      html += '</div>';
    }
    html += '</div>';
  }

  // Fill remaining cells in last week
  var totalCells = firstDay + daysInMonth;
  var remaining = 7 - (totalCells % 7);
  if (remaining < 7) {
    for (var n = 1; n <= remaining; n++) {
      html += '<div class="calendar-day other-month"><span class="calendar-day-num">' + n + '</span></div>';
    }
  }

  html += '</div>';

  // Legend
  var categories = {};
  for (var key in renewalMap) {
    for (var ci = 0; ci < renewalMap[key].length; ci++) {
      categories[renewalMap[key][ci].category] = true;
    }
  }
  var catKeys = Object.keys(categories);
  if (catKeys.length > 0) {
    html += '<div class="calendar-legend">';
    for (var cl = 0; cl < catKeys.length; cl++) {
      html += '<div class="calendar-legend-item">' +
        '<span class="calendar-legend-dot ' + esc(catKeys[cl]) + '"></span>' +
        esc(catKeys[cl]) + '</div>';
    }
    html += '</div>';
  }

  html += '</div>';
  container.innerHTML = html;
}

function calendarNav(delta) {
  calendarViewDate.setMonth(calendarViewDate.getMonth() + delta);
  // Re-render with cached data
  var el = document.getElementById('renewal-calendar-container');
  if (el) {
    // Reload overview to get fresh data
    loadOverview();
  }
}

// ════════════════════════════════════════════
//  PHASE 11: CODEX & SESSIONS
// ════════════════════════════════════════════

// ── Codex Settings Status ──
function loadCodexSettingsStatus() {
  var statusEl = document.getElementById('codex-status-settings');
  if (!statusEl) return;

  fetch('/api/codex/status').then(function(r) { return r.json(); })
  .then(function(data) {
    statusEl.style.display = '';
    if (!data.installed) {
      statusEl.innerHTML = '<span style="color:var(--text-muted)">⚠️ Codex CLI not detected. ' +
        'Install <a href="https://github.com/openai/codex" target="_blank" style="color:var(--accent)">Codex</a> ' +
        'and run <code>codex auth</code> to enable.</span>';
      return;
    }
    var tokenStatus = data.tokenExpired ?
      '<span style="color:var(--warning)">⚠️ Token expired — will auto-refresh on next poll</span>' :
      '<span style="color:var(--success)">✅ Token valid (expires ' + (data.tokenExpiresIn || '?') + ')</span>';
    statusEl.innerHTML =
      '🤖 Codex detected · Account: <strong>' + esc(data.accountId || 'unknown') + '</strong><br>' +
      tokenStatus;
    if (data.snapshot) {
      statusEl.innerHTML += '<br>Latest: <strong>' + data.snapshot.fiveHourPct.toFixed(1) + '%</strong> used (5h) · ' +
        '<span style="color:var(--text-muted)">' + formatTimeAgo(data.snapshot.capturedAt) + '</span>';
    }
  })
  .catch(function() {
    statusEl.style.display = 'none';
  });
}

// ── Codex Manual Snap ──
function handleCodexSnap() {
  showToast('🤖 Capturing Codex snapshot...', 'info');
  fetch('/api/codex/snap', { method: 'POST' })
  .then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.error) {
      showToast('❌ ' + data.error, 'error');
      return;
    }
    showToast('🤖 Codex snapshot captured! Plan: ' + (data.plan || 'unknown'), 'success');
    loadCodexSettingsStatus();
    loadOverview();
  })
  .catch(function() { showToast('❌ Codex snap failed', 'error'); });
}

// ── Codex Status Card (for Overview tab) ──
function renderCodexCard(container) {
  fetch('/api/codex/status').then(function(r) { return r.json(); })
  .then(function(data) {
    if (!data.installed && !data.snapshot) return; // Don't show card if no Codex at all

    var html = '<div class="overview-card codex-card">';
    html += '<div class="card-header"><h3>🤖 Codex / ChatGPT</h3>';
    html += '<button class="btn-add" onclick="handleCodexSnap()" style="padding:4px 10px;font-size:11px">📸 Snap</button>';
    html += '</div>';

    if (!data.installed) {
      html += '<div class="card-body"><span style="color:var(--text-muted)">Codex not detected</span></div>';
    } else if (!data.snapshot) {
      html += '<div class="card-body"><span style="color:var(--text-muted)">No snapshots yet — click Snap to capture</span></div>';
    } else {
      var snap = data.snapshot;
      var fivePct = snap.fiveHourPct || 0;
      var sevenPct = snap.sevenDayPct || null;
      var reviewPct = snap.codeReviewPct || null;
      var statusClass = fivePct >= 95 ? 'critical' : fivePct >= 80 ? 'danger' : fivePct >= 50 ? 'warning' : 'healthy';

      html += '<div class="card-body">';
      // 5-hour quota bar
      html += '<div class="codex-quota">';
      html += '<div class="codex-quota-label">5-Hour Window</div>';
      html += '<div class="quota-bar-track"><div class="quota-bar-fill ' + statusClass + '" style="width:' + Math.min(fivePct, 100) + '%"></div></div>';
      html += '<div class="codex-quota-value ' + statusClass + '">' + fivePct.toFixed(1) + '% used</div>';
      html += '</div>';

      // 7-day quota bar (optional)
      if (sevenPct !== null) {
        var sevenClass = sevenPct >= 95 ? 'critical' : sevenPct >= 80 ? 'danger' : sevenPct >= 50 ? 'warning' : 'healthy';
        html += '<div class="codex-quota">';
        html += '<div class="codex-quota-label">7-Day Window</div>';
        html += '<div class="quota-bar-track"><div class="quota-bar-fill ' + sevenClass + '" style="width:' + Math.min(sevenPct, 100) + '%"></div></div>';
        html += '<div class="codex-quota-value ' + sevenClass + '">' + sevenPct.toFixed(1) + '% used</div>';
        html += '</div>';
      }

      // Code review bar (optional)
      if (reviewPct !== null) {
        var revClass = reviewPct >= 95 ? 'critical' : reviewPct >= 80 ? 'danger' : reviewPct >= 50 ? 'warning' : 'healthy';
        html += '<div class="codex-quota">';
        html += '<div class="codex-quota-label">Code Review</div>';
        html += '<div class="quota-bar-track"><div class="quota-bar-fill ' + revClass + '" style="width:' + Math.min(reviewPct, 100) + '%"></div></div>';
        html += '<div class="codex-quota-value ' + revClass + '">' + reviewPct.toFixed(1) + '% used</div>';
        html += '</div>';
      }

      // Meta info
      html += '<div class="codex-meta">';
      html += '<span>Plan: <strong>' + esc(snap.planType || '—') + '</strong></span>';
      if (snap.creditsBalance !== null && snap.creditsBalance !== undefined) {
        html += '<span>Credits: <strong>' + snap.creditsBalance.toFixed(2) + '</strong></span>';
      }
      html += '<span>Captured: ' + formatTimeAgo(snap.capturedAt) + '</span>';
      html += '<span class="codex-provenance">' + esc(snap.captureMethod) + '/' + esc(snap.captureSource) + '</span>';
      html += '</div>';
      html += '</div>';
    }
    html += '</div>';

    // Insert before the calendar or at end
    var existing = container.querySelector('.codex-card');
    if (existing) {
      existing.outerHTML = html;
    } else {
      container.insertAdjacentHTML('afterbegin', html);
    }
  })
  .catch(function() {}); // Silently fail
}

// ── Sessions Timeline (for Overview tab) ──
function renderSessionsTimeline(container) {
  fetch('/api/sessions?limit=10').then(function(r) { return r.json(); })
  .then(function(data) {
    if (!data.sessions || data.sessions.length === 0) return;

    var html = '<div class="overview-card sessions-card">';
    html += '<div class="card-header"><h3>⏱️ Usage Sessions</h3>';
    html += '<span class="card-count">' + data.count + ' sessions</span>';
    html += '</div>';
    html += '<div class="card-body">';
    html += '<div class="session-timeline">';

    for (var i = 0; i < data.sessions.length; i++) {
      var sess = data.sessions[i];
      var isActive = !sess.endedAt;
      var duration = isActive ?
        formatDurationSec(Math.floor((Date.now() - new Date(sess.startedAt).getTime()) / 1000)) :
        formatDurationSec(sess.durationSec);
      var providerIcon = sess.provider === 'codex' ? '🤖' :
                         sess.provider === 'claude' ? '🔮' : '⚡';

      html += '<div class="session-item' + (isActive ? ' active' : '') + '">';
      html += '<div class="session-dot' + (isActive ? ' pulse' : '') + '"></div>';
      html += '<div class="session-content">';
      html += '<div class="session-top">';
      html += '<span class="session-provider">' + providerIcon + ' ' + esc(sess.provider) + '</span>';
      html += '<span class="session-duration">' + duration + '</span>';
      html += '</div>';
      html += '<div class="session-bottom">';
      html += '<span class="session-time">' + formatTimeAgo(sess.startedAt) + '</span>';
      html += '<span class="session-snaps">' + sess.snapCount + ' snaps</span>';
      if (isActive) html += '<span class="session-active-badge">LIVE</span>';
      html += '</div>';
      html += '</div></div>';
    }

    html += '</div></div></div>';

    // Insert after codex card or at start
    var codexCard = container.querySelector('.codex-card');
    var existing = container.querySelector('.sessions-card');
    if (existing) {
      existing.outerHTML = html;
    } else if (codexCard) {
      codexCard.insertAdjacentHTML('afterend', html);
    } else {
      container.insertAdjacentHTML('afterbegin', html);
    }
  })
  .catch(function() {});
}

function formatDurationSec(sec) {
  if (!sec || sec <= 0) return '0m';
  var h = Math.floor(sec / 3600);
  var m = Math.floor((sec % 3600) / 60);
  if (h > 0) return h + 'h ' + m + 'm';
  return m + 'm';
}

