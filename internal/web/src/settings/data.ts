// Niyantra Dashboard — Data Sources
// @ts-nocheck
import { esc, formatTimeAgo } from '../core/utils';


export function loadDataSources(): void {
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



// ════════════════════════════════════════════
