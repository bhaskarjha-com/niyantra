// Niyantra Dashboard — Safe to Spend Guardrail (F1-UX)
// Hero card showing how much budget remains this month.
// Inspired by PocketGuard's "In My Pocket" — the #1 daily engagement driver.
// NOTE: No inline onclick — CSP requires addEventListener.

export function renderSafeToSpend(bf: { monthlyBudget: number; currentSpend: number; onTrack: boolean } | null, currency: string): string {
  if (!bf || bf.monthlyBudget <= 0) {
    return '<div class="safe-to-spend-card no-budget">' +
      '<div class="sts-icon">💰</div>' +
      '<div class="sts-label">Set a monthly AI budget to unlock your Safe to Spend guardrail</div>' +
      '<button class="btn-add-sm" id="sts-set-budget-btn">Set Budget</button>' +
      '</div>';
  }

  var safeAmount = Math.max(0, bf.monthlyBudget - bf.currentSpend);
  var pct = Math.round((bf.currentSpend / bf.monthlyBudget) * 100);
  var now = new Date();
  var daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  var daysLeft = daysInMonth - now.getDate();
  var dayOfMonth = now.getDate();

  // Daily burn rate and projection
  var dailyBurn = dayOfMonth > 0 ? bf.currentSpend / dayOfMonth : 0;
  var projected = dailyBurn * daysInMonth;
  var willExceed = projected > bf.monthlyBudget;

  // Semantic classification
  var cls: string, statusIcon: string, statusText: string;
  if (pct >= 100) {
    cls = 'over';
    statusIcon = '🚨';
    statusText = 'Over budget by ' + sym(currency) + (bf.currentSpend - bf.monthlyBudget).toFixed(2);
  } else if (pct >= 80) {
    cls = 'warning';
    statusIcon = '⚠️';
    statusText = willExceed
      ? 'Projected to exceed by ' + sym(currency) + (projected - bf.monthlyBudget).toFixed(2)
      : daysLeft + ' days left at ' + sym(currency) + dailyBurn.toFixed(2) + '/day';
  } else {
    cls = 'healthy';
    statusIcon = '✅';
    statusText = daysLeft + ' days left · ' + sym(currency) + dailyBurn.toFixed(2) + '/day burn rate';
  }

  var s = sym(currency);

  return '<div class="safe-to-spend-card ' + cls + '">' +
    '<div class="sts-header">' +
      '<span class="sts-title">Safe to Spend</span>' +
      '<button class="sts-edit" id="sts-edit-budget-btn" title="Edit budget">✏️</button>' +
    '</div>' +
    '<div class="sts-amount">' + s + safeAmount.toFixed(2) + '</div>' +
    '<div class="sts-bar-container">' +
      '<div class="sts-bar-fill ' + cls + '" style="width:' + Math.min(pct, 100) + '%"></div>' +
    '</div>' +
    '<div class="sts-details">' +
      '<span>' + statusIcon + ' ' + statusText + '</span>' +
      '<span class="sts-budget">Budget: ' + s + bf.monthlyBudget.toFixed(0) + '/mo</span>' +
    '</div>' +
    '</div>';
}

// Wire up click handlers AFTER the HTML is in the DOM (CSP-safe)
export function wireSafeToSpendButtons(openBudgetFn: () => void): void {
  var editBtn = document.getElementById('sts-edit-budget-btn');
  if (editBtn) editBtn.addEventListener('click', openBudgetFn);

  var setBtn = document.getElementById('sts-set-budget-btn');
  if (setBtn) setBtn.addEventListener('click', openBudgetFn);
}

function sym(currency: string): string {
  if (currency === 'INR') return '₹';
  if (currency === 'EUR') return '€';
  if (currency === 'GBP') return '£';
  return '$';
}
