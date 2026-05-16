// Niyantra Dashboard — Activity Heatmap (F6)
// GitHub-style contribution calendar showing daily snapshot activity across all providers.

export function loadHeatmap(): void {
  var container = document.getElementById('heatmap-container');
  if (!container) return;

  fetch('/api/history/heatmap?days=365').then(function(res) { return res.json(); }).then(function(data) {
    if (!data) {
      container!.innerHTML = '';
      return;
    }
    renderHeatmap(container!, data);
  }).catch(function(err) {
    console.error('Heatmap fetch failed:', err);
    container!.innerHTML = '';
  });
}

interface HeatmapDay {
  date: string;
  count: number;
  antigravity: number;
  claude: number;
  codex: number;
  cursor: number;
  gemini: number;
  copilot: number;
}

interface HeatmapData {
  days: HeatmapDay[];
  maxCount: number;
  totalSnapshots: number;
  activeDays: number;
  streak: number;
  longestStreak: number;
}

function renderHeatmap(container: HTMLElement, data: HeatmapData): void {
  var days = data.days || [];
  var maxCount = data.maxCount || 1;

  // Build lookup map: "YYYY-MM-DD" → HeatmapDay
  var dayMap: Record<string, HeatmapDay> = {};
  for (var i = 0; i < days.length; i++) {
    dayMap[days[i].date] = days[i];
  }

  // Generate 52 weeks of cells ending at today
  var today = new Date();
  // Find the start: go back ~52 weeks, then align to the nearest Sunday
  var startDate = new Date(today);
  startDate.setDate(startDate.getDate() - 364); // 52 * 7 - 1 = 363 days back + today
  // Align to Sunday (start of week)
  var dayOfWeek = startDate.getDay();
  startDate.setDate(startDate.getDate() - dayOfWeek);

  // Calculate total days from startDate to today
  var totalDays = Math.ceil((today.getTime() - startDate.getTime()) / (1000 * 60 * 60 * 24)) + 1;
  var totalWeeks = Math.ceil(totalDays / 7);

  // Generate month labels
  var monthLabels: { label: string; col: number }[] = [];
  var lastMonth = -1;
  var monthNames = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

  // Build grid cells
  var cellsHTML = '';
  for (var w = 0; w < totalWeeks; w++) {
    for (var d = 0; d < 7; d++) {
      var cellDate = new Date(startDate);
      cellDate.setDate(startDate.getDate() + w * 7 + d);

      // Don't render future dates
      if (cellDate > today) {
        cellsHTML += '<div class="heatmap-cell heatmap-empty"></div>';
        continue;
      }

      var dateStr = formatDateISO(cellDate);
      var entry = dayMap[dateStr];
      var count = entry ? entry.count : 0;
      var level = getIntensityLevel(count, maxCount);

      // Track month transitions for labels
      var month = cellDate.getMonth();
      if (month !== lastMonth && d === 0) {
        monthLabels.push({ label: monthNames[month], col: w });
        lastMonth = month;
      }

      // Build tooltip text
      var tooltip = formatDateHuman(cellDate) + ': ';
      if (count === 0) {
        tooltip += 'No activity';
      } else {
        tooltip += count + ' snapshot' + (count !== 1 ? 's' : '');
        if (entry) {
          var parts: string[] = [];
          if (entry.antigravity > 0) parts.push(entry.antigravity + ' AG');
          if (entry.claude > 0) parts.push(entry.claude + ' Claude');
          if (entry.codex > 0) parts.push(entry.codex + ' Codex');
          if (entry.cursor > 0) parts.push(entry.cursor + ' Cursor');
          if (entry.gemini > 0) parts.push(entry.gemini + ' Gemini');
          if (entry.copilot > 0) parts.push(entry.copilot + ' Copilot');
          if (parts.length > 0) tooltip += ' (' + parts.join(', ') + ')';
        }
      }

      cellsHTML += '<div class="heatmap-cell heatmap-level-' + level + '" ' +
        'data-date="' + dateStr + '" ' +
        'data-count="' + count + '" ' +
        'aria-label="' + tooltip + '" ' +
        'title="' + tooltip + '"></div>';
    }
  }

  // Month label row
  var monthLabelHTML = '<div class="heatmap-month-labels" style="grid-template-columns: 28px repeat(' + totalWeeks + ', 1fr)">';
  monthLabelHTML += '<div></div>'; // spacer for day labels
  var lastCol = -2;
  for (var m = 0; m < monthLabels.length; m++) {
    // Only show if there's enough space from previous label
    if (monthLabels[m].col > lastCol + 2) {
      monthLabelHTML += '<div class="heatmap-month" style="grid-column: ' + (monthLabels[m].col + 2) + '">' + monthLabels[m].label + '</div>';
      lastCol = monthLabels[m].col;
    }
  }
  monthLabelHTML += '</div>';

  // Stats bar
  var statsHTML = '<div class="heatmap-stats">' +
    '<span class="heatmap-stat">' +
      '<span class="heatmap-stat-value">' + data.totalSnapshots + '</span>' +
      '<span class="heatmap-stat-label">snapshots</span>' +
    '</span>' +
    '<span class="heatmap-stat">' +
      '<span class="heatmap-stat-value">' + data.activeDays + '</span>' +
      '<span class="heatmap-stat-label">active days</span>' +
    '</span>' +
    '<span class="heatmap-stat">' +
      '<span class="heatmap-stat-value">' + data.streak + 'd</span>' +
      '<span class="heatmap-stat-label">current streak</span>' +
    '</span>' +
    '<span class="heatmap-stat">' +
      '<span class="heatmap-stat-value">' + data.longestStreak + 'd</span>' +
      '<span class="heatmap-stat-label">longest streak</span>' +
    '</span>' +
  '</div>';

  // Legend
  var legendHTML = '<div class="heatmap-legend">' +
    '<span class="heatmap-legend-label">Less</span>' +
    '<div class="heatmap-cell heatmap-level-0 heatmap-legend-cell"></div>' +
    '<div class="heatmap-cell heatmap-level-1 heatmap-legend-cell"></div>' +
    '<div class="heatmap-cell heatmap-level-2 heatmap-legend-cell"></div>' +
    '<div class="heatmap-cell heatmap-level-3 heatmap-legend-cell"></div>' +
    '<div class="heatmap-cell heatmap-level-4 heatmap-legend-cell"></div>' +
    '<span class="heatmap-legend-label">More</span>' +
  '</div>';

  // Day labels + grid
  var dayLabels = '<div class="heatmap-day-labels">' +
    '<div></div>' +  // empty for alignment
    '<div class="heatmap-day-label">Mon</div>' +
    '<div></div>' +
    '<div class="heatmap-day-label">Wed</div>' +
    '<div></div>' +
    '<div class="heatmap-day-label">Fri</div>' +
    '<div></div>' +
  '</div>';

  var gridHTML = '<div class="heatmap-scroll">' +
    '<div class="heatmap-body">' +
      dayLabels +
      '<div class="heatmap-grid" style="grid-template-columns: repeat(' + totalWeeks + ', 1fr)">' +
        cellsHTML +
      '</div>' +
    '</div>' +
  '</div>';

  container.innerHTML =
    '<h3>Activity</h3>' +
    statsHTML +
    monthLabelHTML +
    gridHTML +
    '<div class="heatmap-footer">' + legendHTML + '</div>';
}

function getIntensityLevel(count: number, max: number): number {
  if (count === 0) return 0;
  if (max <= 1) return 4; // only 1 snap → max intensity
  var ratio = count / max;
  if (ratio <= 0.25) return 1;
  if (ratio <= 0.50) return 2;
  if (ratio <= 0.75) return 3;
  return 4;
}

function formatDateISO(d: Date): string {
  var y = d.getFullYear();
  var m = (d.getMonth() + 1).toString().padStart(2, '0');
  var day = d.getDate().toString().padStart(2, '0');
  return y + '-' + m + '-' + day;
}

function formatDateHuman(d: Date): string {
  var months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  var days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  return days[d.getDay()] + ', ' + months[d.getMonth()] + ' ' + d.getDate() + ', ' + d.getFullYear();
}
