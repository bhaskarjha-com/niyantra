// Niyantra Dashboard — Model Pricing Config
import { esc, showToast } from '../core/utils';


export var pricingDataCache: any[] | null = null;

export function loadModelPricing(): void {
  fetch('/api/config/pricing').then(function(res) { return res.json(); })
  .then(function(data) {
    pricingDataCache = data.pricing || [];
    renderPricingTable(pricingDataCache!);
  }).catch(function(err) {
    console.error('Failed to load model pricing:', err);
  });
}

export function renderPricingTable(pricing: any[]): void {
  var tbody = document.getElementById('pricing-tbody');
  if (!tbody) return;

  var providerIcons: Record<string, string> = { anthropic: '🟤', openai: '🟢', google: '🔵' };

  var html = '';
  for (var i = 0; i < pricing.length; i++) {
    var p = pricing[i];
    var providerCls = p.provider || 'custom';
    var providerLabel = p.provider ? (p.provider.charAt(0).toUpperCase() + p.provider.slice(1)) : 'Custom';
    var icon = providerIcons[p.provider] || '⚪';

    html += '<tr data-pricing-idx="' + i + '">' +
      '<td><span class="pricing-model-name">' + esc(p.displayName) + '</span></td>' +
      '<td><span class="pricing-provider ' + esc(providerCls) + '">' + icon + ' ' + esc(providerLabel) + '</span></td>' +
      '<td style="text-align:right"><input type="number" class="pricing-input" data-field="inputPer1M" step="0.01" min="0" value="' + p.inputPer1M + '"></td>' +
      '<td style="text-align:right"><input type="number" class="pricing-input" data-field="outputPer1M" step="0.01" min="0" value="' + p.outputPer1M + '"></td>' +
      '<td style="text-align:right"><input type="number" class="pricing-input" data-field="cachePer1M" step="0.001" min="0" value="' + p.cachePer1M + '"></td>' +
      '<td><button class="pricing-delete-btn" data-pricing-del="' + i + '" title="Remove this model">✕</button></td>' +
      '</tr>';
  }

  tbody.innerHTML = html;

  // Wire change handlers on inputs
  tbody.querySelectorAll('.pricing-input').forEach(function(input) {
    input.addEventListener('change', function() {
      var tr = (input as HTMLElement).closest('tr');
      var idx = parseInt((tr as HTMLElement).dataset.pricingIdx!);
      var field = (input as HTMLElement).dataset.field!;
      var val = parseFloat((input as HTMLInputElement).value) || 0;
      if (val < 0) val = 0;
      (input as HTMLInputElement).value = String(val);
      if (pricingDataCache && pricingDataCache[idx]) {
        pricingDataCache[idx][field] = val;
        savePricingFromTable();
      }
    });
  });

  // Wire delete buttons
  tbody.querySelectorAll('.pricing-delete-btn').forEach(function(btn) {
    btn.addEventListener('click', function() {
      var idx = parseInt((btn as HTMLElement).dataset.pricingDel!);
      deletePricingRow(idx);
    });
  });
}

export function savePricingFromTable(): void {
  if (!pricingDataCache || pricingDataCache.length === 0) return;

  fetch('/api/config/pricing', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pricing: pricingDataCache })
  })
  .then(function(res) { return res.json(); })
  .then(function(data) {
    if (data.error) {
      showToast('❌ ' + data.error, 'error');
      return;
    }
    showToast('💰 Pricing saved', 'success');
  })
  .catch(function() { showToast('❌ Failed to save pricing', 'error'); });
}

export function addPricingRow(): void {
  if (!pricingDataCache) pricingDataCache = [];

  var newModel = {
    modelId: 'custom-' + Date.now(),
    displayName: 'New Model',
    provider: 'custom',
    inputPer1M: 1.00,
    outputPer1M: 5.00,
    cachePer1M: 0.10
  };
  pricingDataCache.push(newModel);
  renderPricingTable(pricingDataCache);

  // Focus the name cell of the new row for editing
  var tbody = document.getElementById('pricing-tbody');
  var lastRow = tbody!.lastElementChild as HTMLElement;
  if (lastRow) {
    var nameCell = lastRow.querySelector('.pricing-model-name');
    if (nameCell) {
      // Make name editable inline
      (nameCell as HTMLElement).contentEditable = 'true';
      (nameCell as HTMLElement).focus();
      // Select all text for quick replace
      var range = document.createRange();
      range.selectNodeContents(nameCell!);
      var sel = window.getSelection();
      sel!.removeAllRanges();
      sel!.addRange(range);

      nameCell!.addEventListener('blur', function() {
        (nameCell as HTMLElement).contentEditable = 'false';
        var idx = parseInt(lastRow!.dataset.pricingIdx!);
        var newName = nameCell!.textContent!.trim();
        if (newName && pricingDataCache![idx]) {
          pricingDataCache![idx].displayName = newName;
          pricingDataCache![idx].modelId = newName.toLowerCase().replace(/[^a-z0-9]+/g, '-');
          savePricingFromTable();
        }
      }, { once: true });

      nameCell!.addEventListener('keydown', function(e) {
        if ((e as KeyboardEvent).key === 'Enter') {
          e.preventDefault();
          (nameCell as HTMLElement).blur();
        }
      });
    }
  }

  showToast('💰 New model added — edit the name and prices', 'info');
}

export function deletePricingRow(idx: number): void {
  if (!pricingDataCache || idx < 0 || idx >= pricingDataCache.length) return;

  var name = pricingDataCache[idx].displayName;
  if (!confirm('Remove pricing for "' + name + '"?')) return;

  pricingDataCache.splice(idx, 1);
  renderPricingTable(pricingDataCache);
  savePricingFromTable();
  showToast('🗑️ Removed ' + name, 'success');
}

export function resetPricingDefaults(): void {
  if (!confirm('Reset all model pricing to current market defaults? This will overwrite your custom prices.')) return;

  // Fetch defaults from API by deleting the config key and re-fetching
  // We can't easily get defaults from the backend without a dedicated endpoint,
  // so we'll use the hardcoded defaults matching the backend.
  var defaults = [
    { modelId: 'claude-opus-4.6', displayName: 'Claude Opus 4.6', provider: 'anthropic', inputPer1M: 5.00, outputPer1M: 25.00, cachePer1M: 0.50 },
    { modelId: 'claude-sonnet-4.6', displayName: 'Claude Sonnet 4.6', provider: 'anthropic', inputPer1M: 3.00, outputPer1M: 15.00, cachePer1M: 0.30 },
    { modelId: 'claude-haiku-4.5', displayName: 'Claude Haiku 4.5', provider: 'anthropic', inputPer1M: 1.00, outputPer1M: 5.00, cachePer1M: 0.10 },
    { modelId: 'gpt-4o', displayName: 'GPT-4o', provider: 'openai', inputPer1M: 2.50, outputPer1M: 10.00, cachePer1M: 1.25 },
    { modelId: 'gemini-3.1-pro', displayName: 'Gemini 3.1 Pro', provider: 'google', inputPer1M: 2.00, outputPer1M: 12.00, cachePer1M: 0.50 },
    { modelId: 'gemini-2.5-flash', displayName: 'Gemini 2.5 Flash', provider: 'google', inputPer1M: 0.30, outputPer1M: 2.50, cachePer1M: 0.075 }
  ];

  pricingDataCache = defaults;
  renderPricingTable(pricingDataCache);

  fetch('/api/config/pricing', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ pricing: defaults })
  })
  .then(function(res) { return res.json(); })
  .then(function(data) {
    if (data.error) {
      showToast('❌ ' + data.error, 'error');
      return;
    }
    showToast('↻ Pricing reset to defaults', 'success');
  })
  .catch(function() { showToast('❌ Failed to reset pricing', 'error'); });
}


// ════════════════════════════════════════════
