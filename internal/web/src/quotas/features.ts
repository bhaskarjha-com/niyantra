// Niyantra Dashboard — Quota Features
// Pinned model, credit renewal, account tags & notes.

import { GROUP_NAMES } from '../core/state';
import type { AccountReadiness } from '../types/api';
import { esc, showToast } from '../core/utils';
import { fetchStatus } from '../core/api';

// Forward reference — set by render.js to avoid circular import
var _renderAccounts: ((data: any) => void) | null = null;
export function setRenderAccounts(fn: (data: any) => void): void { _renderAccounts = fn; }

function refreshGrid(): void {
  if (_renderAccounts) fetchStatus().then(_renderAccounts);
}

// ── Pinned/Favorite Model ──

export function renderPinnedBadge(groupData: any, pinnedKey: string): string {
  if (!groupData) return '';
  var pct = Math.round(groupData.remainingPercent);
  var cls = 'good';
  if (groupData.isExhausted || pct === 0) cls = 'exhausted';
  else if (pct < 20) cls = 'warning';
  else if (pct < 50) cls = 'ok';
  return ' <span class="pinned-badge ' + cls + '" title="Pinned: ' + esc(groupData.displayName || pinnedKey) + '">' +
    '★ ' + esc(groupData.displayName || GROUP_NAMES[pinnedKey] || pinnedKey) + ': ' + pct + '%</span>';
}

export function pinGroup(accountId: number | string, groupKey: string): void {
  updateAccountMeta(accountId, { pinnedGroup: groupKey }).then(function() {
    showToast('⭐ Pinned ' + (GROUP_NAMES[groupKey] || groupKey), 'success');
    refreshGrid();
  });
}

export function unpinGroup(accountId: number | string): void {
  updateAccountMeta(accountId, { pinnedGroup: '' }).then(function() {
    showToast('☆ Unpinned — will show first group', 'info');
    refreshGrid();
  });
}

// ── Credit Renewal Day ──

export function daysUntilRenewal(day: number): number {
  if (!day || day < 1 || day > 31) return -1;
  var now = new Date();
  var y = now.getFullYear();
  var m = now.getMonth();
  var today = now.getDate();
  var targetMonth = today < day ? m : m + 1;
  var target = new Date(y, targetMonth, day);
  var diff = Math.ceil((target.getTime() - now.getTime()) / (1000 * 60 * 60 * 24));
  return diff < 0 ? 0 : diff;
}

export function renderCreditRenewal(accountId: number, renewalDay: number): string {
  if (!renewalDay || renewalDay < 1) {
    return '<span class="credit-renewal-set" data-renewal-edit="' + accountId + '" title="Set credit renewal day">↻ set</span>';
  }
  var days = daysUntilRenewal(renewalDay);
  var label = days === 0 ? 'today' : days === 1 ? '1d' : days + 'd';
  return '<span class="credit-renewal" data-renewal-edit="' + accountId + '" data-renewal-day="' + renewalDay + '" title="Credits renew on day ' + renewalDay + ' (↻ ' + label + ')">↻ ' + label + '</span>';
}

export function openRenewalPicker(el: HTMLElement): void {
  var existing = document.querySelector('.renewal-picker');
  if (existing) existing.remove();

  var accountId = el.getAttribute('data-renewal-edit');
  var currentDay = parseInt(el.getAttribute('data-renewal-day')!) || 0;

  var picker = document.createElement('div');
  picker.className = 'renewal-picker';
  picker!.innerHTML =
    '<div class="renewal-picker-label">Credit Renewal Day</div>' +
    '<input type="number" class="renewal-picker-input" min="1" max="31" value="' + (currentDay || '') + '" placeholder="1–31">' +
    '<div class="renewal-picker-hint">Day of month when AI credits refresh.<br>Find at one.google.com/ai/activity</div>';

  el!.closest('.credits-cell')!.appendChild(picker);
  var input = picker!.querySelector('input') as HTMLInputElement | null;
  input!.focus();
  input!.select();

  function save() {
    var day = parseInt((input as HTMLInputElement).value) || 0;
    if (day > 31) day = 31;
    if (day < 0) day = 0;
    picker!.remove();
    updateAccountMeta(accountId!, { creditRenewalDay: day }).then(function() {
      if (day > 0) {
        showToast('↻ Renewal day set to ' + day, 'success');
      } else {
        showToast('↻ Renewal day cleared', 'info');
      }
      refreshGrid();
    });
  }

  input!.addEventListener('keydown', function(e) {
    if ((e as KeyboardEvent).key === 'Enter') { e.preventDefault(); save(); }
    if ((e as KeyboardEvent).key === 'Escape') { e.preventDefault(); picker!.remove(); }
  });
  input!.addEventListener('blur', function() {
    setTimeout(function() { if (picker.parentNode) save(); }, 150);
  });
}

// ── Account Tags + Notes ──

var TAG_PRESETS = ['work', 'personal', 'primary', 'backup', 'shared', 'test', 'dev'];

export function renderAccountTags(acc: any): string {
  var tags = (acc.tags || '').split(',').filter(function(t: any) { return t.trim(); });
  var html = '<span class="account-tags" data-account-id="' + acc.accountId + '">';
  for (var i = 0; i < tags.length; i++) {
    html += '<span class="tag-chip" data-tag="' + esc(tags[i].trim()) + '">' +
      esc(tags[i].trim()) +
      '<span class="tag-remove" data-remove-tag="' + esc(tags[i].trim()) + '" data-account-id="' + acc.accountId + '" title="Remove tag">✕</span>' +
      '</span>';
  }
  html += '</span>';
  html += '<button class="tag-add-btn" data-tag-add="' + acc.accountId + '" title="Add tag">+</button>';
  return html;
}

export function renderAccountNote(acc: any): string {
  if (acc.notes) {
    return '<span class="account-note" data-note-edit="' + acc.accountId + '" data-current-note="' + esc(acc.notes) + '" title="' + esc(acc.notes) + ' — click to edit">📝 ' + esc(acc.notes) + '</span>';
  }
  return '<span class="account-note-empty" data-note-edit="' + acc.accountId + '" data-current-note="">+ note</span>';
}

export function updateAccountMeta(accountId: number | string, patch: Record<string, any>): Promise<any> {
  return fetch('/api/accounts/' + accountId + '/meta', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  }).then(function(r) { return r.json(); });
}

export function addTagToAccount(accountId: number | string, newTag: string): void {
  newTag = newTag.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '');
  if (!newTag) return;
  var container = document.querySelector('.account-tags[data-account-id="' + accountId + '"]');
  var existing: any[] = [];
  if (container) {
    container.querySelectorAll('.tag-chip').forEach(function(chip) {
      existing.push(chip.getAttribute('data-tag'));
    });
  }
  if (existing.indexOf(newTag) >= 0) return;
  existing.push(newTag);
  updateAccountMeta(accountId, { tags: existing.join(',') }).then(function() {
    showToast('🏷️ Tag "' + newTag + '" added', 'success');
    refreshGrid();
  });
}

export function removeTagFromAccount(accountId: number | string, tag: string): void {
  var container = document.querySelector('.account-tags[data-account-id="' + accountId + '"]');
  var tags: any[] = [];
  if (container) {
    container.querySelectorAll('.tag-chip').forEach(function(chip) {
      var t = chip.getAttribute('data-tag');
      if (t !== tag) tags.push(t);
    });
  }
  updateAccountMeta(accountId, { tags: tags.join(',') }).then(function() {
    showToast('🏷️ Tag removed', 'success');
    refreshGrid();
  });
}

export function openTagPicker(btn: HTMLElement): void {
  closeTagPicker();
  var accountId = btn.getAttribute('data-tag-add');
  var meta = btn.closest('.account-meta');
  if (!meta) return;
  var existing: any[] = [];
  var container = meta.querySelector('.account-tags');
  if (container) {
    container.querySelectorAll('.tag-chip').forEach(function(chip) {
      existing.push(chip.getAttribute('data-tag'));
    });
  }
  var picker = document.createElement('div');
  picker.className = 'tag-picker';
  picker.id = 'active-tag-picker';
  picker!.innerHTML = '<input type="text" class="tag-picker-input" placeholder="Type tag name..." autocomplete="off" maxlength="20">' +
    '<div class="tag-picker-hint">Enter to add</div>' +
    '<div class="tag-picker-presets">' +
    TAG_PRESETS.map(function(p) {
      var active = existing.indexOf(p) >= 0 ? ' active' : '';
      return '<button class="tag-preset' + active + '" data-preset-tag="' + p + '">' + p + '</button>';
    }).join('') +
    '</div>';
  meta.appendChild(picker);
  var input = picker!.querySelector('.tag-picker-input');
  (input as HTMLElement).focus();
  picker.addEventListener('click', function(e) { e.stopPropagation(); });
  input!.addEventListener('keydown', function(e) {
    if ((e as KeyboardEvent).key === 'Enter') {
      e.preventDefault();
      var val = (input as HTMLInputElement).value.trim();
      if (val) { addTagToAccount(accountId!, val); closeTagPicker(); }
    }
    if ((e as KeyboardEvent).key === 'Escape') { closeTagPicker(); }
  });
  picker!.querySelectorAll('.tag-preset').forEach(function(btn2) {
    btn2.addEventListener('click', function(e) {
      e.stopPropagation();
      var tag = btn2.getAttribute('data-preset-tag');
      if (btn2.classList.contains('active')) { removeTagFromAccount(accountId!, tag!); }
      else { addTagToAccount(accountId!, tag!); }
      closeTagPicker();
    });
  });
  setTimeout(function() { document.addEventListener('click', closeTagPickerOnOutside); }, 10);
}

export function closeTagPicker(): void {
  var picker = document.getElementById('active-tag-picker');
  if (picker) picker!.remove();
  document.removeEventListener('click', closeTagPickerOnOutside);
}

function closeTagPickerOnOutside(e: Event): void {
  var picker = document.getElementById('active-tag-picker');
  if (picker && !picker!.contains(e.target as Node)) { closeTagPicker(); }
}

export function openNoteEditor(el: HTMLElement): void {
  var accountId = el.getAttribute('data-note-edit');
  var currentNote = el.getAttribute('data-current-note') || '';
  var editor = document.createElement('span');
  editor.className = 'note-inline-editor';
  editor.innerHTML = '<input type="text" class="note-inline-input" value="' + esc(currentNote) + '" placeholder="Add a note..." maxlength="100">';
  el.replaceWith(editor);
  var input = editor.querySelector('.note-inline-input');
  (input as HTMLElement).focus();
  (input as HTMLInputElement).select();
  editor.addEventListener('click', function(e) { e.stopPropagation(); });
  function save() {
    var val = (input as HTMLInputElement).value.trim();
    updateAccountMeta(accountId!, { notes: val }).then(function() {
      if (val) showToast('📝 Note saved', 'success');
      refreshGrid();
    });
  }
  input!.addEventListener('keydown', function(e) {
    if ((e as KeyboardEvent).key === 'Enter') { e.preventDefault(); save(); }
    if ((e as KeyboardEvent).key === 'Escape') { refreshGrid(); }
  });
  input!.addEventListener('blur', save);
}

// Wire F1 interactions via delegation on the grid
export function initAccountMetaHandlers(): void {
  var grid = document.getElementById('account-grid');
  if (!grid) return;
  grid.addEventListener('click', function(e) {
    var removeBtn = (e.target as HTMLElement).closest('[data-remove-tag]');
    if (removeBtn) {
      e.stopPropagation(); e.preventDefault();
      removeTagFromAccount(removeBtn.getAttribute('data-account-id')!, removeBtn.getAttribute('data-remove-tag')!);
      return;
    }
    var addBtn = (e!.target as HTMLElement).closest('[data-tag-add]');
    if (addBtn) { e.stopPropagation(); e.preventDefault(); openTagPicker(addBtn as HTMLElement); return; }
    var noteEl = (e!.target as HTMLElement).closest('[data-note-edit]');
    if (noteEl) { e.stopPropagation(); e.preventDefault(); openNoteEditor(noteEl as HTMLElement); return; }
    var pinBtn = (e!.target as HTMLElement).closest('[data-pin-group]');
    if (pinBtn) {
      e.stopPropagation(); e.preventDefault();
      var pinAccountId = pinBtn.getAttribute('data-pin-account')!;
      var pinGroupKey = pinBtn.getAttribute('data-pin-group')!;
      if (pinBtn.classList.contains('pinned')) { unpinGroup(pinAccountId); }
      else { pinGroup(pinAccountId, pinGroupKey); }
      return;
    }
    var renewalEl = (e!.target as HTMLElement).closest('[data-renewal-edit]');
    if (renewalEl) { e.stopPropagation(); e.preventDefault(); openRenewalPicker(renewalEl as HTMLElement); return; }
  });
}
