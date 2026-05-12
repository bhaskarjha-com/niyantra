// Niyantra Dashboard — Snap Handler
import { snapInProgress, setSnapInProgress } from '../core/state.js';
import { showToast, updateTimestamp } from '../core/utils.js';
import { triggerSnap } from '../core/api.js';
import { renderAccounts } from '../quotas/render.js';




// H3: Split-button snap — source-aware snapping
var snapDefault = localStorage.getItem('niyantra_snap_default') || 'antigravity';

export function initSnapDropdown() {
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

export function updateSnapDropdownIndicators() {
  var dropdown = document.getElementById('snap-dropdown');
  if (!dropdown) return;
  dropdown.querySelectorAll('.snap-option').forEach(function(opt) {
    if (opt.dataset.source === 'all') return; // divider option
    var isActive = opt.dataset.source === snapDefault;
    opt.textContent = (isActive ? '◉ ' : '○ ') + opt.textContent.replace(/^[◉○] /, '');
    opt.classList.toggle('active', isActive);
  });
}

export function handleSnap() {
  snapSource(snapDefault);
}

export function snapSource(source) {
  var btn = document.getElementById('snap-btn');
  if (!btn || btn.disabled || snapInProgress) return;

  setSnapInProgress(true);
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
    setSnapInProgress(false);
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
    setSnapInProgress(false);
  });
}



