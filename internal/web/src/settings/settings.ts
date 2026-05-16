// Niyantra Dashboard — Settings Panel
import { serverConfig, presetsData } from '../core/state';
import { showToast, formatTimeAgo } from '../core/utils';
import { fetchStatus } from '../core/api';
import { renderAccounts } from '../quotas/render';
import { updateConfig, loadConfig, setBudget, getBudget } from '../overview/budget';
import { loadOverview } from '../overview/overview';
import { loadMode } from './mode';
import { loadDataSources } from './data';
import { loadActivityLog } from './activity';
import { loadModelPricing, addPricingRow, resetPricingDefaults } from './pricing';
import { loadClaudeBridgeStatus } from '../advanced/claude';
import { loadCodexSettingsStatus } from '../advanced/codex';
import { loadSubscriptions } from '../subscriptions';
import { updateChartTheme } from '../charts/history';
import { loadPlugins } from './plugins';


export function initSettings(): void {
  var themeEl = document.getElementById('s-theme');

  // Theme stays in localStorage (visual-only)
  var savedTheme = localStorage.getItem('niyantra-theme') || 'dark';
  (themeEl as HTMLSelectElement).value = savedTheme;
  if (!themeEl) return;

  themeEl.addEventListener('change', function() {
    var val = (themeEl as HTMLSelectElement).value;
    if (val === 'system') {
      localStorage.removeItem('niyantra-theme');
      var prefer = window.matchMedia('(prefers-color-scheme: light)').matches ? 'light' : 'dark';
      document.documentElement.setAttribute('data-theme', prefer);
    } else {
      localStorage.setItem('niyantra-theme', val);
      document.documentElement.setAttribute('data-theme', val);
    }
    // M2: Update chart theme in-place instead of destroy+rebuild
    var applied = document.documentElement.getAttribute('data-theme')!;
    updateChartTheme(applied);
  });

  // Load server config and populate settings UI
  loadConfig().then(function() {
    var cfg = serverConfig;
    // Migrate localStorage budget/currency to server config (one-time)
    migrateLocalStorage(cfg);

    // Populate fields from server config
    var budgetEl = document.getElementById('s-budget');
    var currencyEl = document.getElementById('s-currency');
    var autoCaptureEl = document.getElementById('s-auto-capture');
    var autoLinkEl = document.getElementById('s-auto-link');
    var pollEl = document.getElementById('s-poll-interval');
    var retentionEl = document.getElementById('s-retention');

    (budgetEl as HTMLInputElement).value = String(parseFloat(cfg['budget_monthly'] || '0') || '');
    (currencyEl as HTMLSelectElement).value = cfg['currency'] || 'USD';
    (autoCaptureEl as HTMLInputElement).checked = cfg['auto_capture'] === 'true';
    (autoLinkEl as HTMLInputElement).checked = cfg['auto_link_subs'] !== 'false';
    (pollEl as HTMLSelectElement).value = cfg['poll_interval'] || '300';
    (retentionEl as HTMLInputElement).value = cfg['retention_days'] || '365';

    // Show/hide poll interval based on auto_capture
    document.getElementById('poll-interval-row')!.style.display =
      (autoCaptureEl as HTMLInputElement).checked ? '' : 'none';

    // Auto-save handlers
    budgetEl!.addEventListener('change', function() {
      var val = parseFloat((budgetEl as HTMLInputElement).value) || 0;
      setBudget(val);
      if (val > 0) showToast('✅ Budget: $' + val.toFixed(0) + '/mo', 'success');
    });

    currencyEl!.addEventListener('change', function() {
      updateConfig('currency', (currencyEl as HTMLSelectElement).value);
      showToast('✅ Currency: ' + (currencyEl as HTMLSelectElement).value, 'success');
    });

    autoCaptureEl!.addEventListener('change', function() {
      var val = (autoCaptureEl as HTMLInputElement).checked ? 'true' : 'false';
      updateConfig('auto_capture', val).then(function() {
        loadMode();
        showToast((autoCaptureEl as HTMLInputElement).checked ? '🟢 Auto-capture started' : '⏸️ Auto-capture stopped', 'success');
      });
      document.getElementById('poll-interval-row')!.style.display =
        (autoCaptureEl as HTMLInputElement).checked ? '' : 'none';
    });

    autoLinkEl!.addEventListener('change', function() {
      updateConfig('auto_link_subs', (autoLinkEl as HTMLInputElement).checked ? 'true' : 'false');
    });

    pollEl!.addEventListener('change', function() {
      var v = (pollEl as HTMLSelectElement).value;
      updateConfig('poll_interval', v).then(function() {
        // Show human-readable label from the selected option
        var label = (pollEl as HTMLSelectElement).options[(pollEl as HTMLSelectElement).selectedIndex].text;
        showToast('⏱️ Interval updated to ' + label + ' — takes effect on next cycle.', 'success');
        loadMode();
      });
    });

    retentionEl!.addEventListener('change', function() {
      var v = parseInt((retentionEl as HTMLInputElement).value);
      if (v >= 30 && v <= 3650) updateConfig('retention_days', v.toString());
    });

    // ── Phase 9: Claude Code Bridge ──
    var claudeBridgeEl = document.getElementById('s-claude-bridge');
    if (claudeBridgeEl) {
      (claudeBridgeEl as HTMLInputElement).checked = cfg['claude_bridge'] === 'true';
      claudeBridgeEl.addEventListener('change', function() {
        var val = (claudeBridgeEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('claude_bridge', val).then(function() {
          showToast((claudeBridgeEl as HTMLInputElement).checked ? '🔗 Claude Code bridge enabled' : '🔗 Bridge disabled', 'success');
          loadClaudeBridgeStatus();
        });
      });
      loadClaudeBridgeStatus();
    }

    // ── Phase 9: Notifications ──
    var notifyEl = document.getElementById('s-notify-enabled');
    var thresholdEl = document.getElementById('s-notify-threshold');
    var thresholdRow = document.getElementById('notify-threshold-row');
    var testRow = document.getElementById('notify-test-row');
    if (notifyEl) {
      (notifyEl as HTMLInputElement).checked = cfg['notify_enabled'] === 'true';
      (thresholdEl as HTMLInputElement).value = cfg['notify_threshold'] || '10';
      thresholdRow!.style.display = (notifyEl as HTMLInputElement).checked ? '' : 'none';
      testRow!.style.display = (notifyEl as HTMLInputElement).checked ? '' : 'none';

      notifyEl.addEventListener('change', function() {
        var val = (notifyEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('notify_enabled', val).then(function() {
          showToast((notifyEl as HTMLInputElement).checked ? '🔔 Notifications enabled' : '🔕 Notifications disabled', 'success');
        });
        thresholdRow!.style.display = (notifyEl as HTMLInputElement).checked ? '' : 'none';
        testRow!.style.display = (notifyEl as HTMLInputElement).checked ? '' : 'none';
      });

      thresholdEl!.addEventListener('change', function() {
        var v = parseInt((thresholdEl as HTMLInputElement).value);
        if (v >= 5 && v <= 50) {
          updateConfig('notify_threshold', v.toString());
          showToast('🔔 Threshold: ' + v + '%', 'success');
        }
      });

      document.getElementById('notify-test-btn')!.addEventListener('click', function() {
        fetch('/api/notify/test', { method: 'POST' })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) showToast('❌ ' + data.error, 'error');
            else showToast('🔔 Test notification sent!', 'success');
          })
          .catch(function() { showToast('❌ Failed to send test', 'error'); });
      });
    }

    // ── F11: SMTP Email Notifications ──
    var smtpEnabledEl = document.getElementById('s-smtp-enabled');
    var smtpConfigRows = document.getElementById('smtp-config-rows');
    if (smtpEnabledEl) {
      (smtpEnabledEl as HTMLInputElement).checked = cfg['smtp_enabled'] === 'true';
      smtpConfigRows!.style.display = (smtpEnabledEl as HTMLInputElement).checked ? '' : 'none';

      // Populate SMTP fields from config
      var smtpHostEl = document.getElementById('s-smtp-host') as HTMLInputElement;
      var smtpPortEl = document.getElementById('s-smtp-port') as HTMLSelectElement;
      var smtpTlsEl = document.getElementById('s-smtp-tls') as HTMLSelectElement;
      var smtpUserEl = document.getElementById('s-smtp-user') as HTMLInputElement;
      var smtpPassEl = document.getElementById('s-smtp-pass') as HTMLInputElement;
      var smtpFromEl = document.getElementById('s-smtp-from') as HTMLInputElement;
      var smtpToEl = document.getElementById('s-smtp-to') as HTMLInputElement;

      smtpHostEl.value = cfg['smtp_host'] || '';
      smtpPortEl.value = cfg['smtp_port'] || '587';
      smtpTlsEl.value = cfg['smtp_tls'] || 'starttls';
      smtpUserEl.value = cfg['smtp_user'] || '';
      smtpFromEl.value = cfg['smtp_from'] || '';
      smtpToEl.value = cfg['smtp_to'] || '';

      // Show placeholder if password is configured
      if (cfg['smtp_pass']) {
        smtpPassEl.placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (configured)';
      }

      // Toggle handler
      smtpEnabledEl.addEventListener('change', function() {
        var val = (smtpEnabledEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('smtp_enabled', val).then(function() {
          showToast((smtpEnabledEl as HTMLInputElement).checked ? '📧 Email notifications enabled' : '📧 Email notifications disabled', 'success');
        });
        smtpConfigRows!.style.display = (smtpEnabledEl as HTMLInputElement).checked ? '' : 'none';
      });

      // Auto-save for each SMTP field
      smtpHostEl.addEventListener('change', function() {
        updateConfig('smtp_host', smtpHostEl.value.trim());
        showToast('📧 SMTP host saved', 'success');
      });
      smtpPortEl.addEventListener('change', function() {
        updateConfig('smtp_port', smtpPortEl.value);
        showToast('📧 SMTP port saved', 'success');
      });
      smtpTlsEl.addEventListener('change', function() {
        updateConfig('smtp_tls', smtpTlsEl.value);
        showToast('📧 Encryption mode saved', 'success');
      });
      smtpUserEl.addEventListener('change', function() {
        updateConfig('smtp_user', smtpUserEl.value.trim());
        showToast('📧 SMTP username saved', 'success');
      });
      smtpPassEl.addEventListener('change', function() {
        var val = smtpPassEl.value.trim();
        if (val) {
          updateConfig('smtp_pass', val).then(function() {
            showToast('📧 SMTP password saved', 'success');
            smtpPassEl.value = '';
            smtpPassEl.placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (configured)';
          });
        }
      });
      smtpFromEl.addEventListener('change', function() {
        updateConfig('smtp_from', smtpFromEl.value.trim());
        showToast('📧 From address saved', 'success');
      });
      smtpToEl.addEventListener('change', function() {
        updateConfig('smtp_to', smtpToEl.value.trim());
        showToast('📧 To address saved', 'success');
      });

      // Test email button
      document.getElementById('smtp-test-btn')!.addEventListener('click', function() {
        var btn = document.getElementById('smtp-test-btn') as HTMLButtonElement;
        btn.disabled = true;
        btn.textContent = '📧 Sending...';
        fetch('/api/notify/test-email', { method: 'POST' })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) showToast('❌ ' + data.error, 'error');
            else showToast('📧 Test email sent!', 'success');
          })
          .catch(function() { showToast('❌ Failed to send test email', 'error'); })
          .finally(function() {
            btn.disabled = false;
            btn.textContent = '📧 Send Test';
          });
      });
    }

    // ── F22: Webhook Notifications ──
    var webhookEnabledEl = document.getElementById('s-webhook-enabled');
    var webhookConfigRows = document.getElementById('webhook-config-rows');
    if (webhookEnabledEl) {
      (webhookEnabledEl as HTMLInputElement).checked = cfg['webhook_enabled'] === 'true';
      webhookConfigRows!.style.display = (webhookEnabledEl as HTMLInputElement).checked ? '' : 'none';

      // Populate webhook fields from config
      var webhookTypeEl = document.getElementById('s-webhook-type') as HTMLSelectElement;
      var webhookUrlEl = document.getElementById('s-webhook-url') as HTMLInputElement;
      var webhookSecretEl = document.getElementById('s-webhook-secret') as HTMLInputElement;

      webhookTypeEl.value = cfg['webhook_type'] || 'discord';
      webhookUrlEl.value = cfg['webhook_url'] || '';

      if (cfg['webhook_secret']) {
        webhookSecretEl.placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (configured)';
      }

      // Update labels/hints based on selected type
      function updateWebhookLabels() {
        var urlLabel = document.getElementById('webhook-url-label')!;
        var urlHint = document.getElementById('webhook-url-hint')!;
        var secretLabel = document.getElementById('webhook-secret-label')!;
        var secretHint = document.getElementById('webhook-secret-hint')!;
        var secretRow = document.getElementById('webhook-secret-row')!;
        var urlInput = document.getElementById('s-webhook-url') as HTMLInputElement;

        switch (webhookTypeEl.value) {
          case 'discord':
            urlLabel.textContent = 'Webhook URL';
            urlHint.textContent = 'Discord channel webhook URL';
            urlInput.placeholder = 'https://discord.com/api/webhooks/...';
            secretRow.style.display = 'none';
            break;
          case 'telegram':
            urlLabel.textContent = 'Chat ID';
            urlHint.textContent = 'Telegram chat/group ID (numeric)';
            urlInput.placeholder = '123456789';
            secretRow.style.display = '';
            secretLabel.textContent = 'Bot Token';
            secretHint.textContent = 'Telegram bot token from @BotFather';
            webhookSecretEl.placeholder = '123456:ABC-DEF...';
            break;
          case 'slack':
            urlLabel.textContent = 'Webhook URL';
            urlHint.textContent = 'Slack incoming webhook URL';
            urlInput.placeholder = 'https://hooks.slack.com/services/...';
            secretRow.style.display = 'none';
            break;
          case 'generic':
            urlLabel.textContent = 'Endpoint URL';
            urlHint.textContent = 'ntfy/Gotify/custom POST URL';
            urlInput.placeholder = 'https://ntfy.sh/mytopic';
            secretRow.style.display = '';
            secretLabel.textContent = 'Auth Header';
            secretHint.textContent = 'Optional: Bearer token or Basic auth';
            webhookSecretEl.placeholder = 'Bearer your-token';
            break;
        }
      }
      updateWebhookLabels();

      // Toggle handler
      webhookEnabledEl.addEventListener('change', function() {
        var val = (webhookEnabledEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('webhook_enabled', val).then(function() {
          showToast((webhookEnabledEl as HTMLInputElement).checked ? '🔗 Webhook enabled' : '🔗 Webhook disabled', 'success');
        });
        webhookConfigRows!.style.display = (webhookEnabledEl as HTMLInputElement).checked ? '' : 'none';
      });

      // Type selector — save + update labels
      webhookTypeEl.addEventListener('change', function() {
        updateConfig('webhook_type', webhookTypeEl.value);
        showToast('🔗 Webhook service updated', 'success');
        updateWebhookLabels();
      });

      // Auto-save URL
      webhookUrlEl.addEventListener('change', function() {
        updateConfig('webhook_url', webhookUrlEl.value.trim());
        showToast('🔗 Webhook URL saved', 'success');
      });

      // Auto-save secret
      webhookSecretEl.addEventListener('change', function() {
        var val = webhookSecretEl.value.trim();
        if (val) {
          updateConfig('webhook_secret', val).then(function() {
            showToast('🔗 Webhook secret saved', 'success');
            webhookSecretEl.value = '';
            webhookSecretEl.placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (configured)';
          });
        }
      });

      // Test webhook button
      document.getElementById('webhook-test-btn')!.addEventListener('click', function() {
        var btn = document.getElementById('webhook-test-btn') as HTMLButtonElement;
        btn.disabled = true;
        btn.textContent = '🔗 Sending...';
        fetch('/api/notify/test-webhook', { method: 'POST' })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) showToast('❌ ' + data.error, 'error');
            else showToast('🔗 Test webhook sent!', 'success');
          })
          .catch(function() { showToast('❌ Failed to send test webhook', 'error'); })
          .finally(function() {
            btn.disabled = false;
            btn.textContent = '🔗 Send Test';
          });
      });
    }

    // ── F19: WebPush Notifications ──
    var webpushEnabledEl = document.getElementById('s-webpush-enabled');
    var webpushConfigRows = document.getElementById('webpush-config-rows');
    if (webpushEnabledEl && 'serviceWorker' in navigator && 'PushManager' in window) {
      (webpushEnabledEl as HTMLInputElement).checked = cfg['webpush_enabled'] === 'true';
      webpushConfigRows!.style.display = (webpushEnabledEl as HTMLInputElement).checked ? '' : 'none';

      // Helper: convert VAPID base64url to Uint8Array
      function urlBase64ToUint8Array(base64String: string): Uint8Array {
        var padding = '='.repeat((4 - base64String.length % 4) % 4);
        var base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
        var rawData = window.atob(base64);
        var outputArray = new Uint8Array(rawData.length);
        for (var i = 0; i < rawData.length; ++i) {
          outputArray[i] = rawData.charCodeAt(i);
        }
        return outputArray;
      }

      // Check subscription status
      function updateWebPushStatus() {
        var badge = document.getElementById('webpush-status-badge')!;
        var btn = document.getElementById('webpush-subscribe-btn') as HTMLButtonElement;

        navigator.serviceWorker.getRegistration('/sw.js').then(function(reg) {
          if (!reg) {
            badge.textContent = '⚪ Not registered';
            badge.style.color = 'var(--text-secondary)';
            btn.textContent = '🔔 Subscribe';
            return;
          }
          reg.pushManager.getSubscription().then(function(sub) {
            if (sub) {
              badge.textContent = '🟢 Subscribed';
              badge.style.color = '#22c55e';
              btn.textContent = '🔕 Unsubscribe';
            } else {
              badge.textContent = '⚪ Not subscribed';
              badge.style.color = 'var(--text-secondary)';
              btn.textContent = '🔔 Subscribe';
            }
          });
        });
      }
      updateWebPushStatus();

      // Toggle handler
      webpushEnabledEl.addEventListener('change', function() {
        var val = (webpushEnabledEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('webpush_enabled', val).then(function() {
          showToast((webpushEnabledEl as HTMLInputElement).checked ? '🔔 WebPush enabled' : '🔔 WebPush disabled', 'success');
        });
        webpushConfigRows!.style.display = (webpushEnabledEl as HTMLInputElement).checked ? '' : 'none';
      });

      // Subscribe/Unsubscribe button
      document.getElementById('webpush-subscribe-btn')!.addEventListener('click', function() {
        var btn = document.getElementById('webpush-subscribe-btn') as HTMLButtonElement;
        btn.disabled = true;

        navigator.serviceWorker.getRegistration('/sw.js').then(function(reg) {
          if (!reg) {
            // Register service worker first
            return navigator.serviceWorker.register('/sw.js');
          }
          return reg;
        }).then(function(reg) {
          return reg!.pushManager.getSubscription().then(function(existingSub) {
            if (existingSub) {
              // Unsubscribe
              return existingSub.unsubscribe().then(function() {
                return fetch('/api/webpush/subscribe', {
                  method: 'DELETE',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify({ endpoint: existingSub.endpoint })
                });
              }).then(function() {
                showToast('🔕 Unsubscribed from push notifications', 'success');
                updateWebPushStatus();
              });
            } else {
              // Subscribe — get VAPID key first
              return fetch('/api/webpush/vapid-key').then(function(r) { return r.json(); }).then(function(data) {
                var applicationServerKey = urlBase64ToUint8Array(data.publicKey);
                return reg!.pushManager.subscribe({
                  userVisibleOnly: true,
                  applicationServerKey: applicationServerKey
                });
              }).then(function(sub) {
                // Send subscription to server
                return fetch('/api/webpush/subscribe', {
                  method: 'POST',
                  headers: { 'Content-Type': 'application/json' },
                  body: JSON.stringify(sub.toJSON())
                });
              }).then(function() {
                showToast('🔔 Subscribed to push notifications!', 'success');
                updateWebPushStatus();
              });
            }
          });
        }).catch(function(err) {
          showToast('❌ Push subscription failed: ' + err.message, 'error');
        }).finally(function() {
          btn.disabled = false;
        });
      });

      // Test push button
      document.getElementById('webpush-test-btn')!.addEventListener('click', function() {
        var btn = document.getElementById('webpush-test-btn') as HTMLButtonElement;
        btn.disabled = true;
        btn.textContent = '🔔 Sending...';
        fetch('/api/notify/test-webpush', { method: 'POST' })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) showToast('❌ ' + data.error, 'error');
            else showToast('🔔 Test push sent!', 'success');
          })
          .catch(function() { showToast('❌ Failed to send test push', 'error'); })
          .finally(function() {
            btn.disabled = false;
            btn.textContent = '🔔 Send Test';
          });
      });
    } else if (webpushEnabledEl) {
      // Browser doesn't support push — disable the toggle
      (webpushEnabledEl as HTMLInputElement).disabled = true;
      var hint = document.getElementById('webpush-status-hint');
      if (hint) hint.textContent = 'Not supported in this browser';
    }

    // ── Phase 11: Codex Capture Toggle ──
    var codexCaptureEl = document.getElementById('s-codex-capture');
    if (codexCaptureEl) {
      (codexCaptureEl as HTMLInputElement).checked = cfg['codex_capture'] === 'true';
      codexCaptureEl.addEventListener('change', function() {
        var val = (codexCaptureEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('codex_capture', val).then(function() {
          showToast((codexCaptureEl as HTMLInputElement).checked ? '🤖 Codex capture enabled' : '🤖 Codex capture disabled', 'success');
          loadCodexSettingsStatus();
          // T1: Refresh data sources list to reflect new enabled state
          loadDataSources();
        });
      });
      loadCodexSettingsStatus();
    }

    // ── F15a: Cursor Capture Toggle ──
    var cursorCaptureEl = document.getElementById('s-cursor-capture');
    if (cursorCaptureEl) {
      (cursorCaptureEl as HTMLInputElement).checked = cfg['cursor_capture'] === 'true';
      cursorCaptureEl.addEventListener('change', function() {
        var val = (cursorCaptureEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('cursor_capture', val).then(function() {
          showToast((cursorCaptureEl as HTMLInputElement).checked ? '\ud83d\uddb1\ufe0f Cursor capture enabled' : '\ud83d\uddb1\ufe0f Cursor capture disabled', 'success');
          loadDataSources();
        });
      });
    }

    // ── F15b: Gemini CLI Capture Toggle ──
    var geminiCaptureEl = document.getElementById('s-gemini-capture');
    if (geminiCaptureEl) {
      (geminiCaptureEl as HTMLInputElement).checked = cfg['gemini_capture'] === 'true';
      geminiCaptureEl.addEventListener('change', function() {
        var val = (geminiCaptureEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('gemini_capture', val).then(function() {
          showToast((geminiCaptureEl as HTMLInputElement).checked ? '\u2728 Gemini capture enabled' : '\u2728 Gemini capture disabled', 'success');
          loadDataSources();
        });
      });
    }

    // ── F15c: GitHub Copilot Capture Toggle + PAT ──
    var copilotCaptureEl = document.getElementById('s-copilot-capture');
    var copilotPatEl = document.getElementById('s-copilot-pat');
    if (copilotCaptureEl) {
      (copilotCaptureEl as HTMLInputElement).checked = cfg['copilot_capture'] === 'true';
      copilotCaptureEl.addEventListener('change', function() {
        var val = (copilotCaptureEl as HTMLInputElement).checked ? 'true' : 'false';
        updateConfig('copilot_capture', val).then(function() {
          showToast((copilotCaptureEl as HTMLInputElement).checked ? '\ud83d\udc19 Copilot capture enabled' : '\ud83d\udc19 Copilot capture disabled', 'success');
          loadDataSources();
        });
      });
    }
    if (copilotPatEl) {
      // Show masked value if PAT exists
      if (cfg['copilot_pat']) {
        (copilotPatEl as HTMLInputElement).placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (configured)';
      }
      copilotPatEl.addEventListener('change', function() {
        var val = (copilotPatEl as HTMLInputElement).value.trim();
        if (val) {
          updateConfig('copilot_pat', val).then(function() {
            showToast('\ud83d\udc19 Copilot PAT saved', 'success');
            (copilotPatEl as HTMLInputElement).value = '';
            (copilotPatEl as HTMLInputElement).placeholder = '\u2022\u2022\u2022\u2022\u2022\u2022\u2022\u2022 (configured)';
            loadDataSources();
          });
        }
      });
    }

    // ── Phase 11: JSON Import ──
    var importBtn = document.getElementById('import-json-btn');
    var importFile = document.getElementById('import-file');
    if (importBtn && importFile) {
      importBtn.addEventListener('click', function() { (importFile as HTMLInputElement).click(); });
      importFile.addEventListener('change', function() {
        if (!(importFile as HTMLInputElement).files || !(importFile as HTMLInputElement).files![0]) return;
        var file = (importFile as HTMLInputElement).files![0];
        showToast('📥 Importing ' + file.name + '...', 'info');
        var reader = new FileReader();
        reader.onload = function(e) {
          fetch('/api/import/json', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: (e.target as FileReader).result as string
          })
          .then(function(r) { return r.json(); })
          .then(function(data) {
            if (data.error) {
              showToast('❌ Import failed: ' + data.error, 'error');
              return;
            }
            var msg = '✅ Imported: ' + (data.accountsCreated || 0) + ' accounts, ' +
                      (data.subsCreated || 0) + ' subs, ' +
                      (data.snapshotsImported || 0) + ' snapshots';
            showToast(msg, 'success');
            var resultEl = document.getElementById('import-result');
            if (resultEl) {
              resultEl.style.display = '';
              resultEl.innerHTML = '<span style="color:var(--accent)">' + msg + '</span>' +
                (data.accountsSkipped ? '<br>Accounts skipped (existing): ' + data.accountsSkipped : '') +
                (data.subsSkipped ? '<br>Subs skipped (existing): ' + data.subsSkipped : '') +
                (data.snapshotsDuped ? '<br>Snapshots deduped: ' + data.snapshotsDuped : '') +
                (data.errors && data.errors.length ? '<br>⚠️ Errors: ' + data.errors.length : '');
            }
            // Refresh data
            fetchStatus().then(renderAccounts);
            loadSubscriptions();
          })
          .catch(function() { showToast('❌ Import failed', 'error'); });
        };
        reader.readAsText(file);
        (importFile as HTMLInputElement).value = ''; // reset for re-import
      });
    }
  });

  // ── Phase 13 F5: Model Pricing ──
  loadModelPricing();
  document.getElementById('pricing-add-btn')!.addEventListener('click', addPricingRow);
  document.getElementById('pricing-reset-btn')!.addEventListener('click', resetPricingDefaults);

  // Load mode badge
  loadMode();

  // Load data sources
  loadDataSources();

  // Activity log controls
  document.getElementById('activity-refresh')!.addEventListener('click', loadActivityLog);
  document.getElementById('activity-filter')!.addEventListener('change', loadActivityLog);
  loadActivityLog();

  // F18: Load plugins
  loadPlugins();
}

// One-time migration of localStorage budget/currency to server config
export function migrateLocalStorage(cfg: Record<string, string>): void {
  var lsBudget = localStorage.getItem('niyantra-budget');
  var lsCurrency = localStorage.getItem('niyantra-currency');

  if (lsBudget && (!cfg['budget_monthly'] || cfg['budget_monthly'] === '0')) {
    updateConfig('budget_monthly', lsBudget);
    serverConfig['budget_monthly'] = lsBudget;
    localStorage.removeItem('niyantra-budget');
  }
  if (lsCurrency && cfg['currency'] === 'USD') {
    updateConfig('currency', lsCurrency);
    serverConfig['currency'] = lsCurrency;
    localStorage.removeItem('niyantra-currency');
  }
}

// ════════════════════════════════════════════
