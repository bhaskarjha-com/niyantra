// Niyantra Dashboard — SVG Sparkline Renderer (F2-UX)
// Pure SVG micro-charts, no dependencies.
// Shows 7-point trend line inline with KPI values.

export function sparkline(
  data: number[],
  opts?: { width?: number; height?: number; color?: string; direction?: string }
): string {
  var w = (opts && opts.width) || 60;
  var h = (opts && opts.height) || 20;
  var color = (opts && opts.color) || 'var(--accent)';
  var dir = (opts && opts.direction) || 'flat';

  if (!data || data.length < 2) {
    return '<span class="sparkline-container"><svg width="' + w + '" height="' + h + '"></svg></span>';
  }

  var min = Math.min.apply(null, data);
  var max = Math.max.apply(null, data);
  var range = max - min || 1;
  var pad = 2;

  var points = '';
  var lastX = 0;
  var lastY = 0;
  for (var i = 0; i < data.length; i++) {
    var x = pad + (i / (data.length - 1)) * (w - 2 * pad);
    var y = h - pad - ((data[i] - min) / range) * (h - 2 * pad);
    points += x.toFixed(1) + ',' + y.toFixed(1) + ' ';
    lastX = x;
    lastY = y;
  }

  // Direction arrow
  var arrowColor = dir === 'up' ? 'var(--green)' : dir === 'down' ? 'var(--red)' : 'var(--text-muted)';
  var arrow = dir === 'up' ? '↑' : dir === 'down' ? '↓' : '→';

  return '<span class="sparkline-container">' +
    '<svg width="' + w + '" height="' + h + '" class="sparkline-svg">' +
    '<polyline points="' + points.trim() + '" fill="none" stroke="' + color + '" ' +
    'stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.85"/>' +
    '<circle cx="' + lastX.toFixed(1) + '" cy="' + lastY.toFixed(1) +
    '" r="2" fill="' + color + '"/>' +
    '</svg>' +
    '<span class="sparkline-arrow" style="color:' + arrowColor + '">' + arrow + '</span>' +
    '</span>';
}

// Determine trend direction from an array of numbers
export function trendDirection(data: number[]): string {
  if (!data || data.length < 2) return 'flat';
  var first = data[0];
  var last = data[data.length - 1];
  if (last > first * 1.05) return 'up';
  if (last < first * 0.95) return 'down';
  return 'flat';
}
