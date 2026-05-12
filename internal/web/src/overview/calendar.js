// Niyantra Dashboard — Renewal Calendar
import { esc } from '../core/utils.js';
//  Phase 10: RENEWAL CALENDAR
// ════════════════════════════════════════════

export var calendarViewDate = new Date();

export function renderRenewalCalendar(renewals, subs) {
  var container = document.getElementById('renewal-calendar-container');
  if (!container) return;

  // Build a map of date -> [{ platform, category }]
  var renewalMap = {};
  if (renewals) {
    for (var i = 0; i < renewals.length; i++) {
      var r = renewals[i];
      var dateKey = r.nextRenewal; // "YYYY-MM-DD"
      if (!renewalMap[dateKey]) renewalMap[dateKey] = [];
      // Find category for this platform
      var cat = 'other';
      if (subs) {
        for (var s = 0; s < subs.length; s++) {
          if (subs[s].platform === r.platform && subs[s].category) {
            cat = subs[s].category;
            break;
          }
        }
      }
      renewalMap[dateKey].push({ platform: r.platform, category: cat, daysUntil: r.daysUntil });
    }
  }

  var year = calendarViewDate.getFullYear();
  var month = calendarViewDate.getMonth();
  var today = new Date();
  var todayKey = today.getFullYear() + '-' + String(today.getMonth() + 1).padStart(2, '0') + '-' + String(today.getDate()).padStart(2, '0');

  var monthNames = ['January', 'February', 'March', 'April', 'May', 'June',
    'July', 'August', 'September', 'October', 'November', 'December'];
  var dayNames = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

  var firstDay = new Date(year, month, 1).getDay();
  var daysInMonth = new Date(year, month + 1, 0).getDate();
  var prevDays = new Date(year, month, 0).getDate();

  var html = '<div class="calendar-container">' +
    '<div class="calendar-header">' +
    '<h3>📅 Renewal Calendar</h3>' +
    '<div class="calendar-nav">' +
    '<button class="calendar-nav-btn" onclick="calendarNav(-1)">‹</button>' +
    '<span class="calendar-month-label">' + monthNames[month] + ' ' + year + '</span>' +
    '<button class="calendar-nav-btn" onclick="calendarNav(1)">›</button>' +
    '</div></div>';

  // Weekday headers
  html += '<div class="calendar-weekdays">';
  for (var d = 0; d < 7; d++) {
    html += '<div class="calendar-weekday">' + dayNames[d] + '</div>';
  }
  html += '</div>';

  // Calendar grid
  html += '<div class="calendar-grid">';

  // Previous month's trailing days
  for (var p = firstDay - 1; p >= 0; p--) {
    html += '<div class="calendar-day other-month"><span class="calendar-day-num">' + (prevDays - p) + '</span></div>';
  }

  // Current month days
  for (var day = 1; day <= daysInMonth; day++) {
    var dateKey = year + '-' + String(month + 1).padStart(2, '0') + '-' + String(day).padStart(2, '0');
    var isToday = dateKey === todayKey;
    var dayClass = isToday ? 'calendar-day today' : 'calendar-day';
    var events = renewalMap[dateKey];

    html += '<div class="' + dayClass + '"';

    // Tooltip
    if (events && events.length > 0) {
      var tooltipText = events.map(function(e) { return e.platform; }).join(', ');
      html += ' title="' + esc(tooltipText) + '"';
    }
    html += '>';

    html += '<span class="calendar-day-num">' + day + '</span>';

    // Renewal pins
    if (events && events.length > 0) {
      html += '<div class="calendar-pins">';
      for (var e = 0; e < Math.min(events.length, 4); e++) {
        html += '<span class="calendar-pin ' + esc(events[e].category) + '"></span>';
      }
      html += '</div>';
    }
    html += '</div>';
  }

  // Fill remaining cells in last week
  var totalCells = firstDay + daysInMonth;
  var remaining = 7 - (totalCells % 7);
  if (remaining < 7) {
    for (var n = 1; n <= remaining; n++) {
      html += '<div class="calendar-day other-month"><span class="calendar-day-num">' + n + '</span></div>';
    }
  }

  html += '</div>';

  // Legend
  var categories = {};
  for (var key in renewalMap) {
    for (var ci = 0; ci < renewalMap[key].length; ci++) {
      categories[renewalMap[key][ci].category] = true;
    }
  }
  var catKeys = Object.keys(categories);
  if (catKeys.length > 0) {
    html += '<div class="calendar-legend">';
    for (var cl = 0; cl < catKeys.length; cl++) {
      html += '<div class="calendar-legend-item">' +
        '<span class="calendar-legend-dot ' + esc(catKeys[cl]) + '"></span>' +
        esc(catKeys[cl]) + '</div>';
    }
    html += '</div>';
  }

  html += '</div>';
  container.innerHTML = html;
}

export function calendarNav(delta) {
  calendarViewDate.setMonth(calendarViewDate.getMonth() + delta);
  // Re-render with cached data
  var el = document.getElementById('renewal-calendar-container');
  if (el) {
    // Reload overview to get fresh data
    document.dispatchEvent(new CustomEvent('niyantra:overview-refresh'));
  }
}

