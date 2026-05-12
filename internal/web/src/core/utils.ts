// Niyantra Dashboard — Utility Functions
// Shared helpers used across all modules.

export function formatSeconds(seconds: number): string {
  seconds = Math.floor(seconds);
  if (seconds <= 0) return 'now';
  var h = Math.floor(seconds / 3600);
  var m = Math.floor((seconds % 3600) / 60);
  if (h >= 24) return Math.floor(h / 24) + 'd ' + (h % 24) + 'h';
  if (h > 0) return h + 'h ' + m + 'm';
  if (m === 0) return '<1m';
  return m + 'm';
}

export function formatCredits(n: number): string {
  if (n >= 1000) return (n / 1000).toFixed(n % 1000 === 0 ? 0 : 1) + 'k';
  return Math.round(n).toString();
}

export function formatNumber(n: number): string {
  if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
  if (n >= 1000) return (n / 1000).toFixed(n % 1000 === 0 ? 0 : 1) + 'k';
  return n.toString();
}

export function currencySymbol(code: string): string {
  var map: Record<string, string> = { USD: '$', EUR: '€', GBP: '£', INR: '₹', CAD: 'C$', AUD: 'A$' };
  return map[code] || code + ' ';
}

export function esc(s: string | null | undefined): string {
  if (!s) return '';
  var d = document.createElement('div');
  d.textContent = s;
  // Also escape quotes for safe use in HTML attributes (M11)
  return d.innerHTML.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

export function showToast(msg: string, type: string): void {
  var el = document.getElementById('toast');
  if (!el) return;
  el.textContent = msg;
  el.className = 'toast ' + type + ' visible';
  el.hidden = false;
  setTimeout(function() {
    el!.classList.remove('visible');
    setTimeout(function() { el!.hidden = true; }, 300);
  }, 3000);
}

// H2: Track last update time for relative display
var lastUpdateTime: Date | null = null;

export function updateTimestamp(): void {
  lastUpdateTime = new Date();
  refreshTimestampDisplay();
}

export function refreshTimestampDisplay(): void {
  var el = document.getElementById('last-updated');
  if (!el || !lastUpdateTime) return;
  var sec = Math.floor((new Date().getTime() - lastUpdateTime.getTime()) / 1000);
  var label: string;
  if (sec < 10) label = 'just now';
  else if (sec < 60) label = sec + 's ago';
  else if (sec < 3600) label = Math.floor(sec / 60) + 'm ago';
  else label = Math.floor(sec / 3600) + 'h ago';
  el.textContent = 'Updated ' + label;
  el.title = lastUpdateTime.toLocaleTimeString(); // absolute on hover
}

export function formatTimeAgo(isoStr: string | null | undefined): string {
  if (!isoStr) return 'never';
  var d = new Date(isoStr);
  var now = new Date();
  var sec = Math.floor((now.getTime() - d.getTime()) / 1000);
  if (sec < 60) return 'just now';
  if (sec < 3600) return Math.floor(sec / 60) + 'm ago';
  if (sec < 86400) return Math.floor(sec / 3600) + 'h ago';
  return Math.floor(sec / 86400) + 'd ago';
}

// F2: Format poll interval seconds into a human-readable label
export function formatPollInterval(seconds: number): string {
  if (seconds >= 3600) return Math.floor(seconds / 3600) + 'h';
  return Math.floor(seconds / 60) + 'm';
}

export function formatDurationSec(sec: number | null | undefined): string {
  if (!sec || sec <= 0) return '0m';
  var h = Math.floor(sec / 3600);
  var m = Math.floor((sec % 3600) / 60);
  if (h > 0) return h + 'h ' + m + 'm';
  return m + 'm';
}
