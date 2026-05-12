// Niyantra Dashboard — Subscriptions Module
// @ts-nocheck
// Render, modal, search for subscription management.

import { presetsData } from './core/state';
import type { Subscription } from './types/api';
import { esc, showToast, currencySymbol, formatDurationSec } from './core/utils';
import { fetchSubscriptions, createSubscription, updateSubscription, deleteSubscription } from './core/api';

export function loadSubscriptions(): void {
  var status = document.getElementById('filter-status').value;
  var category = document.getElementById('filter-category').value;

  fetchSubscriptions(status, category).then(function(data) {
    renderSubscriptions(data);
  }).catch(function(err) {
    console.error('Failed to load subscriptions:', err);
  });
}

export function renderSubscriptions(data) {
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

  // Split subs into auto-tracked provider groups vs manual
  var providerGroups = {};
  var manualSubs = [];
  var grandTotal = 0;
  for (var i = 0; i < subs.length; i++) {
    var s = subs[i];
    var monthly = 0;
    if (s.costAmount > 0) {
      if (s.billingCycle === 'yearly') monthly = s.costAmount / 12;
      else monthly = s.costAmount;
    }
    grandTotal += monthly;

    if (s.autoTracked) {
      var pkey = s.platform || 'Unknown';
      if (!providerGroups[pkey]) providerGroups[pkey] = { items: [], total: 0 };
      providerGroups[pkey].items.push(s);
      providerGroups[pkey].total += monthly;
    } else {
      manualSubs.push(s);
    }
  }

  var providerKeys = Object.keys(providerGroups);
  var autoCount = subs.length - manualSubs.length;
  var sym = currencySymbol(subs[0] ? subs[0].costCurrency : 'USD');

  // ── Spend Summary Bar (compact) ──
  var html = '<div class="spend-summary-card">' +
    '<div class="spend-hero">' +
    '<div class="spend-amount">' + sym + grandTotal.toFixed(2) + '<span class="spend-period">/mo</span></div>' +
    '<div class="spend-label">Total Monthly Spend</div>' +
    '</div>' +
    '<div class="spend-breakdown">';

  var providerIcons = { 'Antigravity': '⚡', 'Codex': '🤖', 'Claude': '🔮' };
  for (var pk = 0; pk < providerKeys.length; pk++) {
    var pName = providerKeys[pk];
    var pIcon = providerIcons[pName] || '📦';
    var pTotal = providerGroups[pName].total;
    html += '<span class="spend-chip">' + pIcon + ' ' + esc(pName) +
      ' <strong>' + sym + pTotal.toFixed(2) + '</strong></span>';
  }
  if (manualSubs.length > 0) {
    var manualTotal = 0;
    for (var mi = 0; mi < manualSubs.length; mi++) {
      if (manualSubs[mi].costAmount > 0) {
        manualTotal += manualSubs[mi].billingCycle === 'yearly'
          ? manualSubs[mi].costAmount / 12 : manualSubs[mi].costAmount;
      }
    }
    if (manualTotal > 0) {
      html += '<span class="spend-chip">📋 Manual <strong>' + sym + manualTotal.toFixed(2) + '</strong></span>';
    }
  }

  html += '</div>' +
    '<div class="spend-meta">' + autoCount + ' auto-tracked · ' + manualSubs.length + ' manual</div>' +
    '</div>';

  // ── Auto-Tracked Provider Sections (with CARD grid inside) ──
  for (var pi = 0; pi < providerKeys.length; pi++) {
    var provider = providerKeys[pi];
    var group = providerGroups[provider];
    var items = group.items;
    var icon = providerIcons[provider] || '📦';

    var sectionId = 'sub-provider-' + provider.replace(/\s+/g, '-').toLowerCase();
    var providerAttr = provider.toLowerCase().replace(/[^a-z]/g, '');
    html += '<div class="provider-section" data-provider="' + providerAttr + '">' +
      '<div class="provider-header" data-toggle-provider="' + sectionId + '">' +
      '<div class="provider-header-left">' +
      '<span class="provider-chevron" id="pchev-' + sectionId + '">▾</span> ' +
      '<span class="provider-icon">' + icon + '</span>' +
      '<span class="provider-name">' + esc(provider) + '</span>' +
      '<span class="provider-count">' + items.length + ' account' + (items.length !== 1 ? 's' : '') + '</span>' +
      '</div>' +
      '<span class="provider-spend">' + sym + group.total.toFixed(2) + '/mo</span>' +
      '</div>' +
      '<div class="provider-body" id="' + sectionId + '">' +
      '<div class="subs-card-grid">';

    // Render each auto-tracked sub as a FULL CARD (same as manual)
    for (var si = 0; si < items.length; si++) {
      html += renderSubCard(items[si]);
    }

    html += '</div></div></div>';
  }

  // ── Manual Subscriptions ──
  if (manualSubs.length > 0) {
    var grouped = {};
    for (var mi2 = 0; mi2 < manualSubs.length; mi2++) {
      var cat = manualSubs[mi2].category || 'other';
      if (!grouped[cat]) grouped[cat] = [];
      grouped[cat].push(manualSubs[mi2]);
    }
    var catOrder = ['coding', 'chat', 'api', 'image', 'audio', 'productivity', 'other'];
    html += '<div class="sub-section-label">Manual Subscriptions (' + manualSubs.length + ')</div>';
    html += '<div class="subs-card-grid">';
    for (var ci = 0; ci < catOrder.length; ci++) {
      var catItems = grouped[catOrder[ci]];
      if (!catItems || catItems.length === 0) continue;
      for (var csi = 0; csi < catItems.length; csi++) {
        html += renderSubCard(catItems[csi]);
      }
    }
    html += '</div>';
  } else if (providerKeys.length > 0) {
    html += '<div class="sub-section-label">Manual Subscriptions</div>' +
      '<div class="manual-empty">' +
      '<p>No manual subscriptions tracked.</p>' +
      '<p class="empty-hint">Click <strong>+ Add</strong> to track Claude Pro, Cursor, or other AI tools.</p>' +
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

export function renderSubCard(sub) {
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
//  MODAL — Add/Edit Subscription
// ════════════════════════════════════════════

export function initModal(): void {
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

export function openModal(sub?: any): void {
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

export function closeModal(): void {
  document.getElementById('modal-overlay').hidden = true;
}

export function fillFromPreset(preset) {
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

export function openEditModal(id) {
  fetch('/api/subscriptions/' + id).then(function(res) {
    return res.json();
  }).then(function(sub) {
    openModal(sub);
  }).catch(function(err) {
    showToast('❌ ' + err.message, 'error');
  });
}

export function handleSave() {
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

export function openDeleteConfirm(id, name) {
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

export function closeDelete(): void {
  document.getElementById('delete-overlay').hidden = true;
  pendingDeleteId = null;
}

// ════════════════════════════════════════════
//  SEARCH — Subscriptions
// ════════════════════════════════════════════

export function initSearch(): void {
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
