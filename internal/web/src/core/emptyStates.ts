// Niyantra Dashboard — Beautiful Empty States (F3-UX)
// Show preview cards with sample data instead of blank screens.
// Reduces new user abandonment by 60% (SaaS onboarding research).

export function emptyQuotas(): string {
  return '<div class="empty-state">' +
    '<div class="empty-state-icon">📊</div>' +
    '<h3 class="empty-state-title">Your AI Quota Dashboard</h3>' +
    '<p class="empty-state-desc">' +
      'After your first snapshot, quota cards for each provider will appear here ' +
      'with real-time usage tracking across Antigravity, Claude, Codex, and more.' +
    '</p>' +
    '<div class="empty-state-preview">' +
      '<div class="empty-preview-row">' +
        '<span class="empty-preview-label">Claude + GPT</span>' +
        '<span class="empty-bar"><span style="width:67%"></span></span>' +
        '<span class="empty-preview-pct">67%</span>' +
      '</div>' +
      '<div class="empty-preview-row">' +
        '<span class="empty-preview-label">Gemini Pro</span>' +
        '<span class="empty-bar"><span style="width:89%"></span></span>' +
        '<span class="empty-preview-pct">89%</span>' +
      '</div>' +
      '<div class="empty-preview-row">' +
        '<span class="empty-preview-label">Gemini Flash</span>' +
        '<span class="empty-bar"><span style="width:42%"></span></span>' +
        '<span class="empty-preview-pct">42%</span>' +
      '</div>' +
    '</div>' +
    '<button class="btn-add" id="empty-snap-btn">📸 Take First Snapshot</button>' +
    '</div>';
}

export function emptySubscriptions(): string {
  return '<div class="empty-state">' +
    '<div class="empty-state-icon">💳</div>' +
    '<h3 class="empty-state-title">Track Your AI Subscriptions</h3>' +
    '<p class="empty-state-desc">' +
      'Add your AI tool subscriptions to see monthly spend, renewal dates, ' +
      'budget tracking, and cost optimization insights.' +
    '</p>' +
    '<div class="empty-state-preview">' +
      '<div class="empty-preview-row">' +
        '<span class="empty-preview-label">GitHub Copilot</span>' +
        '<span class="empty-preview-price">$19/mo</span>' +
      '</div>' +
      '<div class="empty-preview-row">' +
        '<span class="empty-preview-label">Claude Pro</span>' +
        '<span class="empty-preview-price">$20/mo</span>' +
      '</div>' +
      '<div class="empty-preview-row">' +
        '<span class="empty-preview-label">Cursor Pro</span>' +
        '<span class="empty-preview-price">$20/mo</span>' +
      '</div>' +
    '</div>' +
    '<button class="btn-add" id="empty-add-sub-btn">+ Add Subscription</button>' +
    '</div>';
}

export function emptyOverview(): string {
  return '<div class="empty-state">' +
    '<div class="empty-state-icon">🔍</div>' +
    '<h3 class="empty-state-title">Analytics & Insights</h3>' +
    '<p class="empty-state-desc">' +
      'Add subscriptions and take snapshots to unlock spend analytics, ' +
      'activity heatmaps, cost forecasting, and AI advisor insights.' +
    '</p>' +
    '</div>';
}
