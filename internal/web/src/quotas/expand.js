// Niyantra Dashboard — Quota Expand/Collapse & Init
// Toggle handlers, sort delegation, quick adjust, and quota init.

import {
  GROUP_NAMES, expandedAccounts,
  quotaSortState, latestQuotaData,
} from '../core/state.js';
import { esc, showToast } from '../core/utils.js';
import { fetchStatus } from '../core/api.js';
import { renderAccounts, handleTagFilterClick } from './render.js';
// ── TOGGLE — Quotas expand/collapse ──

export function setupToggle() {
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
            document.dispatchEvent(new CustomEvent('niyantra:chart-refresh'));
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
            document.dispatchEvent(new CustomEvent('niyantra:chart-refresh'));
          })
          .catch(function(err) { showToast('❌ ' + err.message, 'error'); });
      }
      return;
    }

    // Handle Group-level Quick Adjust buttons (±5% on group columns)
    var gadjBtn = e.target.closest('.gadj-btn');
    if (gadjBtn) {
      e.stopPropagation();
      var gControls = gadjBtn.closest('.group-adjust');
      if (!gControls) return;
      var gSnapId = parseInt(gControls.getAttribute('data-snap-id'), 10);
      var gGroupKey = gControls.getAttribute('data-group-key');
      var gLabelsStr = gControls.getAttribute('data-group-labels');
      var gCurrentPct = parseFloat(gControls.getAttribute('data-current-pct'));
      var gDelta = parseFloat(gadjBtn.getAttribute('data-delta'));
      var gNewPct = Math.max(0, Math.min(100, gCurrentPct + gDelta));

      // Optimistic UI update on group cell
      var cell = gControls.closest('.quota-cell');
      if (cell) {
        var gPctSpan = cell.querySelector('.quota-pct');
        var gBarFill = cell.querySelector('.quota-minibar-fill');
        if (gPctSpan) {
          gPctSpan.textContent = Math.round(gNewPct) + '%';
          gPctSpan.className = 'quota-pct ' + (gNewPct <= 0 ? 'exhausted' : gNewPct < 20 ? 'warning' : gNewPct < 50 ? 'ok' : 'good');
        }
        if (gBarFill) {
          gBarFill.style.width = gNewPct + '%';
          gBarFill.className = 'quota-minibar-fill ' + (gNewPct <= 0 ? 'exhausted' : gNewPct < 20 ? 'warning' : gNewPct < 50 ? 'ok' : 'good');
        }
      }
      gControls.setAttribute('data-current-pct', gNewPct);

      // Build adjustments for ALL models in this group
      var gLabels = gLabelsStr.split('|||').filter(function(l) { return l.length > 0; });
      var adjustments = [];
      for (var li = 0; li < gLabels.length; li++) {
        // Each model gets the same delta applied
        // Note: this is approximate — individual models may have different starting values
        // The backend calculates the actual new value per model
        adjustments.push({ label: gLabels[li], remainingPercent: gNewPct });
      }

      if (adjustments.length === 0) return;

      fetch('/api/snap/adjust', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ snapshotId: gSnapId, adjustments: adjustments })
      })
      .then(function(res) { return res.json(); })
      .then(function(data) {
        if (data.error) {
          showToast('❌ ' + data.error, 'error');
          return;
        }
        var groupName = GROUP_NAMES[gGroupKey] || gGroupKey;
        showToast('✎ ' + groupName + ' → ' + Math.round(gNewPct) + '% (' + adjustments.length + ' models)', 'info');
        fetchStatus().then(renderAccounts);
      })
      .catch(function(err) { showToast('❌ ' + err.message, 'error'); });
      return;
    }

    // Handle Quick Adjust buttons (±5%, ±10%)
    var adjBtn = e.target.closest('.adj-btn');
    if (adjBtn) {
      e.stopPropagation();
      var controls = adjBtn.closest('.adjust-controls');
      if (!controls) return;
      var snapId = parseInt(controls.getAttribute('data-snap-id'), 10);
      var label = controls.getAttribute('data-model-label');
      var currentPct = parseFloat(controls.getAttribute('data-current-pct'));
      var delta = parseFloat(adjBtn.getAttribute('data-delta'));
      var newPct = Math.max(0, Math.min(100, currentPct + delta));

      // Optimistic UI update
      var row = controls.closest('.model-row');
      if (row) {
        var pctSpan = row.querySelector('.model-pct');
        var barFill = row.querySelector('.model-bar-fill');
        if (pctSpan) {
          pctSpan.textContent = Math.round(newPct) + '%';
          pctSpan.className = 'model-pct ' + (newPct <= 0 ? 'exhausted' : newPct < 20 ? 'warning' : newPct < 50 ? 'ok' : 'good');
        }
        if (barFill) {
          barFill.style.width = newPct + '%';
          barFill.className = 'model-bar-fill ' + (newPct <= 0 ? 'exhausted' : newPct < 20 ? 'warning' : newPct < 50 ? 'ok' : 'good');
        }
      }
      controls.setAttribute('data-current-pct', newPct);

      // API call
      fetch('/api/snap/adjust', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          snapshotId: snapId,
          adjustments: [{ label: label, remainingPercent: newPct }]
        })
      })
      .then(function(res) { return res.json(); })
      .then(function(data) {
        if (data.error) {
          showToast('❌ ' + data.error, 'error');
          return;
        }
        showToast('✎ Adjusted ' + label + ' → ' + Math.round(newPct) + '%', 'info');
        // Refresh status to recalculate group-level aggregates
        fetchStatus().then(renderAccounts);
      })
      .catch(function(err) { showToast('❌ ' + err.message, 'error'); });
      return;
    }

    // Handle row toggle (existing)
    // Guard: skip expand/collapse if clicking on tag/note/pin/renewal controls
    if (e.target.closest('[data-tag-add]') || e.target.closest('[data-remove-tag]') ||
        e.target.closest('[data-note-edit]') || e.target.closest('[data-pin-group]') ||
        e.target.closest('[data-renewal-edit]') || e.target.closest('.tag-picker') ||
        e.target.closest('.tag-chip')) {
      return;
    }
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

// ── INIT — Quotas tab event wiring ──

export function initQuotas() {
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

  // F4: Tag filter chip click handler (delegated)
  var tagStrip = document.getElementById('tag-filter-strip');
  if (tagStrip) {
    tagStrip.addEventListener('click', handleTagFilterClick);
  }
}
