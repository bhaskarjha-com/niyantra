// Niyantra Dashboard — Budget Threshold
import { serverConfig } from '../core/state';
import { showToast } from '../core/utils';



export function getBudget(): number {
  return parseFloat(serverConfig['budget_monthly'] || '0');
}

export function setBudget(amount: number): void {
  serverConfig['budget_monthly'] = amount.toString();
  updateConfig('budget_monthly', amount.toString());
}

export function getCurrency(): string {
  return serverConfig['currency'] || 'USD';
}

export function updateConfig(key: string, value: string): Promise<any> {
  return fetch('/api/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ key: key, value: value })
  }).then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.config) {
      
    }
  }).catch(function(err) { console.error('Config update failed:', err); });
}

export function loadConfig(): Promise<void> {
  return fetch('/api/config').then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.config) {
      data.config.forEach(function(c: any) { serverConfig[c.key] = c.value; });
    }
    // serverConfig updated in-place
  });
}

export function initBudget(): void {
  document.getElementById('budget-close')!.addEventListener('click', closeBudget);
  document.getElementById('budget-cancel')!.addEventListener('click', closeBudget);
  document.getElementById('budget-overlay')!.addEventListener('click', function(e) {
    if ((e.target as HTMLElement).id === 'budget-overlay') closeBudget();
  });
  document.getElementById('budget-save')!.addEventListener('click', function() {
    var val = parseFloat((document.getElementById('f-budget') as HTMLInputElement).value) || 0;
    setBudget(val);
    closeBudget();
    showToast('✅ Budget set to $' + val.toFixed(0) + '/mo', 'success');
    // Refresh overview if visible
    var overviewPanel = document.getElementById('panel-overview');
    if (overviewPanel && overviewPanel.classList.contains('active')) document.dispatchEvent(new CustomEvent('niyantra:overview-refresh'));
  });
}

export function openBudgetModal(): void {
  (document.getElementById('f-budget') as HTMLInputElement).value = String(getBudget() || '');
  document.getElementById('budget-overlay')!.hidden = false;
}

export function closeBudget(): void {
  document.getElementById('budget-overlay')!.hidden = true;
}

export function renderBudgetAlert(totalMonthly: number): string {
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

