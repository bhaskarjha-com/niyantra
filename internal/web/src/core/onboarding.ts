// Niyantra Dashboard — Onboarding Checklist (F7-UX)
// Persistent 5-step checklist for new users.
// Uses localStorage to track completion — no backend needed.
// Auto-detects milestone completion via existing API data.

var STORAGE_KEY = 'niyantra_onboarding';

interface OnboardingState {
  dismissed: boolean;
  completed: Record<string, boolean>;
}

function getState(): OnboardingState {
  try {
    var raw = localStorage.getItem(STORAGE_KEY);
    if (raw) return JSON.parse(raw) as OnboardingState;
  } catch { /* ignore */ }
  return { dismissed: false, completed: {} };
}

function saveState(state: OnboardingState): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
}

export function checkOnboardingStep(step: string): void {
  var state = getState();
  if (state.dismissed) return;
  if (state.completed[step]) return; // already done
  state.completed[step] = true;
  saveState(state);
  renderOnboarding();
}

export function renderOnboarding(): void {
  var container = document.getElementById('onboarding-container');
  if (!container) return;

  var state = getState();
  if (state.dismissed) { container.innerHTML = ''; return; }

  var steps = [
    { key: 'snapshot', label: 'Take your first snapshot', icon: '📸' },
    { key: 'subscription', label: 'Add a subscription', icon: '💳' },
    { key: 'budget', label: 'Set a monthly budget', icon: '💰' },
    { key: 'notifications', label: 'Enable notifications', icon: '🔔' },
    { key: 'overview', label: 'Explore the Overview tab', icon: '📊' },
  ];

  var done = 0;
  for (var i = 0; i < steps.length; i++) {
    if (state.completed[steps[i].key]) done++;
  }

  // All done — celebrate, then auto-dismiss
  if (done === steps.length) {
    container.innerHTML = '<div class="onboarding-card celebration">' +
      '<div class="onboarding-confetti">🎉</div>' +
      '<h3>You\'re all set!</h3>' +
      '<p style="color:var(--text-secondary);font-size:13px">Niyantra is fully configured. Enjoy your AI dashboard.</p>' +
      '</div>';
    setTimeout(function() {
      state.dismissed = true;
      saveState(state);
      if (container) container.innerHTML = '';
    }, 5000);
    return;
  }

  var pct = Math.round((done / steps.length) * 100);
  var html = '<div class="onboarding-card">' +
    '<div class="onboarding-header">' +
      '<h3>🚀 Getting Started</h3>' +
      '<button class="onboarding-dismiss" id="onboarding-dismiss" title="Dismiss">✕</button>' +
    '</div>' +
    '<div class="onboarding-progress">' +
      '<div class="onboarding-bar"><div class="onboarding-fill" style="width:' + pct + '%"></div></div>' +
      '<span class="onboarding-pct">' + done + '/' + steps.length + '</span>' +
    '</div>' +
    '<div class="onboarding-steps">';

  for (var s = 0; s < steps.length; s++) {
    var step = steps[s];
    var isDone = state.completed[step.key] || false;
    html += '<div class="onboarding-step' + (isDone ? ' done' : '') + '">' +
      '<span class="onboarding-check">' + (isDone ? '✅' : '⬜') + '</span>' +
      '<span>' + step.icon + ' ' + step.label + '</span>' +
      '</div>';
  }

  html += '</div></div>';
  container.innerHTML = html;

  // Wire dismiss button
  var btn = document.getElementById('onboarding-dismiss');
  if (btn) {
    btn.addEventListener('click', function() {
      state.dismissed = true;
      saveState(state);
      if (container) container.innerHTML = '';
    });
  }
}

// Auto-detect completed steps from API data
export function autoDetectSteps(statusData: any, serverConfig: Record<string, string>): void {
  if (statusData && (statusData.snapshotCount || 0) > 0) {
    checkOnboardingStep('snapshot');
  }
  var budget = parseFloat(serverConfig['budget_monthly'] || '0');
  if (budget > 0) {
    checkOnboardingStep('budget');
  }
  if (serverConfig['smtp_enabled'] === 'true' ||
      serverConfig['webhook_enabled'] === 'true' ||
      serverConfig['webpush_enabled'] === 'true') {
    checkOnboardingStep('notifications');
  }
}
