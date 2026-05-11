// Niyantra Dashboard — API Functions
// Fetch wrappers for all backend endpoints.

import { setUsageDataCache } from './state.js';

// ── Quotas (auto-tracked) ──

export function fetchStatus() {
  return fetch('/api/status').then(function(res) {
    if (!res.ok) throw new Error('Failed to fetch status');
    return res.json();
  });
}

export function triggerSnap() {
  return fetch('/api/snap', { method: 'POST' }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Snap failed');
      return data;
    });
  });
}

// ── Subscriptions ──

export function fetchSubscriptions(status, category) {
  var params = new URLSearchParams();
  if (status) params.set('status', status);
  if (category) params.set('category', category);
  var url = '/api/subscriptions' + (params.toString() ? '?' + params : '');
  return fetch(url).then(function(res) { return res.json(); });
}

export function createSubscription(sub) {
  return fetch('/api/subscriptions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub),
  }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Create failed');
      return data;
    });
  });
}

export function updateSubscription(id, sub) {
  return fetch('/api/subscriptions/' + id, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub),
  }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Update failed');
      return data;
    });
  });
}

export function deleteSubscription(id) {
  return fetch('/api/subscriptions/' + id, { method: 'DELETE' }).then(function(res) {
    return res.json().then(function(data) {
      if (!res.ok) throw new Error(data.error || 'Delete failed');
      return data;
    });
  });
}

// ── Overview & Presets ──

export function fetchOverview() {
  return fetch('/api/overview').then(function(res) { return res.json(); });
}

export function fetchPresets() {
  return fetch('/api/presets').then(function(res) { return res.json(); });
}

// ── Usage Intelligence ──

export function fetchUsage(accountId) {
  var url = '/api/usage';
  if (accountId) url += '?account=' + accountId;
  return fetch(url).then(function(res) { return res.json(); }).then(function(data) {
    setUsageDataCache(data);
    return data;
  });
}
