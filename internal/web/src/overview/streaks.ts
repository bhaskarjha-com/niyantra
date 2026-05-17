// Niyantra Dashboard — Usage Streak Hero Card (F4-UX)
// Prominent display of usage streak, total snapshots, and activity score.
// Data comes from /api/history/heatmap (already computed server-side).

export interface StreakData {
  streak: number;
  longestStreak: number;
  totalSnapshots: number;
  activeDays: number;
}

export function renderStreakCard(data: StreakData | null): string {
  if (!data || data.totalSnapshots === 0) return '';

  var fireEmojis = data.streak >= 30 ? '🔥🔥🔥' :
                   data.streak >= 14 ? '🔥🔥' :
                   data.streak >= 7  ? '🔥' :
                   data.streak >= 1  ? '🔥' : '';

  var streakLabel = data.streak === 0 ? 'No active streak — take a snapshot today!' :
    data.streak + '-day streak ' + fireEmojis;

  return '<div class="streak-card">' +
    '<div class="streak-main">' +
      '<div class="streak-number">' + (data.streak || 0) + '</div>' +
      '<div class="streak-label">' + streakLabel + '</div>' +
    '</div>' +
    '<div class="streak-stats">' +
      '<div class="streak-stat">' +
        '<span class="streak-stat-val">' + formatCount(data.totalSnapshots) + '</span>' +
        '<span class="streak-stat-label">Total Snaps</span>' +
      '</div>' +
      '<div class="streak-stat">' +
        '<span class="streak-stat-val">' + data.activeDays + '</span>' +
        '<span class="streak-stat-label">Active Days</span>' +
      '</div>' +
      '<div class="streak-stat">' +
        '<span class="streak-stat-val">' + data.longestStreak + '</span>' +
        '<span class="streak-stat-label">Best Streak</span>' +
      '</div>' +
    '</div>' +
    '</div>';
}

function formatCount(n: number): string {
  if (n >= 10000) return (n / 1000).toFixed(1) + 'k';
  if (n >= 1000) return (n / 1000).toFixed(1) + 'k';
  return n.toString();
}
