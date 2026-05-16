// Niyantra Dashboard — Plugin System UI (F18)
import { showToast, esc, formatTimeAgo } from '../core/utils';

interface PluginInfo {
  manifest: {
    id: string;
    name: string;
    version: string;
    description: string;
    author: string;
    timeout: number;
    config: Record<string, { type: string; label: string; default?: string; required?: boolean; secret?: boolean }>;
  };
  dir: string;
  enabled: boolean;
  config: Record<string, string>;
  lastCapture?: string;
  captureCount: number;
}

export function loadPlugins(): void {
  fetch('/api/plugins').then(function(r) { return r.json(); })
  .then(function(data) {
    var container = document.getElementById('plugins-list');
    if (!container) return;

    var plugins: PluginInfo[] = data.plugins || [];
    var pluginsDir: string = data.pluginsDir || '';
    var errors: string[] = data.errors || [];

    if (plugins.length === 0 && errors.length === 0) {
      container.innerHTML =
        '<div class="plugin-empty">' +
          '<div class="plugin-empty-icon">🧩</div>' +
          '<div class="plugin-empty-title">No plugins installed</div>' +
          '<div class="plugin-empty-hint">' +
            'Add plugins to <code>' + esc(pluginsDir) + '</code><br>' +
            'Each plugin needs a <code>plugin.json</code> manifest and an executable entry point.' +
          '</div>' +
        '</div>';
      return;
    }

    var html = '';

    // Show discovery errors if any
    if (errors.length > 0) {
      html += '<div class="plugin-errors">';
      errors.forEach(function(e) {
        html += '<div class="plugin-error">⚠️ ' + esc(e) + '</div>';
      });
      html += '</div>';
    }

    // Render each plugin
    plugins.forEach(function(p) {
      var meta = p.captureCount + ' captures';
      if (p.lastCapture) {
        meta += ' · Last: ' + formatTimeAgo(p.lastCapture);
      }

      html += '<div class="plugin-card" data-plugin-id="' + esc(p.manifest.id) + '">';
      html += '<div class="plugin-header">';
      html += '<div class="plugin-info">';
      html += '<div class="plugin-name">' + esc(p.manifest.name) +
              '<span class="plugin-version">v' + esc(p.manifest.version) + '</span></div>';
      html += '<div class="plugin-meta">' + esc(p.manifest.description || 'No description') + '</div>';
      if (p.manifest.author) {
        html += '<div class="plugin-meta">By ' + esc(p.manifest.author) + ' · ' + meta + '</div>';
      }
      html += '</div>';
      html += '<div class="plugin-actions">';
      html += '<label class="toggle-label">';
      html += '<input type="checkbox" class="plugin-toggle" data-plugin="' + esc(p.manifest.id) + '"' +
              (p.enabled ? ' checked' : '') + '>';
      html += '<span class="toggle-slider"></span>';
      html += '</label>';
      html += '</div>';
      html += '</div>'; // plugin-header

      // Config fields (if any)
      var configKeys = Object.keys(p.manifest.config || {});
      if (configKeys.length > 0) {
        html += '<div class="plugin-config" style="' + (p.enabled ? '' : 'display:none') + '">';
        configKeys.forEach(function(key) {
          var field = p.manifest.config[key];
          var val = p.config[key] || field.default || '';
          html += '<div class="plugin-config-row">';
          html += '<label class="plugin-config-label">' + esc(field.label || key);
          if (field.required) html += ' <span style="color:var(--accent)">*</span>';
          html += '</label>';
          if (field.secret) {
            html += '<input type="password" class="plugin-config-input" data-plugin="' +
                    esc(p.manifest.id) + '" data-key="' + esc(key) + '"' +
                    ' placeholder="' + (val === '••••••••' ? '•••••••• (configured)' : 'Enter ' + esc(field.label || key)) + '">';
          } else {
            html += '<input type="text" class="plugin-config-input" data-plugin="' +
                    esc(p.manifest.id) + '" data-key="' + esc(key) + '"' +
                    ' value="' + esc(val) + '" placeholder="Enter ' + esc(field.label || key) + '">';
          }
          html += '</div>';
        });
        html += '</div>'; // plugin-config
      }

      // Test run button
      html += '<div class="plugin-footer" style="' + (p.enabled ? '' : 'display:none') + '">';
      html += '<button class="btn-sm plugin-test-btn" data-plugin="' + esc(p.manifest.id) + '">▶ Test Run</button>';
      html += '<span class="plugin-test-result" id="plugin-result-' + esc(p.manifest.id) + '"></span>';
      html += '</div>';

      html += '</div>'; // plugin-card
    });

    container.innerHTML = html;

    // Bind toggle handlers
    container.querySelectorAll('.plugin-toggle').forEach(function(el) {
      el.addEventListener('change', function() {
        var input = el as HTMLInputElement;
        var pluginId = input.dataset.plugin!;
        var enabled = input.checked;

        fetch('/api/plugins/' + pluginId + '/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ enabled: enabled ? 'true' : 'false' })
        }).then(function() {
          showToast(enabled ? '🧩 Plugin enabled: ' + pluginId : '🧩 Plugin disabled: ' + pluginId, 'success');
          // Show/hide config + footer
          var card = input.closest('.plugin-card') as HTMLElement;
          var config = card.querySelector('.plugin-config') as HTMLElement;
          var footer = card.querySelector('.plugin-footer') as HTMLElement;
          if (config) config.style.display = enabled ? '' : 'none';
          if (footer) footer.style.display = enabled ? '' : 'none';
        }).catch(function() {
          showToast('❌ Failed to update plugin', 'error');
        });
      });
    });

    // Bind config input handlers
    container.querySelectorAll('.plugin-config-input').forEach(function(el) {
      el.addEventListener('change', function() {
        var input = el as HTMLInputElement;
        var pluginId = input.dataset.plugin!;
        var key = input.dataset.key!;
        var val = input.value.trim();
        if (!val) return;

        var body: Record<string, string> = {};
        body[key] = val;

        fetch('/api/plugins/' + pluginId + '/config', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body)
        }).then(function() {
          showToast('🧩 ' + key + ' saved', 'success');
          // Mask secret fields after saving
          if (input.type === 'password') {
            input.value = '';
            input.placeholder = '•••••••• (configured)';
          }
        }).catch(function() {
          showToast('❌ Failed to save config', 'error');
        });
      });
    });

    // Bind test run handlers
    container.querySelectorAll('.plugin-test-btn').forEach(function(el) {
      el.addEventListener('click', function() {
        var btn = el as HTMLButtonElement;
        var pluginId = btn.dataset.plugin!;
        var resultEl = document.getElementById('plugin-result-' + pluginId)!;

        btn.disabled = true;
        btn.textContent = '⏳ Running...';
        resultEl.textContent = '';

        fetch('/api/plugins/' + pluginId + '/run', { method: 'POST' })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) {
              resultEl.textContent = '❌ ' + data.error;
              resultEl.style.color = '#ef4444';
            } else if (data.status === 'ok') {
              var d = data.data || {};
              resultEl.textContent = '✅ ' + (d.label || d.provider || 'OK') +
                (d.usage_pct ? ' — ' + d.usage_pct.toFixed(1) + '%' : '') +
                (d.usage_display ? ' (' + d.usage_display + ')' : '');
              resultEl.style.color = '#22c55e';
            } else {
              resultEl.textContent = '⚠️ ' + (data.error || 'Unknown response');
              resultEl.style.color = '#f59e0b';
            }
          })
          .catch(function() {
            resultEl.textContent = '❌ Network error';
            resultEl.style.color = '#ef4444';
          })
          .finally(function() {
            btn.disabled = false;
            btn.textContent = '▶ Test Run';
          });
      });
    });

  }).catch(function() {});
}


// ════════════════════════════════════════════
