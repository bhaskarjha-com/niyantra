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

// ════════════════════════════════════════════
//  RENDER — Quotas Tab (existing)
// ════════════════════════════════════════════

function renderAccounts(data) {
  var grid = document.getElementById('account-grid');
  var countBadge = document.getElementById('account-count');
  var snapCount = document.getElementById('snap-count');
  if (!grid) return;

  if (!data.accounts || data.accounts.length === 0) {
    countBadge.textContent = '0 accounts';
    if (snapCount) snapCount.textContent = '';
    grid.innerHTML = '<div class="empty-state">' +
      '<svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.4"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/><path d="M12 2v4M12 18v4M2 12h4M18 12h4"/></svg>' +
      '<p>No accounts tracked yet</p>' +
      '<p class="empty-hint">Click <strong>Snap Now</strong> to capture your first snapshot</p>' +
      '</div>';
    return;
  }

  var count = data.accountCount || data.accounts.length;
  countBadge.textContent = count + ' account' + (count !== 1 ? 's' : '');
  if (snapCount) snapCount.textContent = data.snapshotCount ? (data.snapshotCount + ' snapshots') : '';

  var html = '';
  for (var i = 0; i < data.accounts.length; i++) {
    var acc = data.accounts[i];
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
      groupCells += '<div class="quota-cell">' +
        '<span class="quota-pct ' + cls + '">' + pct + '%</span>' +
        reset + '</div>';
    }

    var allExhausted = true;
    var grps = acc.groups || [];
    for (var ei = 0; ei < grps.length; ei++) {
      if (!grps[ei].isExhausted && grps[ei].remainingPercent > 0) { allExhausted = false; break; }
    }
    var badgeCls = acc.isReady ? 'ready' : 'partial';
    var badgeText = acc.isReady ? 'Ready' : 'Low';
    if (allExhausted) { badgeCls = 'exhausted'; badgeText = 'Empty'; }

    var creditsHTML = '';
    if (acc.promptCredits > 0) {
      creditsHTML = '<span class="credits-badge" title="Prompt credits remaining">✦ ' + formatCredits(acc.promptCredits) + '</span>';
    }

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
        modelRows += '<div class="model-row">' +
          '<div class="model-indicator" style="background:' + color + '"></div>' +
          '<span class="model-label">' + esc(m.label || m.modelId) + '</span>' +
          '<div class="model-bar-track"><div class="model-bar-fill ' + mcls + '" style="width:' + mpct + '%"></div></div>' +
          '<span class="model-pct ' + mcls + '">' + mpct + '%</span>' +
          '<span class="model-reset">' + resetStr + '</span>' +
          '</div>';
      }
      var expandedCls = isExpanded ? ' is-expanded' : '';
      modelsHTML = '<div class="model-details' + expandedCls + '" id="' + accId + '">' + modelRows + '</div>';
    }

    var chevronCls = isExpanded ? 'chevron expanded' : 'chevron';
    html += '<div class="account-card">' +
      '<div class="account-row" data-toggle="' + accId + '">' +
      '<div class="account-info">' +
      '<div class="account-email"><span class="' + chevronCls + '" id="chev-' + accId + '">▸</span> ' + esc(acc.email) + '</div>' +
      '<div class="account-meta">' +
      (acc.planName ? '<span class="plan-badge">' + esc(acc.planName) + '</span>' : '') +
      creditsHTML +
      '<span class="staleness">' + esc(acc.stalenessLabel) + '</span>' +
      '</div></div>' +
      groupCells +
      '<div style="text-align:center"><span class="status-badge ' + badgeCls + '">' + badgeText + '</span></div>' +
      '</div>' +
      modelsHTML +
      '</div>';
  }

  grid.innerHTML = html;
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

  // Group by category
  var grouped = {};
  for (var i = 0; i < subs.length; i++) {
    var s = subs[i];
    var cat = s.category || 'other';
    if (!grouped[cat]) grouped[cat] = [];
    grouped[cat].push(s);
  }

  var catOrder = ['coding', 'chat', 'api', 'image', 'audio', 'productivity', 'other'];
  var html = '';

  for (var ci = 0; ci < catOrder.length; ci++) {
    var catKey = catOrder[ci];
    var items = grouped[catKey];
    if (!items || items.length === 0) continue;

    html += '<div class="sub-category-label">' + esc(catKey) + ' (' + items.length + ')</div>';

    for (var si = 0; si < items.length; si++) {
      var sub = items[si];
      html += renderSubCard(sub);
    }
  }

  grid.innerHTML = html;
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

  return '<div class="sub-card" data-sub-id="' + sub.id + '">' +
    '<div class="sub-card-header">' +
    '<div class="sub-card-title">' + esc(sub.platform) + '</div>' +
    '<div class="sub-card-badges">' + trialHTML + badgesHTML + '</div>' +
    '</div>' +
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

function handleSnap() {
  var btn = document.getElementById('snap-btn');
  if (!btn || btn.disabled || snapInProgress) return;

  snapInProgress = true;
  btn.disabled = true;
  btn.classList.add('snapping');
  var orig = btn.innerHTML;
  btn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/></svg> Capturing...';

  triggerSnap().then(function(data) {
    showToast('✅ Captured: ' + data.email, 'success');
    renderAccounts(data);
    updateTimestamp();
  }).catch(function(err) {
    showToast('❌ ' + err.message, 'error');
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
  return d.innerHTML;
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

function updateTimestamp() {
  var el = document.getElementById('last-updated');
  if (el) el.textContent = 'Updated: ' + new Date().toLocaleTimeString();
}

// ════════════════════════════════════════════
//  CHART — Quota History
// ════════════════════════════════════════════

var historyChart = null;

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

  for (var i = 0; i < snapshots.length; i++) {
    var groups = snapshots[i].groups || [];
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
      fill: true,
      tension: 0.3,
      pointRadius: 3,
      pointHoverRadius: 6,
      borderWidth: 2,
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
            label: function(ctx) { return ctx.dataset.label + ': ' + ctx.parsed.y + '%'; }
          }
        }
      },
      scales: {
        y: {
          min: 0, max: 100,
          grid: { color: gridColor },
          ticks: { color: textColor, font: { family: "'Inter', sans-serif", size: 11 }, callback: function(v) { return v + '%'; } },
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
    chips.push({ icon: '💡', text: 'Could save ~$' + annualSavings.toFixed(0) + '/yr by switching to annual billing', cls: 'good' });
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
  // Fetch both overview and subscriptions for insights
  Promise.all([fetchOverview(), fetchSubscriptions('', '')]).then(function(results) {
    var data = results[0];
    var subsData = results[1];
    renderOverviewEnhanced(data, subsData.subscriptions || []);
  }).catch(function(err) {
    console.error('Failed to load overview:', err);
  });
}

function renderOverviewEnhanced(data, subs) {
  var el = document.getElementById('overview-content');
  if (!el) return;

  var stats = data.stats || { totalMonthlySpend: 0, totalAnnualSpend: 0, byCategory: {}, byStatus: {} };
  var renewals = data.renewals || [];
  var links = data.quickLinks || [];
  var quotas = data.quotaSummary;

  // ── Budget Alert ──
  var budgetHTML = renderBudgetAlert(stats.totalMonthlySpend);
  if (!getBudget()) {
    budgetHTML = '<div class="budget-alert ok">' +
      '<span class="budget-icon">💰</span>' +
      '<span class="budget-msg">No monthly budget set. Set one to track spending.</span>' +
      '<button class="budget-btn" onclick="openBudgetModal()">Set Budget</button>' +
      '</div>';
  }

  // ── Insights ──
  var insights = generateInsights(stats, renewals, subs);
  var insightsHTML = renderInsightChips(insights);

  // ── Spend card ──
  var spendHTML = '<div class="overview-card">' +
    '<h3>Monthly AI Spend</h3>' +
    '<div class="overview-big-number">$' + stats.totalMonthlySpend.toFixed(2) + '</div>' +
    '<div class="overview-big-label">$' + stats.totalAnnualSpend.toFixed(2) + '/year estimated</div>' +
    '</div>';

  // ── Category breakdown ──
  var catHTML = '<div class="overview-card"><h3>By Category</h3>';
  var cats = Object.keys(stats.byCategory);
  if (cats.length === 0) {
    catHTML += '<div class="empty-hint">No subscriptions yet</div>';
  } else {
    cats.sort(function(a, b) {
      return (stats.byCategory[b].monthlySpend || 0) - (stats.byCategory[a].monthlySpend || 0);
    });
    for (var i = 0; i < cats.length; i++) {
      var c = stats.byCategory[cats[i]];
      catHTML += '<div class="overview-category-row">' +
        '<span class="overview-category-name">' + esc(cats[i]) + '<span class="overview-category-count">' + c.count + ' subs</span></span>' +
        '<span class="overview-category-spend">$' + c.monthlySpend.toFixed(2) + '/mo</span>' +
        '</div>';
    }
  }
  catHTML += '</div>';

  // ── Renewals ──
  var renewHTML = '<div class="overview-card full-width"><h3>Upcoming Renewals</h3>';
  if (renewals.length === 0) {
    renewHTML += '<div class="empty-hint">No upcoming renewals</div>';
  } else {
    for (var r = 0; r < renewals.length; r++) {
      var ren = renewals[r];
      var daysCls = ren.daysUntil <= 7 ? 'soon' : 'distant';
      renewHTML += '<div class="renewal-item">' +
        '<span class="renewal-name">' + esc(ren.platform) + '</span>' +
        '<span class="renewal-date">' + ren.nextRenewal + '</span>' +
        '<span class="renewal-days ' + daysCls + '">' + ren.daysUntil + ' days</span>' +
        '</div>';
    }
  }
  renewHTML += '</div>';

  // ── Quick Links ──
  var linksHTML = '<div class="overview-card full-width"><h3>Quick Links</h3>';
  if (links.length === 0) {
    linksHTML += '<div class="empty-hint">Add subscriptions with dashboard URLs to see quick links here</div>';
  } else {
    linksHTML += '<div class="quick-links-grid">';
    for (var l = 0; l < links.length; l++) {
      linksHTML += '<a class="quick-link" href="' + esc(links[l].url) + '" target="_blank" rel="noopener">' +
        '🔗 ' + esc(links[l].platform) + '</a>';
    }
    linksHTML += '</div>';
  }
  linksHTML += '</div>';

  // ── CSV Export ──
  var exportHTML = '<div class="overview-card full-width"><h3>Export</h3>' +
    '<p style="font-size:13px;color:var(--text-secondary);margin-bottom:12px">' +
    'Download all subscriptions as CSV for expense tracking or tax reports.</p>' +
    '<a class="btn-add" href="/api/export/csv" download style="text-decoration:none;display:inline-flex">📥 Export CSV</a>' +
    '</div>';

  // ── Ready Now ──
  var readyHTML = '';
  if (quotas && quotas.length > 0) {
    readyHTML = '<div class="overview-card full-width"><h3>Ready Now — Auto-Tracked Quotas</h3>';
    for (var q = 0; q < quotas.length; q++) {
      var acct = quotas[q];
      var readyIcon = acct.isReady ? '✅' : '⚠️';
      var grps = acct.groups || [];
      var grpInfo = '';
      for (var gi = 0; gi < grps.length; gi++) {
        var g = grps[gi];
        var gpct = Math.round(g.remainingPercent);
        var gicon = gpct > 50 ? '✅' : (gpct > 0 ? '⚠️' : '❌');
        var resetInfo = '';
        if (g.timeUntilResetSec > 0 && gpct === 0) {
          resetInfo = ' ↻ ' + formatSeconds(g.timeUntilResetSec);
        }
        grpInfo += '<span class="sub-limit-chip">' + gicon + ' ' + esc(g.displayName) + ': ' + gpct + '%' + resetInfo + '</span> ';
      }
      readyHTML += '<div style="margin-bottom:10px">' +
        '<div style="font-weight:600;font-size:14px;margin-bottom:4px">' + readyIcon + ' ' + esc(acct.email) + '</div>' +
        '<div class="sub-card-limits">' + grpInfo + '</div>' +
        '</div>';
    }
    readyHTML += '</div>';
  }

  el.innerHTML = budgetHTML + insightsHTML + spendHTML + catHTML + renewHTML + linksHTML + exportHTML + readyHTML;
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
    if (typeof loadHistoryChart === 'function') loadHistoryChart();
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
      aboutEl.textContent = 'Schema v3 · 26 presets · Mode: ' +
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

document.addEventListener('DOMContentLoaded', function() {
  initTheme();
  initTabs();
  setupToggle();
  initModal();
  initBudget();
  initSettings();
  initSearch();
  initKeyboardShortcuts();

  document.getElementById('snap-btn').addEventListener('click', handleSnap);

  // Chart controls
  document.getElementById('chart-account').addEventListener('change', loadHistoryChart);
  document.getElementById('chart-range').addEventListener('change', loadHistoryChart);

  // Load quotas (existing behavior)
  fetchStatus().then(function(data) {
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

  // Auto-capture polling is handled server-side by the agent.
  // Manual data refreshes on snap or page reload.
});

