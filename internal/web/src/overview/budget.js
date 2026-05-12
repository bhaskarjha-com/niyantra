// Niyantra Dashboard — Budget Threshold
import { serverConfig } from '../core/state.js';
import { showToast } from '../core/utils.js';



export function getBudget() {
  return parseFloat(serverConfig['budget_monthly'] || '0');
}

export function setBudget(amount) {
  serverConfig['budget_monthly'] = amount.toString();
  updateConfig('budget_monthly', amount.toString());
}

export function getCurrency() {
  return serverConfig['currency'] || 'USD';
}

export function updateConfig(key, value) {
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

export function loadConfig() {
  return fetch('/api/config').then(function(r) { return r.json(); })
  .then(function(data) {
    if (data.config) {
      data.config.forEach(function(c) { serverConfig[c.key] = c.value; });
    }
    return serverConfig;
  });
}

export function initBudget() {
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

export function openBudgetModal() {
  document.getElementById('f-budget').value = getBudget() || '';
  document.getElementById('budget-overlay').hidden = false;
}

export function closeBudget() {
  document.getElementById('budget-overlay').hidden = true;
}

export function renderBudgetAlert(totalMonthly) {
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

