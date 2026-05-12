// Niyantra Dashboard — API Functions
// Fetch wrappers for all backend endpoints.

import { setUsageDataCache } from './state';
import type { StatusResponse, Subscription, OverviewResponse, PresetsResponse } from '../types/api';

// ── Quotas (auto-tracked) ──

export function fetchStatus(): Promise<StatusResponse> {
  return fetch('/api/status').then(function(res) {
    if (!res.ok) throw new Error('Failed to fetch status');
    return res.json();
  });
}

export function triggerSnap(): Promise<any> {
  return fetch('/api/snap', { method: 'POST' }).then(function(res) {
    return res.json().then(function(data: any) {
      if (!res.ok) throw new Error(data.error || 'Snap failed');
      return data;
    });
  });
}

// ── Subscriptions ──

export function fetchSubscriptions(status?: string, category?: string): Promise<Subscription[]> {
  var params = new URLSearchParams();
  if (status) params.set('status', status);
  if (category) params.set('category', category);
  var url = '/api/subscriptions' + (params.toString() ? '?' + params : '');
  return fetch(url).then(function(res) { return res.json(); });
}

export function createSubscription(sub: Partial<Subscription>): Promise<Subscription> {
  return fetch('/api/subscriptions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub),
  }).then(function(res) {
    return res.json().then(function(data: any) {
      if (!res.ok) throw new Error(data.error || 'Create failed');
      return data;
    });
  });
}

export function updateSubscription(id: number, sub: Partial<Subscription>): Promise<Subscription> {
  return fetch('/api/subscriptions/' + id, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(sub),
  }).then(function(res) {
    return res.json().then(function(data: any) {
      if (!res.ok) throw new Error(data.error || 'Update failed');
      return data;
    });
  });
}

export function deleteSubscription(id: number): Promise<any> {
  return fetch('/api/subscriptions/' + id, { method: 'DELETE' }).then(function(res) {
    return res.json().then(function(data: any) {
      if (!res.ok) throw new Error(data.error || 'Delete failed');
      return data;
    });
  });
}

// ── Overview & Presets ──

export function fetchOverview(): Promise<OverviewResponse> {
  return fetch('/api/overview').then(function(res) { return res.json(); });
}

export function fetchPresets(): Promise<PresetsResponse> {
  return fetch('/api/presets').then(function(res) { return res.json(); });
}

// ── Usage Intelligence ──

export function fetchUsage(accountId?: number): Promise<any> {
  var url = '/api/usage';
  if (accountId) url += '?account=' + accountId;
  return fetch(url).then(function(res) { return res.json(); }).then(function(data: any) {
    setUsageDataCache(data);
    return data;
  });
}
