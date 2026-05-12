// Niyantra Dashboard — Command Palette
import { esc } from '../core/utils.js';
import { switchToTab } from '../core/theme.js';
import { handleSnap } from './snap.js';
import { handleCodexSnap } from './codex.js';
import { openModal } from '../subscriptions.js';
import { openBudgetModal } from '../overview/budget.js';
import { updateChartTheme } from '../charts/history.js';


export var PALETTE_COMMANDS = [
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

export var paletteSelectedIndex = 0;
export var paletteFilteredCommands = PALETTE_COMMANDS;

export function initCommandPalette() {
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

export function toggleCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  if (overlay.hidden) {
    openCommandPalette();
  } else {
    closeCommandPalette();
  }
}

export function openCommandPalette() {
  var overlay = document.getElementById('command-palette-overlay');
  var search = document.getElementById('command-palette-search');
  overlay.hidden = false;
  search.value = '';
  paletteFilteredCommands = PALETTE_COMMANDS;
  paletteSelectedIndex = 0;
  renderPaletteList();
  setTimeout(function() { search.focus(); }, 50);
}

export function closeCommandPalette() {
  document.getElementById('command-palette-overlay').hidden = true;
}

export function renderPaletteList() {
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
