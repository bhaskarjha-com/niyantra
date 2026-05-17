// Niyantra Dashboard — Chart Annotations (F19-UX)
// Renders event markers on the history chart timeline.
// Uses activity log data to mark config changes, account additions,
// subscription events on the chart.

export interface ChartAnnotation {
  date: string;      // ISO date string
  type: string;      // event type
  icon: string;      // emoji icon
  color: string;     // CSS color
  label: string;     // short description
  tooltip: string;   // full description for hover
}

// Map event types to visual properties
function getAnnotationMeta(type: string): { icon: string; color: string } {
  switch(type) {
    case 'config_change':          return { icon: '⚙️', color: '#6366f1' };
    case 'account_added':          return { icon: '➕', color: '#10b981' };
    case 'account_removed':        return { icon: '➖', color: '#ef4444' };
    case 'subscription_created':   return { icon: '💳', color: '#f59e0b' };
    case 'subscription_updated':   return { icon: '✏️', color: '#8b5cf6' };
    case 'subscription_deleted':   return { icon: '❌', color: '#ef4444' };
    case 'budget_changed':         return { icon: '💰', color: '#f59e0b' };
    case 'notification':           return { icon: '🔔', color: '#ec4899' };
    case 'quota_alert':            return { icon: '⚠️', color: '#ef4444' };
    default:                       return { icon: '📌', color: '#94a3b8' };
  }
}

// High-frequency events to skip (too noisy for chart annotations)
var SKIP_EVENTS: Record<string, boolean> = {
  'snapshot': true,
  'auto_capture': true,
  'snap': true,
  'poll_cycle': true,
};

// Parse activity events into chart annotations
function parseAnnotations(entries: any[]): ChartAnnotation[] {
  var annotations: ChartAnnotation[] = [];

  for (var i = 0; i < entries.length; i++) {
    var e = entries[i];
    if (SKIP_EVENTS[e.eventType]) continue;

    var meta = getAnnotationMeta(e.eventType);

    // Parse details for richer labels
    var label = e.eventType.replace(/_/g, ' ');
    var details: any = {};
    try { details = JSON.parse(e.details || '{}'); } catch(ex) {}

    if (details.key) {
      label = details.key + ' → ' + (details.value || '');
    } else if (e.accountEmail) {
      label += ': ' + e.accountEmail;
    }

    // Truncate label
    if (label.length > 40) label = label.substring(0, 37) + '…';

    annotations.push({
      date: e.timestamp,
      type: e.eventType,
      icon: meta.icon,
      color: meta.color,
      label: label,
      tooltip: meta.icon + ' ' + label + '\n' + new Date(e.timestamp).toLocaleString()
    });
  }

  return annotations;
}

// Render annotation markers as an overlay on the chart container
export function renderChartAnnotations(
  chartContainer: HTMLElement,
  annotations: ChartAnnotation[],
  chartLabels: string[],
  chartInstance: any
): void {
  // Remove existing annotations
  var existing = chartContainer.querySelectorAll('.chart-annotation');
  for (var i = 0; i < existing.length; i++) existing[i].remove();

  if (!annotations.length || !chartInstance) return;

  // Limit to 10 most significant
  var visible = annotations.slice(0, 10);

  // Get chart area
  var chartArea = chartInstance.chartArea;
  if (!chartArea) return;

  for (var j = 0; j < visible.length; j++) {
    var ann = visible[j];
    var annDate = new Date(ann.date);

    // Find closest label index
    var bestIdx = -1;
    var bestDiff = Infinity;
    for (var k = 0; k < chartLabels.length; k++) {
      // Labels are formatted dates — match by finding closest timestamp
      var labelDate = new Date(chartLabels[k]);
      if (isNaN(labelDate.getTime())) continue;
      var diff = Math.abs(labelDate.getTime() - annDate.getTime());
      if (diff < bestDiff) {
        bestDiff = diff;
        bestIdx = k;
      }
    }

    if (bestIdx < 0) continue;

    // Get x coordinate from chart
    var x = chartInstance.scales.x.getPixelForValue(bestIdx);
    if (x < chartArea.left || x > chartArea.right) continue;

    // Create marker element
    var marker = document.createElement('div');
    marker.className = 'chart-annotation';
    marker.style.left = x + 'px';
    marker.style.top = chartArea.top + 'px';
    marker.style.height = (chartArea.bottom - chartArea.top) + 'px';
    marker.style.borderLeftColor = ann.color;

    // Icon dot at top
    var dot = document.createElement('span');
    dot.className = 'chart-annotation-dot';
    dot.textContent = ann.icon;
    marker.appendChild(dot);

    // Tooltip
    var tip = document.createElement('div');
    tip.className = 'chart-annotation-tooltip';
    tip.textContent = ann.label;
    marker.appendChild(tip);

    chartContainer.appendChild(marker);
  }
}

// Fetch activity and render annotations on the history chart
export function loadChartAnnotations(
  chartContainer: HTMLElement,
  chartLabels: string[],
  chartInstance: any
): void {
  fetch('/api/activity?limit=100').then(function(r) {
    return r.json();
  }).then(function(data: any) {
    if (!data || !data.entries || data.entries.length === 0) return;
    var annotations = parseAnnotations(data.entries);
    if (annotations.length > 0) {
      renderChartAnnotations(chartContainer, annotations, chartLabels, chartInstance);
    }
  }).catch(function(err) {
    console.error('Chart annotations failed:', err);
  });
}
