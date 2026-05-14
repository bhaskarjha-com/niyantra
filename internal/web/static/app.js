"use strict";
var App = (() => {
  // internal/web/src/core/state.ts
  var GROUP_ORDER = ["claude_gpt", "gemini_pro", "gemini_flash"];
  var GROUP_LABELS = ["Claude + GPT", "Gemini Pro", "Gemini Flash"];
  var GROUP_COLORS = { claude_gpt: "#D97757", gemini_pro: "#10B981", gemini_flash: "#3B82F6" };
  var GROUP_NAMES = { claude_gpt: "Claude + GPT", gemini_pro: "Gemini Pro", gemini_flash: "Gemini Flash" };
  var expandedAccounts = /* @__PURE__ */ new Set();
  var collapsedProviders = /* @__PURE__ */ new Set();
  var presetsData = [];
  function setPresetsData(data) {
    presetsData = data;
  }
  var activeTagFilter = null;
  function setActiveTagFilter(val) {
    activeTagFilter = val;
  }
  var usageDataCache = null;
  function setUsageDataCache(data) {
    usageDataCache = data;
  }
  var quotaSortState = { column: "account", direction: "asc" };
  var latestQuotaData = null;
  function setLatestQuotaData(data) {
    latestQuotaData = data;
  }
  var serverConfig = {};
  var snapInProgress = false;
  function setSnapInProgress(val) {
    snapInProgress = val;
  }

  // internal/web/src/core/utils.ts
  function formatSeconds(seconds) {
    seconds = Math.floor(seconds);
    if (seconds <= 0) return "now";
    var h = Math.floor(seconds / 3600);
    var m = Math.floor(seconds % 3600 / 60);
    if (h >= 24) return Math.floor(h / 24) + "d " + h % 24 + "h";
    if (h > 0) return h + "h " + m + "m";
    if (m === 0) return "<1m";
    return m + "m";
  }
  function formatCredits(n) {
    if (n >= 1e3) return (n / 1e3).toFixed(n % 1e3 === 0 ? 0 : 1) + "k";
    return Math.round(n).toString();
  }
  function formatNumber(n) {
    if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
    if (n >= 1e3) return (n / 1e3).toFixed(n % 1e3 === 0 ? 0 : 1) + "k";
    return n.toString();
  }
  function currencySymbol(code) {
    var map = { USD: "$", EUR: "\u20AC", GBP: "\xA3", INR: "\u20B9", CAD: "C$", AUD: "A$" };
    return map[code] || code + " ";
  }
  function esc(s) {
    if (!s) return "";
    var d = document.createElement("div");
    d.textContent = s;
    return d.innerHTML.replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }
  function showToast(msg, type) {
    var el = document.getElementById("toast");
    if (!el) return;
    el.textContent = msg;
    el.className = "toast " + type + " visible";
    el.hidden = false;
    setTimeout(function() {
      el.classList.remove("visible");
      setTimeout(function() {
        el.hidden = true;
      }, 300);
    }, 3e3);
  }
  var lastUpdateTime = null;
  function updateTimestamp() {
    lastUpdateTime = /* @__PURE__ */ new Date();
    refreshTimestampDisplay();
  }
  function refreshTimestampDisplay() {
    var el = document.getElementById("last-updated");
    if (!el || !lastUpdateTime) return;
    var sec = Math.floor(((/* @__PURE__ */ new Date()).getTime() - lastUpdateTime.getTime()) / 1e3);
    var label;
    if (sec < 10) label = "just now";
    else if (sec < 60) label = sec + "s ago";
    else if (sec < 3600) label = Math.floor(sec / 60) + "m ago";
    else label = Math.floor(sec / 3600) + "h ago";
    el.textContent = "Updated " + label;
    el.title = lastUpdateTime.toLocaleTimeString();
  }
  function formatTimeAgo(isoStr) {
    if (!isoStr) return "never";
    var d = new Date(isoStr);
    var now = /* @__PURE__ */ new Date();
    var sec = Math.floor((now.getTime() - d.getTime()) / 1e3);
    if (sec < 60) return "just now";
    if (sec < 3600) return Math.floor(sec / 60) + "m ago";
    if (sec < 86400) return Math.floor(sec / 3600) + "h ago";
    return Math.floor(sec / 86400) + "d ago";
  }
  function formatPollInterval(seconds) {
    if (seconds >= 3600) return Math.floor(seconds / 3600) + "h";
    return Math.floor(seconds / 60) + "m";
  }
  function formatDurationSec(sec) {
    if (!sec || sec <= 0) return "0m";
    var h = Math.floor(sec / 3600);
    var m = Math.floor(sec % 3600 / 60);
    if (h > 0) return h + "h " + m + "m";
    return m + "m";
  }

  // internal/web/src/core/api.ts
  function fetchStatus() {
    return fetch("/api/status").then(function(res) {
      if (!res.ok) throw new Error("Failed to fetch status");
      return res.json();
    });
  }
  function triggerSnap() {
    return fetch("/api/snap", { method: "POST" }).then(function(res) {
      return res.json().then(function(data) {
        if (!res.ok) throw new Error(data.error || "Snap failed");
        return data;
      });
    });
  }
  function fetchSubscriptions(status, category) {
    var params = new URLSearchParams();
    if (status) params.set("status", status);
    if (category) params.set("category", category);
    var url = "/api/subscriptions" + (params.toString() ? "?" + params : "");
    return fetch(url).then(function(res) {
      return res.json();
    });
  }
  function createSubscription(sub) {
    return fetch("/api/subscriptions", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(sub)
    }).then(function(res) {
      return res.json().then(function(data) {
        if (!res.ok) throw new Error(data.error || "Create failed");
        return data;
      });
    });
  }
  function updateSubscription(id, sub) {
    return fetch("/api/subscriptions/" + id, {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(sub)
    }).then(function(res) {
      return res.json().then(function(data) {
        if (!res.ok) throw new Error(data.error || "Update failed");
        return data;
      });
    });
  }
  function deleteSubscription(id) {
    return fetch("/api/subscriptions/" + id, { method: "DELETE" }).then(function(res) {
      return res.json().then(function(data) {
        if (!res.ok) throw new Error(data.error || "Delete failed");
        return data;
      });
    });
  }
  function fetchOverview() {
    return fetch("/api/overview").then(function(res) {
      return res.json();
    });
  }
  function fetchPresets() {
    return fetch("/api/presets").then(function(res) {
      return res.json();
    });
  }
  function fetchUsage(accountId) {
    var url = "/api/usage";
    if (accountId) url += "?account=" + accountId;
    return fetch(url).then(function(res) {
      return res.json();
    }).then(function(data) {
      setUsageDataCache(data);
      return data;
    });
  }

  // internal/web/src/core/theme.ts
  function initTheme() {
    var saved = localStorage.getItem("niyantra-theme");
    if (saved) {
      document.documentElement.setAttribute("data-theme", saved);
    } else if (window.matchMedia("(prefers-color-scheme: light)").matches) {
      document.documentElement.setAttribute("data-theme", "light");
    }
    var themeBtn = document.getElementById("theme-btn");
    if (!themeBtn) return;
    themeBtn.addEventListener("click", function() {
      var current = document.documentElement.getAttribute("data-theme");
      var next = current === "light" ? "dark" : "light";
      document.documentElement.setAttribute("data-theme", next);
      localStorage.setItem("niyantra-theme", next);
      document.dispatchEvent(new CustomEvent("niyantra:theme-change", { detail: { theme: next } }));
    });
  }
  function initTabs() {
    var btns = document.querySelectorAll(".tab-btn");
    btns.forEach(function(btn) {
      btn.addEventListener("click", function() {
        var tab = btn.getAttribute("data-tab");
        if (tab) switchToTab(tab);
      });
    });
  }
  function switchToTab(tabName) {
    var btns = document.querySelectorAll(".tab-btn");
    btns.forEach(function(b) {
      b.classList.remove("active");
    });
    var target = document.querySelector('.tab-btn[data-tab="' + tabName + '"]');
    if (target) target.classList.add("active");
    document.querySelectorAll(".tab-panel").forEach(function(p) {
      p.classList.remove("active");
    });
    var panel = document.getElementById("panel-" + tabName);
    if (panel) panel.classList.add("active");
    document.dispatchEvent(new CustomEvent("niyantra:tab-change", { detail: { tab: tabName } }));
  }

  // internal/web/src/quotas/features.ts
  var _renderAccounts = null;
  function setRenderAccounts(fn) {
    _renderAccounts = fn;
  }
  function refreshGrid() {
    if (_renderAccounts) fetchStatus().then(_renderAccounts);
  }
  function renderPinnedBadge(groupData, pinnedKey) {
    if (!groupData) return "";
    var pct = Math.round(groupData.remainingPercent);
    var cls = "good";
    if (groupData.isExhausted || pct === 0) cls = "exhausted";
    else if (pct < 20) cls = "warning";
    else if (pct < 50) cls = "ok";
    return ' <span class="pinned-badge ' + cls + '" title="Pinned: ' + esc(groupData.displayName || pinnedKey) + '">\u2605 ' + esc(groupData.displayName || GROUP_NAMES[pinnedKey] || pinnedKey) + ": " + pct + "%</span>";
  }
  function pinGroup(accountId, groupKey) {
    updateAccountMeta(accountId, { pinnedGroup: groupKey }).then(function() {
      showToast("\u2B50 Pinned " + (GROUP_NAMES[groupKey] || groupKey), "success");
      refreshGrid();
    });
  }
  function unpinGroup(accountId) {
    updateAccountMeta(accountId, { pinnedGroup: "" }).then(function() {
      showToast("\u2606 Unpinned \u2014 will show first group", "info");
      refreshGrid();
    });
  }
  function daysUntilRenewal(day) {
    if (!day || day < 1 || day > 31) return -1;
    var now = /* @__PURE__ */ new Date();
    var y = now.getFullYear();
    var m = now.getMonth();
    var today = now.getDate();
    var targetMonth = today < day ? m : m + 1;
    var target = new Date(y, targetMonth, day);
    var diff = Math.ceil((target.getTime() - now.getTime()) / (1e3 * 60 * 60 * 24));
    return diff < 0 ? 0 : diff;
  }
  function renderCreditRenewal(accountId, renewalDay) {
    if (!renewalDay || renewalDay < 1) {
      return '<span class="credit-renewal-set" data-renewal-edit="' + accountId + '" title="Set credit renewal day">\u21BB set</span>';
    }
    var days = daysUntilRenewal(renewalDay);
    var label = days === 0 ? "today" : days === 1 ? "1d" : days + "d";
    return '<span class="credit-renewal" data-renewal-edit="' + accountId + '" data-renewal-day="' + renewalDay + '" title="Credits renew on day ' + renewalDay + " (\u21BB " + label + ')">\u21BB ' + label + "</span>";
  }
  function openRenewalPicker(el) {
    var existing = document.querySelector(".renewal-picker");
    if (existing) existing.remove();
    var accountId = el.getAttribute("data-renewal-edit");
    var currentDay = parseInt(el.getAttribute("data-renewal-day")) || 0;
    var picker = document.createElement("div");
    picker.className = "renewal-picker";
    picker.innerHTML = '<div class="renewal-picker-label">Credit Renewal Day</div><input type="number" class="renewal-picker-input" min="1" max="31" value="' + (currentDay || "") + '" placeholder="1\u201331"><div class="renewal-picker-hint">Day of month when AI credits refresh.<br>Find at one.google.com/ai/activity</div>';
    el.closest(".credits-cell").appendChild(picker);
    var input = picker.querySelector("input");
    input.focus();
    input.select();
    function save() {
      var day = parseInt(input.value) || 0;
      if (day > 31) day = 31;
      if (day < 0) day = 0;
      picker.remove();
      updateAccountMeta(accountId, { creditRenewalDay: day }).then(function() {
        if (day > 0) {
          showToast("\u21BB Renewal day set to " + day, "success");
        } else {
          showToast("\u21BB Renewal day cleared", "info");
        }
        refreshGrid();
      });
    }
    input.addEventListener("keydown", function(e) {
      if (e.key === "Enter") {
        e.preventDefault();
        save();
      }
      if (e.key === "Escape") {
        e.preventDefault();
        picker.remove();
      }
    });
    input.addEventListener("blur", function() {
      setTimeout(function() {
        if (picker.parentNode) save();
      }, 150);
    });
  }
  var TAG_PRESETS = ["work", "personal", "primary", "backup", "shared", "test", "dev"];
  function renderAccountTags(acc) {
    var tags = (acc.tags || "").split(",").filter(function(t) {
      return t.trim();
    });
    var html = '<span class="account-tags" data-account-id="' + acc.accountId + '">';
    for (var i = 0; i < tags.length; i++) {
      html += '<span class="tag-chip" data-tag="' + esc(tags[i].trim()) + '">' + esc(tags[i].trim()) + '<span class="tag-remove" data-remove-tag="' + esc(tags[i].trim()) + '" data-account-id="' + acc.accountId + '" title="Remove tag">\u2715</span></span>';
    }
    html += "</span>";
    html += '<button class="tag-add-btn" data-tag-add="' + acc.accountId + '" title="Add tag">+</button>';
    return html;
  }
  function renderAccountNote(acc) {
    if (acc.notes) {
      return '<span class="account-note" data-note-edit="' + acc.accountId + '" data-current-note="' + esc(acc.notes) + '" title="' + esc(acc.notes) + ' \u2014 click to edit">\u{1F4DD} ' + esc(acc.notes) + "</span>";
    }
    return '<span class="account-note-empty" data-note-edit="' + acc.accountId + '" data-current-note="">+ note</span>';
  }
  function updateAccountMeta(accountId, patch) {
    return fetch("/api/accounts/" + accountId + "/meta", {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(patch)
    }).then(function(r) {
      return r.json();
    });
  }
  function addTagToAccount(accountId, newTag) {
    newTag = newTag.trim().toLowerCase().replace(/[^a-z0-9_-]/g, "");
    if (!newTag) return;
    var container = document.querySelector('.account-tags[data-account-id="' + accountId + '"]');
    var existing = [];
    if (container) {
      container.querySelectorAll(".tag-chip").forEach(function(chip) {
        existing.push(chip.getAttribute("data-tag"));
      });
    }
    if (existing.indexOf(newTag) >= 0) return;
    existing.push(newTag);
    updateAccountMeta(accountId, { tags: existing.join(",") }).then(function() {
      showToast('\u{1F3F7}\uFE0F Tag "' + newTag + '" added', "success");
      refreshGrid();
    });
  }
  function removeTagFromAccount(accountId, tag) {
    var container = document.querySelector('.account-tags[data-account-id="' + accountId + '"]');
    var tags = [];
    if (container) {
      container.querySelectorAll(".tag-chip").forEach(function(chip) {
        var t = chip.getAttribute("data-tag");
        if (t !== tag) tags.push(t);
      });
    }
    updateAccountMeta(accountId, { tags: tags.join(",") }).then(function() {
      showToast("\u{1F3F7}\uFE0F Tag removed", "success");
      refreshGrid();
    });
  }
  function openTagPicker(btn) {
    closeTagPicker();
    var accountId = btn.getAttribute("data-tag-add");
    var meta = btn.closest(".account-meta");
    if (!meta) return;
    var existing = [];
    var container = meta.querySelector(".account-tags");
    if (container) {
      container.querySelectorAll(".tag-chip").forEach(function(chip) {
        existing.push(chip.getAttribute("data-tag"));
      });
    }
    var picker = document.createElement("div");
    picker.className = "tag-picker";
    picker.id = "active-tag-picker";
    picker.innerHTML = '<input type="text" class="tag-picker-input" placeholder="Type tag name..." autocomplete="off" maxlength="20"><div class="tag-picker-hint">Enter to add</div><div class="tag-picker-presets">' + TAG_PRESETS.map(function(p) {
      var active = existing.indexOf(p) >= 0 ? " active" : "";
      return '<button class="tag-preset' + active + '" data-preset-tag="' + p + '">' + p + "</button>";
    }).join("") + "</div>";
    meta.appendChild(picker);
    var input = picker.querySelector(".tag-picker-input");
    input.focus();
    picker.addEventListener("click", function(e) {
      e.stopPropagation();
    });
    input.addEventListener("keydown", function(e) {
      if (e.key === "Enter") {
        e.preventDefault();
        var val = input.value.trim();
        if (val) {
          addTagToAccount(accountId, val);
          closeTagPicker();
        }
      }
      if (e.key === "Escape") {
        closeTagPicker();
      }
    });
    picker.querySelectorAll(".tag-preset").forEach(function(btn2) {
      btn2.addEventListener("click", function(e) {
        e.stopPropagation();
        var tag = btn2.getAttribute("data-preset-tag");
        if (btn2.classList.contains("active")) {
          removeTagFromAccount(accountId, tag);
        } else {
          addTagToAccount(accountId, tag);
        }
        closeTagPicker();
      });
    });
    setTimeout(function() {
      document.addEventListener("click", closeTagPickerOnOutside);
    }, 10);
  }
  function closeTagPicker() {
    var picker = document.getElementById("active-tag-picker");
    if (picker) picker.remove();
    document.removeEventListener("click", closeTagPickerOnOutside);
  }
  function closeTagPickerOnOutside(e) {
    var picker = document.getElementById("active-tag-picker");
    if (picker && !picker.contains(e.target)) {
      closeTagPicker();
    }
  }
  function openNoteEditor(el) {
    var accountId = el.getAttribute("data-note-edit");
    var currentNote = el.getAttribute("data-current-note") || "";
    var editor = document.createElement("span");
    editor.className = "note-inline-editor";
    editor.innerHTML = '<input type="text" class="note-inline-input" value="' + esc(currentNote) + '" placeholder="Add a note..." maxlength="100">';
    el.replaceWith(editor);
    var input = editor.querySelector(".note-inline-input");
    input.focus();
    input.select();
    editor.addEventListener("click", function(e) {
      e.stopPropagation();
    });
    function save() {
      var val = input.value.trim();
      updateAccountMeta(accountId, { notes: val }).then(function() {
        if (val) showToast("\u{1F4DD} Note saved", "success");
        refreshGrid();
      });
    }
    input.addEventListener("keydown", function(e) {
      if (e.key === "Enter") {
        e.preventDefault();
        save();
      }
      if (e.key === "Escape") {
        refreshGrid();
      }
    });
    input.addEventListener("blur", save);
  }
  function initAccountMetaHandlers() {
    var grid = document.getElementById("account-grid");
    if (!grid) return;
    grid.addEventListener("click", function(e) {
      var removeBtn = e.target.closest("[data-remove-tag]");
      if (removeBtn) {
        e.stopPropagation();
        e.preventDefault();
        removeTagFromAccount(removeBtn.getAttribute("data-account-id"), removeBtn.getAttribute("data-remove-tag"));
        return;
      }
      var addBtn = e.target.closest("[data-tag-add]");
      if (addBtn) {
        e.stopPropagation();
        e.preventDefault();
        openTagPicker(addBtn);
        return;
      }
      var noteEl = e.target.closest("[data-note-edit]");
      if (noteEl) {
        e.stopPropagation();
        e.preventDefault();
        openNoteEditor(noteEl);
        return;
      }
      var pinBtn = e.target.closest("[data-pin-group]");
      if (pinBtn) {
        e.stopPropagation();
        e.preventDefault();
        var pinAccountId = pinBtn.getAttribute("data-pin-account");
        var pinGroupKey = pinBtn.getAttribute("data-pin-group");
        if (pinBtn.classList.contains("pinned")) {
          unpinGroup(pinAccountId);
        } else {
          pinGroup(pinAccountId, pinGroupKey);
        }
        return;
      }
      var renewalEl = e.target.closest("[data-renewal-edit]");
      if (renewalEl) {
        e.stopPropagation();
        e.preventDefault();
        openRenewalPicker(renewalEl);
        return;
      }
    });
  }

  // internal/web/src/quotas/render.ts
  function getGroupPct(acc, groupKey) {
    if (!acc.groups) return -1;
    for (var i = 0; i < acc.groups.length; i++) {
      if (acc.groups[i].groupKey === groupKey) return acc.groups[i].remainingPercent;
    }
    return -1;
  }
  function getAICredits(acc) {
    if (acc.aiCredits && acc.aiCredits.length > 0) return acc.aiCredits[0].creditAmount;
    return -1;
  }
  function allExhausted(acc) {
    var grps = acc.groups || [];
    if (grps.length === 0) return false;
    for (var i = 0; i < grps.length; i++) {
      if (!grps[i].isExhausted && grps[i].remainingPercent > 0) return false;
    }
    return true;
  }
  function getCodexClaudeStatus(snap) {
    var fiveUsed = snap.fiveHourPct || 0;
    var sevenUsed = snap.sevenDayPct || 0;
    var fiveRem = Math.max(0, 100 - fiveUsed);
    var sevenRem = Math.max(0, 100 - sevenUsed);
    if (fiveRem === 0 && sevenRem === 0) return "empty";
    if (fiveUsed >= 80 || sevenUsed >= 80) return "low";
    return "ready";
  }
  function sortAccountsArray(accounts) {
    var col = quotaSortState.column;
    var dir = quotaSortState.direction;
    return accounts.slice().sort(function(a, b) {
      var va, vb;
      switch (col) {
        case "account":
          va = a.email;
          vb = b.email;
          break;
        case "claude_gpt":
        case "gemini_pro":
        case "gemini_flash":
          va = getGroupPct(a, col);
          vb = getGroupPct(b, col);
          break;
        case "credits":
          va = getAICredits(a);
          vb = getAICredits(b);
          break;
        case "lastsnap":
          va = a.lastSeen ? new Date(a.lastSeen).getTime() : 0;
          vb = b.lastSeen ? new Date(b.lastSeen).getTime() : 0;
          break;
        case "status":
          va = a.isReady ? 1 : 0;
          vb = b.isReady ? 1 : 0;
          break;
        default:
          va = a.email;
          vb = b.email;
          break;
      }
      if (va === vb) return 0;
      var res = va > vb ? 1 : -1;
      return dir === "asc" ? res : -res;
    });
  }
  function filterAccountsArray(accounts) {
    var searchInput = document.getElementById("quota-search");
    var statusFilter = document.getElementById("quota-filter-status");
    var query = searchInput ? searchInput.value.toLowerCase() : "";
    var status = statusFilter ? statusFilter.value : "all";
    return accounts.filter(function(acc) {
      var matchesSearch = !query || acc.email.toLowerCase().includes(query) || (acc.planName || "").toLowerCase().includes(query);
      var matchesStatus = true;
      if (status === "ready") matchesStatus = acc.isReady;
      else if (status === "low") matchesStatus = !acc.isReady && !allExhausted(acc);
      else if (status === "empty") matchesStatus = allExhausted(acc);
      var matchesTag = true;
      if (activeTagFilter) {
        var accTags = (acc.tags || "").split(",").map(function(t) {
          return t.trim().toLowerCase();
        });
        matchesTag = accTags.indexOf(activeTagFilter) >= 0;
      }
      return matchesSearch && matchesStatus && matchesTag;
    });
  }
  function getUniqueTagsFromData(data) {
    var tagCounts = {};
    var accounts = data.accounts || [];
    for (var i = 0; i < accounts.length; i++) {
      var tags = (accounts[i].tags || "").split(",");
      for (var j = 0; j < tags.length; j++) {
        var t = tags[j].trim().toLowerCase();
        if (t) {
          tagCounts[t] = (tagCounts[t] || 0) + 1;
        }
      }
    }
    return tagCounts;
  }
  function renderTagFilterStrip(data) {
    var strip = document.getElementById("tag-filter-strip");
    if (!strip) return;
    var tagCounts = getUniqueTagsFromData(data);
    var tagNames = Object.keys(tagCounts).sort();
    if (tagNames.length === 0) {
      if (activeTagFilter) {
        setActiveTagFilter(null);
      }
      strip.innerHTML = "";
      return;
    }
    if (activeTagFilter && tagNames.indexOf(activeTagFilter) < 0) {
      setActiveTagFilter(null);
    }
    var html = '<span class="tag-filter-label">\u{1F3F7}\uFE0F Filter:</span>';
    var allActive = !activeTagFilter ? " active" : "";
    var totalAccounts = (data.accounts || []).length;
    html += '<button class="tag-filter-chip' + allActive + '" data-tag-filter="">All <span class="tag-filter-count">' + totalAccounts + "</span></button>";
    for (var i = 0; i < tagNames.length; i++) {
      var tag = tagNames[i];
      var isActive = activeTagFilter === tag ? " active" : "";
      html += '<button class="tag-filter-chip' + isActive + '" data-tag-filter="' + esc(tag) + '">' + esc(tag) + ' <span class="tag-filter-count">' + tagCounts[tag] + "</span></button>";
    }
    strip.innerHTML = html;
  }
  function handleTagFilterClick(e) {
    var chip = e.target.closest(".tag-filter-chip");
    if (!chip) return;
    var tag = chip.getAttribute("data-tag-filter");
    setActiveTagFilter(tag || null);
    if (latestQuotaData) {
      renderTagFilterStrip(latestQuotaData);
      renderAccounts(latestQuotaData);
    }
  }
  function renderAccounts(data) {
    setLatestQuotaData(data);
    var grid = document.getElementById("account-grid");
    var countBadge = document.getElementById("account-count");
    var snapCount = document.getElementById("snap-count");
    if (!grid) return;
    renderTagFilterStrip(data);
    var acctCount = (data.accounts || []).length;
    var parts = [];
    if (acctCount > 0) parts.push(acctCount + " Antigravity");
    if (data.codexSnapshot) parts.push("1 Codex");
    if (data.claudeSnapshot) parts.push("1 Claude");
    if (data.cursorSnapshot) parts.push("1 Cursor");
    if (data.geminiSnapshot) parts.push("1 Gemini");
    if (countBadge) countBadge.textContent = parts.join(" \xB7 ") || "0 accounts";
    if (snapCount) snapCount.textContent = data.snapshotCount ? data.snapshotCount + " snapshots" : "";
    if (acctCount === 0 && !data.codexSnapshot && !data.claudeSnapshot && !data.cursorSnapshot && !data.geminiSnapshot) {
      grid.innerHTML = '<div class="empty-state"><svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.4"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/><path d="M12 2v4M12 18v4M2 12h4M18 12h4"/></svg><p>No accounts tracked yet</p><p class="empty-hint">Click <strong>Snap Now</strong> to capture your first snapshot</p></div>';
      return;
    }
    var providerFilter = document.getElementById("quota-filter-provider");
    var pf = providerFilter ? providerFilter.value : "all";
    var html = "";
    if (acctCount > 0 && (pf === "all" || pf === "antigravity")) {
      var filtered = filterAccountsArray(data.accounts);
      var sorted = sortAccountsArray(filtered);
      var agCollapseClass = collapsedProviders.has("section-antigravity") ? " collapsed" : "";
      var agChevron = collapsedProviders.has("section-antigravity") ? "\u25B8" : "\u25BE";
      html += '<div class="provider-section" data-provider="antigravity"><div class="provider-header" data-toggle-provider="section-antigravity"><div class="provider-header-left"><span class="provider-chevron" id="pchev-section-antigravity">' + agChevron + '</span><span class="provider-name">Antigravity</span><span class="provider-count">' + acctCount + " account" + (acctCount !== 1 ? "s" : "") + '</span></div></div><div class="provider-body' + agCollapseClass + '" id="section-antigravity">';
      html += '<div class="grid-header"><div class="grid-col-account sortable" data-sort="account">Account <span class="sort-indicator"></span></div>';
      for (var gh = 0; gh < GROUP_ORDER.length; gh++) {
        html += '<div class="grid-col-group sortable" data-sort="' + GROUP_ORDER[gh] + '">' + (GROUP_LABELS[gh] || GROUP_ORDER[gh]) + ' <span class="sort-indicator"></span></div>';
      }
      html += '<div class="grid-col-credits sortable" data-sort="credits">AI Credits <span class="sort-indicator"></span></div><div class="grid-col-snap sortable" data-sort="lastsnap">Last Snap <span class="sort-indicator"></span></div><div class="grid-col-status sortable" data-sort="status">Status <span class="sort-indicator"></span></div></div>';
      for (var i = 0; i < sorted.length; i++) {
        var acc = sorted[i];
        var accId = "acc-" + acc.accountId;
        var isExpanded = expandedAccounts.has(accId);
        var groupCells = "";
        var modelsByGroup = {};
        if (acc.models) {
          for (var mi2 = 0; mi2 < acc.models.length; mi2++) {
            var mm = acc.models[mi2];
            var gk = mm.groupKey || "claude_gpt";
            if (!modelsByGroup[gk]) modelsByGroup[gk] = [];
            modelsByGroup[gk].push(mm.label || mm.modelId);
          }
        }
        var pinnedKey = acc.pinnedGroup || (acc.groups && acc.groups.length > 0 ? acc.groups[0].groupKey : "claude_gpt");
        var pinnedGroupData = null;
        var groups = acc.groups || [];
        for (var pg = 0; pg < groups.length; pg++) {
          if (groups[pg].groupKey === pinnedKey) {
            pinnedGroupData = groups[pg];
            break;
          }
        }
        for (var gi = 0; gi < GROUP_ORDER.length; gi++) {
          var key = GROUP_ORDER[gi];
          var g = null;
          var groups = acc.groups || [];
          for (var gj = 0; gj < groups.length; gj++) {
            if (groups[gj].groupKey === key) {
              g = groups[gj];
              break;
            }
          }
          if (!g) {
            groupCells += '<div class="quota-cell"><span class="quota-pct">\u2014</span></div>';
            continue;
          }
          var pct = Math.round(g.remainingPercent);
          var cls = "good";
          if (g.isExhausted || pct === 0) cls = "exhausted";
          else if (pct < 20) cls = "warning";
          else if (pct < 50) cls = "ok";
          var reset = "";
          if (g.timeUntilResetSec > 0) {
            reset = '<span class="quota-reset">\u21BB ' + formatSeconds(g.timeUntilResetSec) + "</span>";
          }
          var barCls = cls;
          var groupLabels = (modelsByGroup[key] || []).join("|||");
          var groupAdjust = '<span class="group-adjust" data-snap-id="' + acc.latestSnapshotId + '" data-group-key="' + key + '" data-group-labels="' + esc(groupLabels) + '" data-current-pct="' + pct + '"><button class="gadj-btn" data-delta="-5" title="\u22125% all models in group">\u22125</button><button class="gadj-btn" data-delta="5" title="+5% all models in group">+5</button></span>';
          var ttxBadge = "";
          if (data.forecasts && data.forecasts[acc.accountId]) {
            var acctForecasts = data.forecasts[acc.accountId];
            for (var fi = 0; fi < acctForecasts.length; fi++) {
              if (acctForecasts[fi].groupKey === key && acctForecasts[fi].ttxLabel) {
                var ttxSev = acctForecasts[fi].severity || "safe";
                var ttxLabel = acctForecasts[fi].ttxLabel;
                if (ttxLabel && ttxLabel !== "" && ttxSev !== "none") {
                  ttxBadge = '<span class="ttx-badge ttx-' + ttxSev + '" title="Time to exhaustion at current burn rate">' + esc(ttxLabel) + "</span>";
                }
                break;
              }
            }
          }
          var costBadge = "";
          if (pct < 95 && data.estimatedCosts && data.estimatedCosts[acc.accountId]) {
            var acctCosts = data.estimatedCosts[acc.accountId];
            if (acctCosts.groups) {
              for (var ci = 0; ci < acctCosts.groups.length; ci++) {
                if (acctCosts.groups[ci].groupKey === key && acctCosts.groups[ci].hasData) {
                  var costVal = acctCosts.groups[ci].estimatedCost || 0;
                  if (costVal < 0.01) break;
                  var costLabel = acctCosts.groups[ci].costLabel || "\u2014";
                  var costCls = "cost-low";
                  if (costVal >= 10) costCls = "cost-high";
                  else if (costVal >= 3) costCls = "cost-medium";
                  var costTitle = "Estimated cost this cycle";
                  if (acctCosts.groups[ci].hourlyLabel) {
                    costTitle += " (" + acctCosts.groups[ci].hourlyLabel + ")";
                  }
                  costBadge = '<span class="cost-badge ' + costCls + '" title="' + costTitle + '">' + esc(costLabel) + "</span>";
                  break;
                }
              }
            }
          }
          groupCells += '<div class="quota-cell"><span class="quota-pct ' + cls + '">' + pct + '%</span><div class="quota-minibar"><div class="quota-minibar-fill ' + barCls + '" style="width:' + pct + '%"></div></div>' + groupAdjust + reset + ttxBadge + costBadge + "</div>";
        }
        var dotCls = "dot-ready";
        var badgeText = "Ready";
        if (allExhausted(acc)) {
          dotCls = "dot-empty";
          badgeText = "Empty";
        } else if (!acc.isReady) {
          dotCls = "dot-low";
          badgeText = "Low";
        }
        var creditsCell = '<div class="credits-cell" style="position:relative">';
        if (acc.aiCredits && acc.aiCredits.length > 0) {
          var credits = acc.aiCredits[0].creditAmount;
          var creditCls = credits > 500 ? "good" : credits > 100 ? "ok" : "warning";
          creditsCell += '<span class="credit-amount ' + creditCls + '" title="AI Credits">\u2726 ' + formatCredits(credits) + "</span>";
          creditsCell += renderCreditRenewal(acc.accountId, acc.creditRenewalDay);
        } else {
          creditsCell += '<span class="credit-amount muted">\u2014</span>';
        }
        creditsCell += "</div>";
        var modelsHTML = "";
        if (acc.models && acc.models.length > 0) {
          var groupedModels = {};
          for (var mi = 0; mi < acc.models.length; mi++) {
            var m = acc.models[mi];
            var gk2 = m.groupKey || "claude_gpt";
            if (!groupedModels[gk2]) groupedModels[gk2] = [];
            groupedModels[gk2].push(m);
          }
          var modelRows = "";
          for (var goi = 0; goi < GROUP_ORDER.length; goi++) {
            var groupKey2 = GROUP_ORDER[goi];
            var groupModels = groupedModels[groupKey2];
            if (!groupModels || groupModels.length === 0) continue;
            var isPinned = pinnedKey === groupKey2;
            var starCls = isPinned ? "pin-star pinned" : "pin-star";
            var starTitle = isPinned ? "Pinned \u2014 click to unpin" : "Click to pin this group";
            var starChar = isPinned ? "\u2605" : "\u2606";
            modelRows += '<div class="model-group-header"><button class="' + starCls + '" data-pin-group="' + groupKey2 + '" data-pin-account="' + acc.accountId + '" title="' + starTitle + '">' + starChar + '</button><span class="model-group-name" style="color:' + (GROUP_COLORS[groupKey2] || "var(--text-secondary)") + '">' + (GROUP_NAMES[groupKey2] || groupKey2) + "</span></div>";
            for (var mi3 = 0; mi3 < groupModels.length; mi3++) {
              var m = groupModels[mi3];
              var mpct = Math.round(m.remainingPercent);
              var mcls = "good";
              if (m.isExhausted || mpct === 0) mcls = "exhausted";
              else if (mpct < 20) mcls = "warning";
              else if (mpct < 50) mcls = "ok";
              var color = GROUP_COLORS[m.groupKey] || "#94a3b8";
              var resetStr = m.resetSeconds > 0 ? "\u21BB " + formatSeconds(m.resetSeconds) : "";
              var intellBadges = "";
              if (usageDataCache && usageDataCache.models) {
                for (var ui = 0; ui < usageDataCache.models.length; ui++) {
                  var um = usageDataCache.models[ui];
                  if (um.modelId === m.modelId && um.hasIntelligence) {
                    var rateStr = (um.currentRate * 100).toFixed(1) + "%/hr";
                    intellBadges += '<span class="rate-badge" title="Current consumption rate">' + rateStr + "</span>";
                    if (um.projectedUsage > 0) {
                      var projPct = Math.round(um.projectedUsage * 100);
                      var projCls = projPct > 95 ? "proj-danger" : projPct > 80 ? "proj-warn" : "proj-ok";
                      intellBadges += '<span class="proj-badge ' + projCls + '" title="Projected usage at reset">\u2192' + projPct + "%</span>";
                    }
                    if (um.projectedExhaustion) {
                      var exhaust = new Date(um.projectedExhaustion);
                      var minsLeft = Math.round((exhaust.getTime() - Date.now()) / 6e4);
                      if (minsLeft > 0) {
                        intellBadges += '<span class="exhaust-badge" title="Projected exhaustion time">\u26A0 ' + (minsLeft > 60 ? Math.round(minsLeft / 60) + "h" : minsLeft + "m") + "</span>";
                      }
                    }
                    break;
                  }
                }
              }
              var adjustBtns = '<span class="adjust-controls" data-snap-id="' + acc.latestSnapshotId + '" data-model-label="' + esc(m.label || m.modelId) + '" data-current-pct="' + mpct + '"><button class="adj-btn" data-delta="-10" title="\u221210%">\u221210</button><button class="adj-btn" data-delta="-5" title="\u22125%">\u22125</button><button class="adj-btn" data-delta="5" title="+5%">+5</button><button class="adj-btn" data-delta="10" title="+10%">+10</button></span>';
              modelRows += '<div class="model-row"><div class="model-indicator" style="background:' + color + '"></div><span class="model-label">' + esc(m.label || m.modelId) + '</span><div class="model-bar-track"><div class="model-bar-fill ' + mcls + '" style="width:' + mpct + '%"></div></div><span class="model-pct ' + mcls + '">' + mpct + "%</span>" + adjustBtns + '<span class="model-reset">' + resetStr + "</span>" + intellBadges + "</div>";
            }
          }
          var expandedCls = isExpanded ? " is-expanded" : "";
          modelsHTML = '<div class="model-details' + expandedCls + '" id="' + accId + '">' + modelRows + '<div class="account-actions"><button class="btn-clear-snaps" data-clear-account="' + acc.accountId + '" data-clear-email="' + esc(acc.email) + '" title="Delete all snapshots for this account">Clear Snapshots</button><button class="btn-delete-account" data-delete-account="' + acc.accountId + '" data-delete-email="' + esc(acc.email) + '" title="Remove account and all its data">Remove Account</button></div></div>';
        }
        var chevronCls = isExpanded ? "chevron expanded" : "chevron";
        var staleStyle = "";
        if (!acc.isReady) {
          staleStyle = ' style="opacity:0.6"';
        }
        html += '<div class="account-card"' + staleStyle + '><div class="account-row" data-toggle="' + accId + '"><div class="account-info"><div class="account-email"><span class="' + chevronCls + '" id="chev-' + accId + '">\u25B8</span> ' + esc(acc.email) + renderPinnedBadge(pinnedGroupData, pinnedKey) + '</div><div class="account-meta" style="position:relative">' + (acc.planName ? '<span class="plan-badge">' + esc(acc.planName) + "</span>" : "") + renderAccountTags(acc) + renderAccountNote(acc) + "</div></div>" + groupCells + creditsCell + '<div class="snap-cell"><span class="snap-ago">' + esc(acc.stalenessLabel) + '</span></div><div style="text-align:center"><span class="health-dot ' + dotCls + '">\u25CF ' + badgeText + "</span></div></div>" + modelsHTML + "</div>";
      }
      html += "</div></div>";
    }
    var sf = document.getElementById("quota-filter-status");
    var statusVal = sf ? sf.value : "all";
    if (data.codexSnapshot && (pf === "all" || pf === "codex")) {
      var cxStatus = getCodexClaudeStatus(data.codexSnapshot);
      if (statusVal === "all" || cxStatus === statusVal) {
        html += renderCodexProviderSection(data.codexSnapshot);
      }
    }
    if (data.claudeSnapshot && (pf === "all" || pf === "claude")) {
      var clStatus = getCodexClaudeStatus(data.claudeSnapshot);
      if (statusVal === "all" || clStatus === statusVal) {
        html += renderClaudeProviderSection(data.claudeSnapshot);
      }
    }
    if (data.cursorSnapshot && (pf === "all" || pf === "cursor")) {
      var crStatus = getCursorStatus(data.cursorSnapshot);
      if (statusVal === "all" || crStatus === statusVal) {
        html += renderCursorProviderSection(data.cursorSnapshot);
      }
    }
    if (data.geminiSnapshot && (pf === "all" || pf === "gemini")) {
      var gmStatus = getGeminiStatus(data.geminiSnapshot);
      if (statusVal === "all" || gmStatus === statusVal) {
        html += renderGeminiProviderSection(data.geminiSnapshot);
      }
    }
    if (pf === "antigravity" && acctCount === 0) {
      html += '<div class="provider-empty-state" data-provider="antigravity"><span class="provider-empty-icon">\u26A1</span><p>No Antigravity accounts detected</p><p class="empty-hint">Open Windsurf and log in to start tracking quotas</p></div>';
    }
    if (pf === "codex" && !data.codexSnapshot) {
      html += '<div class="provider-empty-state" data-provider="codex"><span class="provider-empty-icon">\u{1F916}</span><p>No Codex snapshots yet</p><p class="empty-hint">Install Codex CLI and click <strong>Snap Now</strong> to capture</p></div>';
    }
    if (pf === "claude" && !data.claudeSnapshot) {
      html += '<div class="provider-empty-state" data-provider="claude"><span class="provider-empty-icon">\u{1F52E}</span><p>No Claude Code data yet</p><p class="empty-hint">Enable the Claude bridge in <strong>Settings</strong></p></div>';
    }
    if (pf === "cursor" && !data.cursorSnapshot) {
      html += '<div class="provider-empty-state" data-provider="cursor"><span class="provider-empty-icon">\u{1F5B1}\uFE0F</span><p>No Cursor data yet</p><p class="empty-hint">Enable Cursor capture in <strong>Settings</strong> or click <strong>Snap Now</strong></p></div>';
    }
    if (pf === "gemini" && !data.geminiSnapshot) {
      html += '<div class="provider-empty-state" data-provider="gemini"><span class="provider-empty-icon">\u2726</span><p>No Gemini CLI data yet</p><p class="empty-hint">Enable Gemini capture in <strong>Settings</strong> or click <strong>Snap Now</strong></p></div>';
    }
    grid.innerHTML = html;
    grid.querySelectorAll(".provider-header[data-toggle-provider]").forEach(function(hdr) {
      hdr.addEventListener("click", function() {
        var targetId = hdr.dataset.toggleProvider;
        var body = document.getElementById(targetId);
        var chev = document.getElementById("pchev-" + targetId);
        if (!body) return;
        var collapsed = body.classList.toggle("collapsed");
        if (chev) chev.textContent = collapsed ? "\u25B8" : "\u25BE";
        if (collapsed) {
          collapsedProviders.add(targetId);
        } else {
          collapsedProviders.delete(targetId);
        }
      });
    });
  }
  function renderCodexProviderSection(cs) {
    var fiveUsed = cs.fiveHourPct || 0;
    var fiveRem = Math.max(0, 100 - fiveUsed);
    var fiveCls = fiveRem > 50 ? "good" : fiveRem > 20 ? "ok" : fiveRem > 0 ? "warning" : "exhausted";
    var fiveReset = cs.fiveHourReset ? formatResetTime(cs.fiveHourReset) : "";
    var sevenUsed = cs.sevenDayPct ? cs.sevenDayPct : 0;
    var sevenRem = Math.max(0, 100 - sevenUsed);
    var sevenCls = sevenRem > 50 ? "good" : sevenRem > 20 ? "ok" : sevenRem > 0 ? "warning" : "exhausted";
    var sevenReset = cs.sevenDayReset ? formatResetTime(cs.sevenDayReset) : "";
    var capturedAgo = cs.capturedAt ? formatTimeAgo(cs.capturedAt) : "unknown";
    var dotCls = fiveUsed >= 80 || sevenUsed >= 80 ? "dot-low" : "dot-ready";
    var dotText = dotCls === "dot-ready" ? "Ready" : "Low";
    var displayName = cs.email || (cs.accountId && cs.accountId.length > 12 ? cs.accountId.substring(0, 6) + ".." + cs.accountId.slice(-6) : cs.accountId || "Codex");
    var creditsStr = cs.creditsBalance !== null && cs.creditsBalance !== void 0 ? cs.creditsBalance.toFixed(2) : String.fromCharCode(8212);
    var cxCollapseClass = collapsedProviders.has("section-codex") ? " collapsed" : "";
    var cxChevron = collapsedProviders.has("section-codex") ? "\u25B8" : "\u25BE";
    return '<div class="provider-section" data-provider="codex"><div class="provider-header" data-toggle-provider="section-codex"><div class="provider-header-left"><span class="provider-chevron" id="pchev-section-codex">' + cxChevron + '</span><span class="provider-name">\u{1F916} Codex / ChatGPT</span><span class="provider-count">1 account</span></div></div><div class="provider-body' + cxCollapseClass + '" id="section-codex"><div class="grid-header grid-codex"><div>Account</div><div>Plan</div><div>5-Hour</div><div>7-Day</div><div>Credits</div><div>Last Snap</div><div>Status</div></div><div class="account-card"><div class="account-row grid-codex"><div class="account-info"><div class="account-email">' + esc(displayName) + "</div></div><div>" + (cs.planType ? '<span class="plan-badge">' + esc(cs.planType) + "</span>" : String.fromCharCode(8212)) + '</div><div class="quota-cell"><span class="quota-pct ' + fiveCls + '">' + fiveRem.toFixed(0) + '%</span><div class="quota-minibar"><div class="quota-minibar-fill ' + fiveCls + '" style="width:' + fiveRem + '%"></div></div>' + (fiveReset ? '<span class="quota-reset">\u21BB ' + fiveReset + "</span>" : "") + '</div><div class="quota-cell"><span class="quota-pct ' + sevenCls + '">' + sevenRem.toFixed(0) + '%</span><div class="quota-minibar"><div class="quota-minibar-fill ' + sevenCls + '" style="width:' + sevenRem + '%"></div></div>' + (sevenReset ? '<span class="quota-reset">\u21BB ' + sevenReset + "</span>" : "") + '</div><div class="credits-cell"><span class="credit-amount">' + creditsStr + '</span></div><div class="snap-cell"><span class="snap-ago">' + capturedAgo + '</span></div><div style="text-align:center"><span class="health-dot ' + dotCls + '">\u25CF ' + dotText + "</span></div></div></div></div></div>";
  }
  function renderClaudeProviderSection(cl) {
    var clFive = cl.fiveHourPct || 0;
    var clFiveRem = Math.max(0, 100 - clFive);
    var clFiveCls = clFiveRem > 50 ? "good" : clFiveRem > 20 ? "ok" : clFiveRem > 0 ? "warning" : "exhausted";
    var clSeven = cl.sevenDayPct ? cl.sevenDayPct : 0;
    var clSevenRem = Math.max(0, 100 - clSeven);
    var clSevenCls = clSevenRem > 50 ? "good" : clSevenRem > 20 ? "ok" : clSevenRem > 0 ? "warning" : "exhausted";
    var clAgo = cl.capturedAt ? formatTimeAgo(cl.capturedAt) : "unknown";
    var dotCls = clFive >= 80 || clSeven >= 80 ? "dot-low" : "dot-ready";
    var dotText = dotCls === "dot-ready" ? "Ready" : "Low";
    var clCollapseClass = collapsedProviders.has("section-claude") ? " collapsed" : "";
    var clChevron = collapsedProviders.has("section-claude") ? "\u25B8" : "\u25BE";
    return '<div class="provider-section" data-provider="claude"><div class="provider-header" data-toggle-provider="section-claude"><div class="provider-header-left"><span class="provider-chevron" id="pchev-section-claude">' + clChevron + '</span><span class="provider-name">\u{1F517} Claude Code</span><span class="provider-count">1 account \xB7 Bridge</span></div></div><div class="provider-body' + clCollapseClass + '" id="section-claude"><div class="grid-header grid-claude"><div>Source</div><div>5-Hour</div><div>7-Day</div><div>Last Snap</div><div>Status</div></div><div class="account-card"><div class="account-row grid-claude"><div class="account-info"><div class="account-email">' + esc(cl.source || "statusline") + '</div></div><div class="quota-cell"><span class="quota-pct ' + clFiveCls + '">' + clFiveRem.toFixed(0) + '%</span><div class="quota-minibar"><div class="quota-minibar-fill ' + clFiveCls + '" style="width:' + clFiveRem + '%"></div></div></div><div class="quota-cell"><span class="quota-pct ' + clSevenCls + '">' + clSevenRem.toFixed(0) + '%</span><div class="quota-minibar"><div class="quota-minibar-fill ' + clSevenCls + '" style="width:' + clSevenRem + '%"></div></div></div><div class="snap-cell"><span class="snap-ago">' + clAgo + '</span></div><div style="text-align:center"><span class="health-dot ' + dotCls + '">\u25CF ' + dotText + "</span></div></div></div></div></div>";
  }
  function formatResetTime(isoString) {
    if (!isoString) return "";
    var reset = new Date(isoString);
    var now = /* @__PURE__ */ new Date();
    var diffSec = (reset.getTime() - now.getTime()) / 1e3;
    if (diffSec <= 0) return "now";
    return formatSeconds(diffSec);
  }
  function getCursorStatus(snap) {
    var usagePct = snap.usagePct || 0;
    var rem = Math.max(0, 100 - usagePct);
    if (rem === 0) return "empty";
    if (usagePct >= 80) return "low";
    return "ready";
  }
  function renderCursorProviderSection(cs) {
    var usagePct = cs.usagePct || 0;
    var remaining = Math.max(0, 100 - usagePct);
    var cls = remaining > 50 ? "good" : remaining > 20 ? "ok" : remaining > 0 ? "warning" : "exhausted";
    var capturedAgo = cs.capturedAt ? formatTimeAgo(cs.capturedAt) : "unknown";
    var dotCls = usagePct >= 80 ? "dot-low" : "dot-ready";
    var dotText = dotCls === "dot-ready" ? "Ready" : "Low";
    var displayName = cs.email || "Cursor";
    var usedStr = cs.premiumUsed !== void 0 ? cs.premiumUsed : String.fromCharCode(8212);
    var limitStr = cs.premiumLimit !== void 0 ? cs.premiumLimit : String.fromCharCode(8212);
    var crCollapseClass = collapsedProviders.has("section-cursor") ? " collapsed" : "";
    var crChevron = collapsedProviders.has("section-cursor") ? "\u25B8" : "\u25BE";
    var modelRows = "";
    if (cs.modelsJson && cs.modelsJson !== "{}") {
      try {
        var models = typeof cs.modelsJson === "string" ? JSON.parse(cs.modelsJson) : cs.modelsJson;
        var modelKeys = Object.keys(models);
        if (modelKeys.length > 0) {
          modelRows = '<div class="cursor-model-breakdown">';
          for (var mi = 0; mi < modelKeys.length; mi++) {
            var mKey = modelKeys[mi];
            var mVal = models[mKey];
            var mUsed = mVal.numRequests || 0;
            var mLimit = mVal.maxRequestUsage || 0;
            var mPct = mLimit > 0 ? mUsed / mLimit * 100 : 0;
            var mRem = Math.max(0, 100 - mPct);
            var mCls = mRem > 50 ? "good" : mRem > 20 ? "ok" : mRem > 0 ? "warning" : "exhausted";
            modelRows += '<div class="cursor-model-row"><span class="cursor-model-name">' + esc(mKey) + '</span><div class="quota-minibar"><div class="quota-minibar-fill ' + mCls + '" style="width:' + mRem + '%"></div></div><span class="cursor-model-usage">' + mUsed + "/" + mLimit + "</span></div>";
          }
          modelRows += "</div>";
        }
      } catch (e) {
      }
    }
    return '<div class="provider-section" data-provider="cursor"><div class="provider-header" data-toggle-provider="section-cursor"><div class="provider-header-left"><span class="provider-chevron" id="pchev-section-cursor">' + crChevron + '</span><span class="provider-name">\u{1F5B1}\uFE0F Cursor</span><span class="provider-count">1 account</span></div></div><div class="provider-body' + crCollapseClass + '" id="section-cursor"><div class="grid-header grid-cursor"><div>Account</div><div>Plan</div><div>Premium Used</div><div>Usage</div><div>Last Snap</div><div>Status</div></div><div class="account-card"><div class="account-row grid-cursor"><div class="account-info"><div class="account-email">' + esc(displayName) + "</div></div><div>" + (cs.planType ? '<span class="plan-badge">' + esc(cs.planType) + "</span>" : String.fromCharCode(8212)) + '</div><div class="quota-cell"><span class="quota-pct ' + cls + '">' + usedStr + " / " + limitStr + '</span><div class="quota-minibar"><div class="quota-minibar-fill ' + cls + '" style="width:' + remaining + '%"></div></div></div><div class="quota-cell"><span class="quota-pct ' + cls + '">' + remaining.toFixed(0) + '% left</span></div><div class="snap-cell"><span class="snap-ago">' + capturedAgo + '</span></div><div style="text-align:center"><span class="health-dot ' + dotCls + '">\u25CF ' + dotText + "</span></div></div>" + modelRows + "</div></div></div>";
  }
  function getGeminiStatus(snap) {
    var overallPct = snap.overallPct || 0;
    var rem = Math.max(0, 100 - overallPct);
    if (rem === 0) return "empty";
    if (overallPct >= 80) return "low";
    return "ready";
  }
  function renderGeminiProviderSection(gs) {
    var overallPct = gs.overallPct || 0;
    var remaining = Math.max(0, 100 - overallPct);
    var cls = remaining > 50 ? "good" : remaining > 20 ? "ok" : remaining > 0 ? "warning" : "exhausted";
    var capturedAgo = gs.capturedAt ? formatTimeAgo(gs.capturedAt) : "unknown";
    var dotCls = overallPct >= 80 ? "dot-low" : "dot-ready";
    var dotText = dotCls === "dot-ready" ? "Ready" : "Low";
    var displayName = gs.email || "Gemini CLI";
    var gmCollapseClass = collapsedProviders.has("section-gemini") ? " collapsed" : "";
    var gmChevron = collapsedProviders.has("section-gemini") ? "\u25B8" : "\u25BE";
    var modelRows = "";
    if (gs.modelsJson && gs.modelsJson !== "[]") {
      try {
        var models = typeof gs.modelsJson === "string" ? JSON.parse(gs.modelsJson) : gs.modelsJson;
        if (Array.isArray(models) && models.length > 0) {
          modelRows = '<div class="cursor-model-breakdown">';
          for (var mi = 0; mi < models.length; mi++) {
            var m = models[mi];
            var mUsedPct = m.usedPct || 0;
            var mRemPct = Math.max(0, 100 - mUsedPct);
            var mCls = mRemPct > 50 ? "good" : mRemPct > 20 ? "ok" : mRemPct > 0 ? "warning" : "exhausted";
            var mResetStr = m.resetTime ? formatResetTime(m.resetTime) : "";
            var tierLabel = m.tier || m.modelId || "unknown";
            modelRows += '<div class="cursor-model-row"><span class="cursor-model-name">' + esc(m.modelId || tierLabel) + '</span><div class="quota-minibar"><div class="quota-minibar-fill ' + mCls + '" style="width:' + mRemPct + '%"></div></div><span class="cursor-model-usage">' + mRemPct.toFixed(0) + "% left</span>" + (mResetStr ? '<span class="quota-reset">\u21BB ' + mResetStr + "</span>" : "") + "</div>";
          }
          modelRows += "</div>";
        }
      } catch (e) {
      }
    }
    return '<div class="provider-section" data-provider="gemini"><div class="provider-header" data-toggle-provider="section-gemini"><div class="provider-header-left"><span class="provider-chevron" id="pchev-section-gemini">' + gmChevron + '</span><span class="provider-name">\u2728 Gemini CLI</span><span class="provider-count">1 account</span></div></div><div class="provider-body' + gmCollapseClass + '" id="section-gemini"><div class="grid-header grid-gemini"><div>Account</div><div>Tier</div><div>Usage</div><div>Last Snap</div><div>Status</div></div><div class="account-card"><div class="account-row grid-gemini"><div class="account-info"><div class="account-email">' + esc(displayName) + "</div></div><div>" + (gs.tier ? '<span class="plan-badge">' + esc(gs.tier) + "</span>" : String.fromCharCode(8212)) + '</div><div class="quota-cell"><span class="quota-pct ' + cls + '">' + remaining.toFixed(0) + '% left</span><div class="quota-minibar"><div class="quota-minibar-fill ' + cls + '" style="width:' + remaining + '%"></div></div></div><div class="snap-cell"><span class="snap-ago">' + capturedAgo + '</span></div><div style="text-align:center"><span class="health-dot ' + dotCls + '">\u25CF ' + dotText + "</span></div></div>" + modelRows + "</div></div></div>";
  }

  // internal/web/src/quotas/expand.ts
  function setupToggle() {
    var grid = document.getElementById("account-grid");
    if (!grid) return;
    grid.addEventListener("click", function(e) {
      var clearBtn = e.target.closest("[data-clear-account]");
      if (clearBtn) {
        e.stopPropagation();
        var accountId = clearBtn.getAttribute("data-clear-account");
        var email = clearBtn.getAttribute("data-clear-email");
        if (confirm("Clear all snapshots for " + email + "?\n\nThe account will remain but all quota history will be deleted. This cannot be undone.")) {
          fetch("/api/accounts/" + accountId + "/snapshots", { method: "DELETE" }).then(function(res) {
            return res.json();
          }).then(function(data) {
            showToast("\u2705 Cleared " + (data.snapshotsDeleted || 0) + " snapshots for " + email, "success");
            fetchStatus().then(renderAccounts);
            document.dispatchEvent(new CustomEvent("niyantra:chart-refresh"));
          }).catch(function(err) {
            showToast("\u274C " + err.message, "error");
          });
        }
        return;
      }
      var deleteBtn = e.target.closest("[data-delete-account]");
      if (deleteBtn) {
        e.stopPropagation();
        var accountId2 = deleteBtn.getAttribute("data-delete-account");
        var email2 = deleteBtn.getAttribute("data-delete-email");
        if (confirm("Remove account " + email2 + "?\n\nThis will permanently delete the account and ALL associated data (snapshots, cycles, codex data). This cannot be undone.")) {
          fetch("/api/accounts/" + accountId2, { method: "DELETE" }).then(function(res) {
            return res.json();
          }).then(function(data) {
            showToast("\u2705 Removed " + email2 + " (" + (data.totalDeleted || 0) + " records deleted)", "success");
            expandedAccounts.delete("acc-" + accountId2);
            fetchStatus().then(renderAccounts);
            document.dispatchEvent(new CustomEvent("niyantra:chart-refresh"));
          }).catch(function(err) {
            showToast("\u274C " + err.message, "error");
          });
        }
        return;
      }
      var gadjBtn = e.target.closest(".gadj-btn");
      if (gadjBtn) {
        e.stopPropagation();
        var gControls = gadjBtn.closest(".group-adjust");
        if (!gControls) return;
        var gSnapId = parseInt(gControls.getAttribute("data-snap-id"), 10);
        var gGroupKey = gControls.getAttribute("data-group-key");
        var gLabelsStr = gControls.getAttribute("data-group-labels");
        var gCurrentPct = parseFloat(gControls.getAttribute("data-current-pct"));
        var gDelta = parseFloat(gadjBtn.getAttribute("data-delta"));
        var gNewPct = Math.max(0, Math.min(100, gCurrentPct + gDelta));
        var cell = gControls.closest(".quota-cell");
        if (cell) {
          var gPctSpan = cell.querySelector(".quota-pct");
          var gBarFill = cell.querySelector(".quota-minibar-fill");
          if (gPctSpan) {
            gPctSpan.textContent = Math.round(gNewPct) + "%";
            gPctSpan.className = "quota-pct " + (gNewPct <= 0 ? "exhausted" : gNewPct < 20 ? "warning" : gNewPct < 50 ? "ok" : "good");
          }
          if (gBarFill) {
            gBarFill.style.width = gNewPct + "%";
            gBarFill.className = "quota-minibar-fill " + (gNewPct <= 0 ? "exhausted" : gNewPct < 20 ? "warning" : gNewPct < 50 ? "ok" : "good");
          }
        }
        gControls.setAttribute("data-current-pct", String(gNewPct));
        var gLabels = gLabelsStr.split("|||").filter(function(l) {
          return l.length > 0;
        });
        var adjustments = [];
        for (var li = 0; li < gLabels.length; li++) {
          adjustments.push({ label: gLabels[li], remainingPercent: gNewPct });
        }
        if (adjustments.length === 0) return;
        fetch("/api/snap/adjust", {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ snapshotId: gSnapId, adjustments })
        }).then(function(res) {
          return res.json();
        }).then(function(data) {
          if (data.error) {
            showToast("\u274C " + data.error, "error");
            return;
          }
          var groupName = GROUP_NAMES[gGroupKey] || gGroupKey;
          showToast("\u270E " + groupName + " \u2192 " + Math.round(gNewPct) + "% (" + adjustments.length + " models)", "info");
          fetchStatus().then(renderAccounts);
        }).catch(function(err) {
          showToast("\u274C " + err.message, "error");
        });
        return;
      }
      var adjBtn = e.target.closest(".adj-btn");
      if (adjBtn) {
        e.stopPropagation();
        var controls = adjBtn.closest(".adjust-controls");
        if (!controls) return;
        var snapId = parseInt(controls.getAttribute("data-snap-id"), 10);
        var label = controls.getAttribute("data-model-label");
        var currentPct = parseFloat(controls.getAttribute("data-current-pct"));
        var delta = parseFloat(adjBtn.getAttribute("data-delta"));
        var newPct = Math.max(0, Math.min(100, currentPct + delta));
        var row = controls.closest(".model-row");
        if (row) {
          var pctSpan = row.querySelector(".model-pct");
          var barFill = row.querySelector(".model-bar-fill");
          if (pctSpan) {
            pctSpan.textContent = Math.round(newPct) + "%";
            pctSpan.className = "model-pct " + (newPct <= 0 ? "exhausted" : newPct < 20 ? "warning" : newPct < 50 ? "ok" : "good");
          }
          if (barFill) {
            barFill.style.width = newPct + "%";
            barFill.className = "model-bar-fill " + (newPct <= 0 ? "exhausted" : newPct < 20 ? "warning" : newPct < 50 ? "ok" : "good");
          }
        }
        controls.setAttribute("data-current-pct", String(newPct));
        fetch("/api/snap/adjust", {
          method: "PATCH",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            snapshotId: snapId,
            adjustments: [{ label, remainingPercent: newPct }]
          })
        }).then(function(res) {
          return res.json();
        }).then(function(data) {
          if (data.error) {
            showToast("\u274C " + data.error, "error");
            return;
          }
          showToast("\u270E Adjusted " + label + " \u2192 " + Math.round(newPct) + "%", "info");
          fetchStatus().then(renderAccounts);
        }).catch(function(err) {
          showToast("\u274C " + err.message, "error");
        });
        return;
      }
      if (e.target.closest("[data-tag-add]") || e.target.closest("[data-remove-tag]") || e.target.closest("[data-note-edit]") || e.target.closest("[data-pin-group]") || e.target.closest("[data-renewal-edit]") || e.target.closest(".tag-picker") || e.target.closest(".tag-chip")) {
        return;
      }
      var row = e.target.closest(".account-row[data-toggle]");
      if (!row) return;
      var id = row.getAttribute("data-toggle");
      var el = document.getElementById(id);
      var chev = document.getElementById("chev-" + id);
      if (!el) return;
      var willExpand = !el.classList.contains("is-expanded");
      el.classList.toggle("is-expanded", willExpand);
      if (willExpand) expandedAccounts.add(id);
      else expandedAccounts.delete(id);
      if (chev) chev.classList.toggle("expanded", willExpand);
    });
  }
  function initQuotas() {
    var qSearch = document.getElementById("quota-search");
    var qStatus = document.getElementById("quota-filter-status");
    if (qSearch) {
      qSearch.addEventListener("input", function() {
        if (latestQuotaData) renderAccounts(latestQuotaData);
      });
    }
    if (qStatus) {
      qStatus.addEventListener("change", function() {
        if (latestQuotaData) renderAccounts(latestQuotaData);
      });
    }
    var qProvider = document.getElementById("quota-filter-provider");
    if (qProvider) {
      qProvider.addEventListener("change", function() {
        if (latestQuotaData) renderAccounts(latestQuotaData);
      });
    }
    var gridEl = document.getElementById("account-grid");
    if (gridEl) {
      gridEl.addEventListener("click", function(e) {
        var el = e.target.closest(".sortable");
        if (!el) return;
        var col = el.dataset.sort;
        if (quotaSortState.column === col) {
          quotaSortState.direction = quotaSortState.direction === "asc" ? "desc" : "asc";
        } else {
          quotaSortState.column = col;
          quotaSortState.direction = "asc";
        }
        if (latestQuotaData) renderAccounts(latestQuotaData);
      });
    }
    var tagStrip = document.getElementById("tag-filter-strip");
    if (tagStrip) {
      tagStrip.addEventListener("click", handleTagFilterClick);
    }
  }

  // internal/web/src/subscriptions.ts
  function loadSubscriptions() {
    var status = document.getElementById("filter-status").value;
    var category = document.getElementById("filter-category").value;
    fetchSubscriptions(status, category).then(function(data) {
      renderSubscriptions(data);
    }).catch(function(err) {
      console.error("Failed to load subscriptions:", err);
    });
  }
  function renderSubscriptions(data) {
    var grid = document.getElementById("subs-grid");
    var summary = document.getElementById("subs-summary");
    if (!grid) return;
    var subs = data.subscriptions || [];
    summary.textContent = subs.length + " subscription" + (subs.length !== 1 ? "s" : "");
    if (subs.length === 0) {
      grid.innerHTML = '<div class="empty-state"><svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" opacity="0.4"><rect x="2" y="5" width="20" height="14" rx="2"/><path d="M2 10h20"/></svg><p>No subscriptions tracked yet</p><p class="empty-hint">Click <strong>+ Add</strong> to add your first AI subscription</p></div>';
      return;
    }
    var providerGroups = {};
    var manualSubs = [];
    var grandTotal = 0;
    for (var i = 0; i < subs.length; i++) {
      var s = subs[i];
      var monthly = 0;
      if (s.costAmount > 0) {
        if (s.billingCycle === "yearly") monthly = s.costAmount / 12;
        else monthly = s.costAmount;
      }
      grandTotal += monthly;
      if (s.autoTracked) {
        var pkey = s.platform || "Unknown";
        if (!providerGroups[pkey]) providerGroups[pkey] = { items: [], total: 0 };
        providerGroups[pkey].items.push(s);
        providerGroups[pkey].total += monthly;
      } else {
        manualSubs.push(s);
      }
    }
    var providerKeys = Object.keys(providerGroups);
    var autoCount = subs.length - manualSubs.length;
    var sym = currencySymbol(subs[0] ? subs[0].costCurrency : "USD");
    var html = '<div class="spend-summary-card"><div class="spend-hero"><div class="spend-amount">' + sym + grandTotal.toFixed(2) + '<span class="spend-period">/mo</span></div><div class="spend-label">Total Monthly Spend</div></div><div class="spend-breakdown">';
    var providerIcons = { "Antigravity": "\u26A1", "Codex": "\u{1F916}", "Claude": "\u{1F52E}" };
    for (var pk = 0; pk < providerKeys.length; pk++) {
      var pName = providerKeys[pk];
      var pIcon = providerIcons[pName] || "\u{1F4E6}";
      var pTotal = providerGroups[pName].total;
      html += '<span class="spend-chip">' + pIcon + " " + esc(pName) + " <strong>" + sym + pTotal.toFixed(2) + "</strong></span>";
    }
    if (manualSubs.length > 0) {
      var manualTotal = 0;
      for (var mi = 0; mi < manualSubs.length; mi++) {
        if (manualSubs[mi].costAmount > 0) {
          manualTotal += manualSubs[mi].billingCycle === "yearly" ? manualSubs[mi].costAmount / 12 : manualSubs[mi].costAmount;
        }
      }
      if (manualTotal > 0) {
        html += '<span class="spend-chip">\u{1F4CB} Manual <strong>' + sym + manualTotal.toFixed(2) + "</strong></span>";
      }
    }
    html += '</div><div class="spend-meta">' + autoCount + " auto-tracked \xB7 " + manualSubs.length + " manual</div></div>";
    for (var pi = 0; pi < providerKeys.length; pi++) {
      var provider = providerKeys[pi];
      var group = providerGroups[provider];
      var items = group.items;
      var icon = providerIcons[provider] || "\u{1F4E6}";
      var sectionId = "sub-provider-" + provider.replace(/\s+/g, "-").toLowerCase();
      var providerAttr = provider.toLowerCase().replace(/[^a-z]/g, "");
      html += '<div class="provider-section" data-provider="' + providerAttr + '"><div class="provider-header" data-toggle-provider="' + sectionId + '"><div class="provider-header-left"><span class="provider-chevron" id="pchev-' + sectionId + '">\u25BE</span> <span class="provider-icon">' + icon + '</span><span class="provider-name">' + esc(provider) + '</span><span class="provider-count">' + items.length + " account" + (items.length !== 1 ? "s" : "") + '</span></div><span class="provider-spend">' + sym + group.total.toFixed(2) + '/mo</span></div><div class="provider-body" id="' + sectionId + '"><div class="subs-card-grid">';
      for (var si = 0; si < items.length; si++) {
        html += renderSubCard(items[si]);
      }
      html += "</div></div></div>";
    }
    if (manualSubs.length > 0) {
      var grouped = {};
      for (var mi2 = 0; mi2 < manualSubs.length; mi2++) {
        var cat = manualSubs[mi2].category || "other";
        if (!grouped[cat]) grouped[cat] = [];
        grouped[cat].push(manualSubs[mi2]);
      }
      var catOrder = ["coding", "chat", "api", "image", "audio", "productivity", "other"];
      html += '<div class="sub-section-label">Manual Subscriptions (' + manualSubs.length + ")</div>";
      html += '<div class="subs-card-grid">';
      for (var ci = 0; ci < catOrder.length; ci++) {
        var catItems = grouped[catOrder[ci]];
        if (!catItems || catItems.length === 0) continue;
        for (var csi = 0; csi < catItems.length; csi++) {
          html += renderSubCard(catItems[csi]);
        }
      }
      html += "</div>";
    } else if (providerKeys.length > 0) {
      html += '<div class="sub-section-label">Manual Subscriptions</div><div class="manual-empty"><p>No manual subscriptions tracked.</p><p class="empty-hint">Click <strong>+ Add</strong> to track Claude Pro, Cursor, or other AI tools.</p></div>';
    }
    grid.innerHTML = html;
    grid.querySelectorAll(".provider-header").forEach(function(hdr) {
      hdr.addEventListener("click", function() {
        var targetId = hdr.dataset.toggleProvider;
        var body = document.getElementById(targetId);
        var chev = document.getElementById("pchev-" + targetId);
        if (!body) return;
        var collapsed = body.classList.toggle("collapsed");
        if (chev) chev.textContent = collapsed ? "\u25B8" : "\u25BE";
      });
    });
  }
  function renderSubCard(sub) {
    var costHTML = "";
    if (sub.costAmount > 0) {
      var sym = currencySymbol(sub.costCurrency);
      costHTML = '<div class="sub-card-cost">' + sym + sub.costAmount.toFixed(2) + ' <span class="cycle">/' + esc(sub.billingCycle) + "</span></div>";
    } else if (sub.billingCycle === "payg") {
      costHTML = '<div class="sub-card-cost">Pay-as-you-go</div>';
    }
    var limitsHTML = "";
    var chips = [];
    if (sub.tokenLimit > 0) chips.push(formatNumber(sub.tokenLimit) + " tokens/" + esc(sub.limitPeriod));
    if (sub.creditLimit > 0) chips.push(formatNumber(sub.creditLimit) + " credits/" + esc(sub.limitPeriod));
    if (sub.requestLimit > 0) chips.push(formatNumber(sub.requestLimit) + " requests/" + esc(sub.limitPeriod));
    if (chips.length > 0) {
      limitsHTML = '<div class="sub-card-limits">';
      for (var c = 0; c < chips.length; c++) {
        limitsHTML += '<span class="sub-limit-chip">' + chips[c] + "</span>";
      }
      limitsHTML += "</div>";
    }
    var badgesHTML = '<span class="sub-status-badge ' + esc(sub.status) + '">' + esc(sub.status) + "</span>";
    badgesHTML += '<span class="sub-cat-badge">' + esc(sub.category) + "</span>";
    if (sub.autoTracked) badgesHTML += '<span class="sub-auto-badge">AUTO</span>';
    var trialHTML = "";
    if (sub.daysUntilTrialEnd !== void 0 && sub.daysUntilTrialEnd !== null) {
      if (sub.daysUntilTrialEnd <= 0) {
        trialHTML = '<span class="trial-countdown">Trial expired!</span>';
      } else if (sub.daysUntilTrialEnd <= 7) {
        trialHTML = '<span class="trial-countdown">Trial ends in ' + sub.daysUntilTrialEnd + "d</span>";
      }
    }
    var renewalHTML = "";
    if (sub.nextRenewal && sub.daysUntilRenewal !== void 0) {
      var rCls = sub.daysUntilRenewal <= 7 ? "soon" : "";
      if (sub.daysUntilRenewal < 0) rCls = "overdue";
      renewalHTML = '<span class="sub-renewal-tag ' + rCls + '">Renews: ' + sub.nextRenewal + " (" + sub.daysUntilRenewal + "d)</span>";
    }
    var linksHTML = "";
    if (sub.url || sub.statusPageUrl) {
      linksHTML = '<div class="sub-card-links">';
      if (sub.url) linksHTML += '<a href="' + esc(sub.url) + '" target="_blank" rel="noopener">\u{1F517} Dashboard</a>';
      if (sub.statusPageUrl) linksHTML += '<a href="' + esc(sub.statusPageUrl) + '" target="_blank" rel="noopener">\u{1F7E2} Status</a>';
      linksHTML += "</div>";
    }
    var notesHTML = "";
    if (sub.notes) {
      notesHTML = '<div class="sub-card-notes">' + esc(sub.notes) + "</div>";
    }
    var metaParts = [];
    if (sub.email) metaParts.push(esc(sub.email));
    if (sub.planName) metaParts.push(esc(sub.planName));
    var metaHTML = metaParts.length > 0 ? '<div class="sub-card-meta">' + metaParts.join(" \xB7 ") + "</div>" : "";
    var cardTitle, cardSubtitle;
    if (sub.autoTracked && sub.email) {
      cardTitle = esc(sub.email);
      cardSubtitle = '<span class="sub-card-platform-badge">' + esc(sub.platform) + (sub.planName ? " \xB7 " + esc(sub.planName) : "") + "</span>";
    } else {
      cardTitle = esc(sub.platform);
      cardSubtitle = "";
    }
    if (sub.autoTracked) {
      badgesHTML = badgesHTML.replace(/<span[^>]*>AUTO<\/span>/i, "");
    }
    var colorSeed = (sub.platform || "") + (sub.email || "") + sub.id;
    var hue = 0;
    for (var ci = 0; ci < colorSeed.length; ci++) {
      hue = (hue + colorSeed.charCodeAt(ci) * 31) % 360;
    }
    var accentStyle = "border-left: 3px solid hsl(" + hue + ", 60%, 55%)";
    return '<div class="sub-card" data-sub-id="' + sub.id + '" style="' + accentStyle + '"><div class="sub-card-header"><div class="sub-card-title">' + cardTitle + '</div><div class="sub-card-badges">' + trialHTML + badgesHTML + "</div></div>" + (cardSubtitle ? '<div class="sub-card-subtitle">' + cardSubtitle + "</div>" : "") + metaHTML + costHTML + limitsHTML + notesHTML + linksHTML + renewalHTML + '<div class="sub-card-actions"><button class="btn-edit-card" data-edit-id="' + sub.id + '">Edit</button><button class="btn-delete-card" data-delete-id="' + sub.id + '" data-delete-name="' + esc(sub.platform) + '">Delete</button></div></div>';
  }
  function initModal() {
    var overlay = document.getElementById("modal-overlay");
    var closeBtn = document.getElementById("modal-close");
    var cancelBtn = document.getElementById("modal-cancel");
    var saveBtn = document.getElementById("modal-save");
    document.getElementById("add-sub-btn").addEventListener("click", function() {
      openModal();
    });
    document.getElementById("add-sub-btn-2").addEventListener("click", function() {
      openModal();
    });
    closeBtn.addEventListener("click", closeModal);
    cancelBtn.addEventListener("click", closeModal);
    overlay.addEventListener("click", function(e) {
      if (e.target === overlay) closeModal();
    });
    saveBtn.addEventListener("click", handleSave);
    document.getElementById("f-platform").addEventListener("input", function() {
      var val = this.value;
      for (var i = 0; i < presetsData.length; i++) {
        if (presetsData[i].platform === val) {
          fillFromPreset(presetsData[i]);
          break;
        }
      }
    });
    document.getElementById("subs-grid").addEventListener("click", function(e) {
      var editBtn = e.target.closest("[data-edit-id]");
      if (editBtn) {
        var id = parseInt(editBtn.getAttribute("data-edit-id"));
        openEditModal(id);
        return;
      }
      var deleteBtn = e.target.closest("[data-delete-id]");
      if (deleteBtn) {
        var deleteId = parseInt(deleteBtn.getAttribute("data-delete-id"));
        var deleteName = deleteBtn.getAttribute("data-delete-name");
        openDeleteConfirm(deleteId, deleteName);
      }
    });
    document.getElementById("delete-close").addEventListener("click", closeDelete);
    document.getElementById("delete-cancel").addEventListener("click", closeDelete);
    document.getElementById("delete-overlay").addEventListener("click", function(e) {
      if (e.target.id === "delete-overlay") closeDelete();
    });
    document.getElementById("filter-status").addEventListener("change", loadSubscriptions);
    document.getElementById("filter-category").addEventListener("change", loadSubscriptions);
  }
  function openModal(sub) {
    var overlay = document.getElementById("modal-overlay");
    var title = document.getElementById("modal-title");
    if (sub) {
      title.textContent = "Edit Subscription";
      document.getElementById("f-id").value = sub.id || "";
      document.getElementById("f-platform").value = sub.platform || "";
      document.getElementById("f-category").value = sub.category || "other";
      document.getElementById("f-status").value = sub.status || "active";
      document.getElementById("f-email").value = sub.email || "";
      document.getElementById("f-plan").value = sub.planName || "";
      document.getElementById("f-cost").value = sub.costAmount || "";
      document.getElementById("f-currency").value = sub.costCurrency || "USD";
      document.getElementById("f-cycle").value = sub.billingCycle || "monthly";
      document.getElementById("f-token-limit").value = sub.tokenLimit || "";
      document.getElementById("f-credit-limit").value = sub.creditLimit || "";
      document.getElementById("f-request-limit").value = sub.requestLimit || "";
      document.getElementById("f-limit-period").value = sub.limitPeriod || "monthly";
      document.getElementById("f-renewal").value = sub.nextRenewal || "";
      document.getElementById("f-trial-ends").value = sub.trialEndsAt || "";
      document.getElementById("f-url").value = sub.url || "";
      document.getElementById("f-notes").value = sub.notes || "";
      document.getElementById("f-status-page-url").value = sub.statusPageUrl || "";
      document.getElementById("f-auto-tracked").value = sub.autoTracked ? "1" : "0";
      document.getElementById("f-account-id").value = sub.accountId || "0";
    } else {
      title.textContent = "Add Subscription";
      document.getElementById("sub-modal").querySelectorAll("input, select, textarea").forEach(function(el) {
        if (el.type === "hidden") {
          el.value = "";
          return;
        }
        if (el.tagName === "SELECT") {
          el.selectedIndex = 0;
          return;
        }
        el.value = "";
      });
      document.getElementById("f-currency").value = "USD";
      document.getElementById("f-cycle").value = "monthly";
      document.getElementById("f-category").value = "coding";
      document.getElementById("f-limit-period").value = "monthly";
    }
    overlay.hidden = false;
    document.getElementById("f-platform").focus();
  }
  function closeModal() {
    document.getElementById("modal-overlay").hidden = true;
  }
  function fillFromPreset(preset) {
    document.getElementById("f-category").value = preset.category || "other";
    document.getElementById("f-cost").value = preset.costAmount || "";
    document.getElementById("f-cycle").value = preset.billingCycle || "monthly";
    document.getElementById("f-token-limit").value = preset.tokenLimit || "";
    document.getElementById("f-credit-limit").value = preset.creditLimit || "";
    document.getElementById("f-request-limit").value = preset.requestLimit || "";
    document.getElementById("f-limit-period").value = preset.limitPeriod || "monthly";
    document.getElementById("f-url").value = preset.url || "";
    document.getElementById("f-notes").value = preset.notes || "";
    document.getElementById("f-status-page-url").value = preset.statusPageUrl || "";
  }
  function openEditModal(id) {
    fetch("/api/subscriptions/" + id).then(function(res) {
      return res.json();
    }).then(function(sub) {
      openModal(sub);
    }).catch(function(err) {
      showToast("\u274C " + err.message, "error");
    });
  }
  function handleSave() {
    var id = document.getElementById("f-id").value;
    var sub = {
      platform: document.getElementById("f-platform").value.trim(),
      category: document.getElementById("f-category").value,
      status: document.getElementById("f-status").value,
      email: document.getElementById("f-email").value.trim(),
      planName: document.getElementById("f-plan").value.trim(),
      costAmount: parseFloat(document.getElementById("f-cost").value) || 0,
      costCurrency: document.getElementById("f-currency").value,
      billingCycle: document.getElementById("f-cycle").value,
      tokenLimit: parseInt(document.getElementById("f-token-limit").value) || 0,
      creditLimit: parseInt(document.getElementById("f-credit-limit").value) || 0,
      requestLimit: parseInt(document.getElementById("f-request-limit").value) || 0,
      limitPeriod: document.getElementById("f-limit-period").value,
      nextRenewal: document.getElementById("f-renewal").value,
      trialEndsAt: document.getElementById("f-trial-ends").value,
      url: document.getElementById("f-url").value.trim(),
      notes: document.getElementById("f-notes").value.trim(),
      statusPageUrl: document.getElementById("f-status-page-url").value,
      autoTracked: document.getElementById("f-auto-tracked").value === "1",
      accountId: parseInt(document.getElementById("f-account-id").value) || 0
    };
    if (!sub.platform) {
      showToast("\u274C Platform name is required", "error");
      return;
    }
    var saveBtn = document.getElementById("modal-save");
    saveBtn.disabled = true;
    saveBtn.textContent = "Saving...";
    var promise = id ? updateSubscription(parseInt(id), sub) : createSubscription(sub);
    promise.then(function(data) {
      showToast("\u2705 " + (id ? "Updated" : "Created") + ": " + sub.platform, "success");
      closeModal();
      loadSubscriptions();
    }).catch(function(err) {
      showToast("\u274C " + err.message, "error");
    }).finally(function() {
      saveBtn.disabled = false;
      saveBtn.textContent = "Save Subscription";
    });
  }
  var pendingDeleteId = null;
  function openDeleteConfirm(id, name) {
    pendingDeleteId = id;
    document.getElementById("delete-name").textContent = name;
    document.getElementById("delete-overlay").hidden = false;
    document.getElementById("delete-confirm").onclick = function() {
      deleteSubscription(pendingDeleteId).then(function() {
        showToast("\u2705 Deleted: " + name, "success");
        closeDelete();
        loadSubscriptions();
      }).catch(function(err) {
        showToast("\u274C " + err.message, "error");
      });
    };
  }
  function closeDelete() {
    document.getElementById("delete-overlay").hidden = true;
    pendingDeleteId = null;
  }
  function initSearch() {
    var searchEl = document.getElementById("search-subs");
    if (!searchEl) return;
    searchEl.addEventListener("input", function() {
      var query = searchEl.value.toLowerCase().trim();
      var cards = document.querySelectorAll(".sub-card");
      var labels = document.querySelectorAll(".sub-category-label");
      cards.forEach(function(card) {
        var text = card.textContent.toLowerCase();
        card.style.display = text.indexOf(query) >= 0 ? "" : "none";
      });
      labels.forEach(function(label) {
        var next = label.nextElementSibling;
        var anyVisible = false;
        while (next && !next.classList.contains("sub-category-label")) {
          if (next.classList.contains("sub-card") && next.style.display !== "none") {
            anyVisible = true;
          }
          next = next.nextElementSibling;
        }
        label.style.display = anyVisible ? "" : "none";
      });
    });
  }

  // internal/web/src/overview/budget.ts
  function getBudget() {
    return parseFloat(serverConfig["budget_monthly"] || "0");
  }
  function setBudget(amount) {
    serverConfig["budget_monthly"] = amount.toString();
    updateConfig("budget_monthly", amount.toString());
  }
  function updateConfig(key, value) {
    return fetch("/api/config", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ key, value })
    }).then(function(r) {
      return r.json();
    }).then(function(data) {
      if (data.config) {
      }
    }).catch(function(err) {
      console.error("Config update failed:", err);
    });
  }
  function loadConfig() {
    return fetch("/api/config").then(function(r) {
      return r.json();
    }).then(function(data) {
      if (data.config) {
        data.config.forEach(function(c) {
          serverConfig[c.key] = c.value;
        });
      }
    });
  }
  function initBudget() {
    document.getElementById("budget-close").addEventListener("click", closeBudget);
    document.getElementById("budget-cancel").addEventListener("click", closeBudget);
    document.getElementById("budget-overlay").addEventListener("click", function(e) {
      if (e.target.id === "budget-overlay") closeBudget();
    });
    document.getElementById("budget-save").addEventListener("click", function() {
      var val = parseFloat(document.getElementById("f-budget").value) || 0;
      setBudget(val);
      closeBudget();
      showToast("\u2705 Budget set to $" + val.toFixed(0) + "/mo", "success");
      var overviewPanel = document.getElementById("panel-overview");
      if (overviewPanel && overviewPanel.classList.contains("active")) document.dispatchEvent(new CustomEvent("niyantra:overview-refresh"));
    });
  }
  function openBudgetModal() {
    document.getElementById("f-budget").value = String(getBudget() || "");
    document.getElementById("budget-overlay").hidden = false;
  }
  function closeBudget() {
    document.getElementById("budget-overlay").hidden = true;
  }

  // internal/web/src/overview/insights.ts
  function renderServerInsights(insights) {
    if (!insights || insights.length === 0) return "";
    var html = '<div class="insight-panel"><h3>\u{1F9E0} Intelligence Insights</h3><div class="insight-list">';
    var iconMap = {
      renewal_imminent: "\u{1F534}",
      trial_expiring: "\u23F3",
      unused_subscription: "\u{1F4A4}",
      spending_anomaly: "\u{1F4C8}",
      category_overlap: "\u{1F501}",
      annual_savings: "\u{1F4A1}",
      budget_exceeded: "\u{1F6A8}"
    };
    for (var i = 0; i < insights.length; i++) {
      var ins = insights[i];
      var icon = iconMap[ins.type] || "\u{1F4A1}";
      var cls = ins.severity === "critical" ? "critical" : ins.severity === "warning" ? "warning" : "info";
      html += '<div class="insight-item ' + cls + '"><span class="insight-item-icon">' + icon + '</span><div class="insight-item-content"><div class="insight-item-title">' + esc(ins.type.replace(/_/g, " ")) + '</div><div class="insight-item-msg">' + esc(ins.message) + "</div></div></div>";
    }
    html += "</div></div>";
    return html;
  }
  var advisorGroupPref = localStorage.getItem("niyantra_advisor_group") || "claude_gpt";
  function loadAdvisorCard() {
    var container = document.getElementById("advisor-card-container");
    if (!container) return;
    if (!latestQuotaData || !latestQuotaData.accounts || latestQuotaData.accounts.length < 2) {
      container.innerHTML = "";
      return;
    }
    renderAdvisorWithGroup(container, advisorGroupPref);
  }
  function renderAdvisorWithGroup(container, groupKey) {
    var accounts = latestQuotaData.accounts;
    var ranked = [];
    for (var i = 0; i < accounts.length; i++) {
      var acc = accounts[i];
      var groups = acc.groups || [];
      var pct = null;
      for (var g = 0; g < groups.length; g++) {
        if (groups[g].groupKey === groupKey) {
          pct = Math.round(groups[g].remainingPercent);
          break;
        }
      }
      if (pct === null) {
        if (groupKey === "all") {
          var total = 0;
          for (var gx = 0; gx < groups.length; gx++) total += groups[gx].remainingPercent;
          pct = groups.length > 0 ? Math.round(total / groups.length) : 0;
        } else {
          pct = 0;
        }
      }
      var isStale = false;
      if (acc.lastSeen) {
        var ageMs = Date.now() - new Date(acc.lastSeen).getTime();
        isStale = ageMs > 6 * 3600 * 1e3;
      }
      ranked.push({
        email: acc.email,
        pct,
        stale: isStale,
        label: acc.stalenessLabel || ""
      });
    }
    ranked.sort(function(a, b) {
      return b.pct - a.pct;
    });
    var groupNames = {
      "claude_gpt": "Claude + GPT",
      "gemini_pro": "Gemini Pro",
      "gemini_flash": "Gemini Flash",
      "all": "All Models (avg)"
    };
    var best = ranked[0];
    var worst = ranked[ranked.length - 1];
    var allHealthy = ranked.every(function(a) {
      return a.pct > 80;
    });
    var actionIcon = allHealthy ? "\u2705" : best.pct > 20 ? "\u26A1" : "\u23F3";
    var actionLabel = allHealthy ? "ALL READY" : best.pct > 20 ? "SWITCH" : "WAIT";
    var bestLabel = best.email.split("@")[0] + "@...";
    var html = '<div class="advisor-card"><h3>\u26A1 Antigravity Account Advisor</h3><div class="advisor-group-select"><label>Optimize for:</label><select id="advisor-group-filter" class="filter-select" style="margin-left:8px;font-size:12px"><option value="claude_gpt"' + (groupKey === "claude_gpt" ? " selected" : "") + '>Claude + GPT</option><option value="gemini_pro"' + (groupKey === "gemini_pro" ? " selected" : "") + '>Gemini Pro</option><option value="gemini_flash"' + (groupKey === "gemini_flash" ? " selected" : "") + '>Gemini Flash</option><option value="all"' + (groupKey === "all" ? " selected" : "") + ">All Models (avg)</option></select></div>";
    var actionCls = allHealthy ? "stay" : best.pct > 20 ? "switch" : "wait";
    html += '<div class="advisor-action ' + actionCls + '">' + actionIcon + " " + actionLabel + '</div><div class="advisor-reason">' + (allHealthy ? "All accounts have healthy quotas \u2014 no switch needed" : "Best: " + esc(best.email) + " (" + best.pct + "% " + esc(groupNames[groupKey] || groupKey) + " remaining)") + (best.stale ? " \u26A0\uFE0F stale data" : "") + "</div>";
    html += '<div class="advisor-scores">';
    var initialShow = Math.min(ranked.length, 5);
    for (var s = 0; s < ranked.length; s++) {
      var acct = ranked[s];
      var isBest = s === 0;
      var barCls = acct.pct > 50 ? "good" : acct.pct > 20 ? "ok" : "low";
      var staleIcon = acct.stale ? ' <span class="stale-icon" title="Data ' + esc(acct.label) + '">\u26A0</span>' : "";
      var hidden = s >= initialShow ? ' style="display:none" data-advisor-extra' : "";
      html += '<div class="advisor-score-row' + (isBest ? " best" : "") + '"' + hidden + '><span class="advisor-score-email" title="' + esc(acct.email) + '">' + esc(acct.email) + '</span><div class="advisor-score-bar"><div class="advisor-score-fill ' + barCls + '" style="width:' + acct.pct + '%"></div></div><span class="advisor-score-val">' + acct.pct + "%" + staleIcon + "</span></div>";
    }
    if (ranked.length > initialShow) {
      html += '<button class="advisor-show-all" id="advisor-toggle-all">Show all ' + ranked.length + " accounts</button>";
    }
    html += "</div></div>";
    container.innerHTML = html;
    var sel = document.getElementById("advisor-group-filter");
    if (sel) {
      sel.addEventListener("change", function() {
        advisorGroupPref = sel.value;
        localStorage.setItem("niyantra_advisor_group", advisorGroupPref);
        renderAdvisorWithGroup(container, advisorGroupPref);
      });
    }
    var toggleBtn = document.getElementById("advisor-toggle-all");
    if (toggleBtn) {
      toggleBtn.addEventListener("click", function() {
        var extras = container.querySelectorAll("[data-advisor-extra]");
        var showing = toggleBtn.textContent.indexOf("Hide") >= 0;
        extras.forEach(function(el) {
          el.style.display = showing ? "none" : "";
        });
        toggleBtn.textContent = showing ? "Show all " + ranked.length + " accounts" : "Hide extras";
      });
    }
  }

  // internal/web/src/overview/cost.ts
  function loadCostKPI() {
    var container = document.getElementById("cost-kpi-container");
    if (!container) return;
    fetch("/api/cost").then(function(res) {
      return res.json();
    }).then(function(data) {
      if (!data || !data.accounts || data.accounts.length === 0) {
        container.innerHTML = "";
        return;
      }
      var total = data.totalCost || 0;
      if (total < 0.01) {
        container.innerHTML = "";
        return;
      }
      var totalLabel = data.totalLabel || "$0.00";
      var html = '<div class="cost-kpi-card overview-card"><h3>Estimated Spend (Current Cycle)</h3><div class="cost-kpi-amount">' + esc(totalLabel) + '</div><div class="cost-kpi-label">Estimated cost based on quota consumption \xD7 model pricing</div>';
      var hasChips = false;
      var chipsHTML = '<div class="cost-kpi-breakdown">';
      if (data.accounts && data.accounts.length > 0) {
        for (var i = 0; i < data.accounts.length; i++) {
          var acct = data.accounts[i];
          if (acct.totalCost >= 0.01) {
            hasChips = true;
            var emailShort = acct.email;
            if (emailShort && emailShort.length > 20) {
              emailShort = emailShort.split("@")[0] + "@\u2026";
            }
            chipsHTML += '<span class="cost-kpi-chip" title="' + esc(acct.email) + '">' + esc(emailShort) + ": " + esc(acct.totalLabel) + "</span>";
          }
        }
      }
      chipsHTML += "</div>";
      if (hasChips) html += chipsHTML;
      html += "</div>";
      container.innerHTML = html;
    }).catch(function(err) {
      console.error("Cost KPI fetch failed:", err);
      container.innerHTML = "";
    });
  }

  // internal/web/src/overview/heatmap.ts
  function loadHeatmap() {
    var container = document.getElementById("heatmap-container");
    if (!container) return;
    fetch("/api/history/heatmap?days=365").then(function(res) {
      return res.json();
    }).then(function(data) {
      if (!data) {
        container.innerHTML = "";
        return;
      }
      renderHeatmap(container, data);
    }).catch(function(err) {
      console.error("Heatmap fetch failed:", err);
      container.innerHTML = "";
    });
  }
  function renderHeatmap(container, data) {
    var days = data.days || [];
    var maxCount = data.maxCount || 1;
    var dayMap = {};
    for (var i = 0; i < days.length; i++) {
      dayMap[days[i].date] = days[i];
    }
    var today = /* @__PURE__ */ new Date();
    var startDate = new Date(today);
    startDate.setDate(startDate.getDate() - 364);
    var dayOfWeek = startDate.getDay();
    startDate.setDate(startDate.getDate() - dayOfWeek);
    var totalDays = Math.ceil((today.getTime() - startDate.getTime()) / (1e3 * 60 * 60 * 24)) + 1;
    var totalWeeks = Math.ceil(totalDays / 7);
    var monthLabels = [];
    var lastMonth = -1;
    var monthNames = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    var cellsHTML = "";
    for (var w = 0; w < totalWeeks; w++) {
      for (var d = 0; d < 7; d++) {
        var cellDate = new Date(startDate);
        cellDate.setDate(startDate.getDate() + w * 7 + d);
        if (cellDate > today) {
          cellsHTML += '<div class="heatmap-cell heatmap-empty"></div>';
          continue;
        }
        var dateStr = formatDateISO(cellDate);
        var entry = dayMap[dateStr];
        var count = entry ? entry.count : 0;
        var level = getIntensityLevel(count, maxCount);
        var month = cellDate.getMonth();
        if (month !== lastMonth && d === 0) {
          monthLabels.push({ label: monthNames[month], col: w });
          lastMonth = month;
        }
        var tooltip = formatDateHuman(cellDate) + ": ";
        if (count === 0) {
          tooltip += "No activity";
        } else {
          tooltip += count + " snapshot" + (count !== 1 ? "s" : "");
          if (entry) {
            var parts = [];
            if (entry.antigravity > 0) parts.push(entry.antigravity + " AG");
            if (entry.claude > 0) parts.push(entry.claude + " Claude");
            if (entry.codex > 0) parts.push(entry.codex + " Codex");
            if (parts.length > 0) tooltip += " (" + parts.join(", ") + ")";
          }
        }
        cellsHTML += '<div class="heatmap-cell heatmap-level-' + level + '" data-date="' + dateStr + '" data-count="' + count + '" aria-label="' + tooltip + '" title="' + tooltip + '"></div>';
      }
    }
    var monthLabelHTML = '<div class="heatmap-month-labels" style="grid-template-columns: 28px repeat(' + totalWeeks + ', 1fr)">';
    monthLabelHTML += "<div></div>";
    var lastCol = -2;
    for (var m = 0; m < monthLabels.length; m++) {
      if (monthLabels[m].col > lastCol + 2) {
        monthLabelHTML += '<div class="heatmap-month" style="grid-column: ' + (monthLabels[m].col + 2) + '">' + monthLabels[m].label + "</div>";
        lastCol = monthLabels[m].col;
      }
    }
    monthLabelHTML += "</div>";
    var statsHTML = '<div class="heatmap-stats"><span class="heatmap-stat"><span class="heatmap-stat-value">' + data.totalSnapshots + '</span><span class="heatmap-stat-label">snapshots</span></span><span class="heatmap-stat"><span class="heatmap-stat-value">' + data.activeDays + '</span><span class="heatmap-stat-label">active days</span></span><span class="heatmap-stat"><span class="heatmap-stat-value">' + data.streak + 'd</span><span class="heatmap-stat-label">current streak</span></span><span class="heatmap-stat"><span class="heatmap-stat-value">' + data.longestStreak + 'd</span><span class="heatmap-stat-label">longest streak</span></span></div>';
    var legendHTML = '<div class="heatmap-legend"><span class="heatmap-legend-label">Less</span><div class="heatmap-cell heatmap-level-0 heatmap-legend-cell"></div><div class="heatmap-cell heatmap-level-1 heatmap-legend-cell"></div><div class="heatmap-cell heatmap-level-2 heatmap-legend-cell"></div><div class="heatmap-cell heatmap-level-3 heatmap-legend-cell"></div><div class="heatmap-cell heatmap-level-4 heatmap-legend-cell"></div><span class="heatmap-legend-label">More</span></div>';
    var dayLabels = '<div class="heatmap-day-labels"><div></div><div class="heatmap-day-label">Mon</div><div></div><div class="heatmap-day-label">Wed</div><div></div><div class="heatmap-day-label">Fri</div><div></div></div>';
    var gridHTML = '<div class="heatmap-scroll"><div class="heatmap-body">' + dayLabels + '<div class="heatmap-grid" style="grid-template-columns: repeat(' + totalWeeks + ', 1fr)">' + cellsHTML + "</div></div></div>";
    container.innerHTML = "<h3>Activity</h3>" + statsHTML + monthLabelHTML + gridHTML + '<div class="heatmap-footer">' + legendHTML + "</div>";
  }
  function getIntensityLevel(count, max) {
    if (count === 0) return 0;
    if (max <= 1) return 4;
    var ratio = count / max;
    if (ratio <= 0.25) return 1;
    if (ratio <= 0.5) return 2;
    if (ratio <= 0.75) return 3;
    return 4;
  }
  function formatDateISO(d) {
    var y = d.getFullYear();
    var m = (d.getMonth() + 1).toString().padStart(2, "0");
    var day = d.getDate().toString().padStart(2, "0");
    return y + "-" + m + "-" + day;
  }
  function formatDateHuman(d) {
    var months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
    var days = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
    return days[d.getDay()] + ", " + months[d.getMonth()] + " " + d.getDate() + ", " + d.getFullYear();
  }

  // internal/web/src/overview/calendar.ts
  var calendarViewDate = /* @__PURE__ */ new Date();
  function renderRenewalCalendar(renewals, subs) {
    var container = document.getElementById("renewal-calendar-container");
    if (!container) return;
    var renewalMap = {};
    if (renewals) {
      for (var i = 0; i < renewals.length; i++) {
        var r = renewals[i];
        var dateKey = r.nextRenewal;
        if (!renewalMap[dateKey]) renewalMap[dateKey] = [];
        var cat = "other";
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
    var today = /* @__PURE__ */ new Date();
    var todayKey = today.getFullYear() + "-" + String(today.getMonth() + 1).padStart(2, "0") + "-" + String(today.getDate()).padStart(2, "0");
    var monthNames = [
      "January",
      "February",
      "March",
      "April",
      "May",
      "June",
      "July",
      "August",
      "September",
      "October",
      "November",
      "December"
    ];
    var dayNames = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
    var firstDay = new Date(year, month, 1).getDay();
    var daysInMonth = new Date(year, month + 1, 0).getDate();
    var prevDays = new Date(year, month, 0).getDate();
    var html = '<div class="calendar-container"><div class="calendar-header"><h3>\u{1F4C5} Renewal Calendar</h3><div class="calendar-nav"><button class="calendar-nav-btn" onclick="calendarNav(-1)">\u2039</button><span class="calendar-month-label">' + monthNames[month] + " " + year + '</span><button class="calendar-nav-btn" onclick="calendarNav(1)">\u203A</button></div></div>';
    html += '<div class="calendar-weekdays">';
    for (var d = 0; d < 7; d++) {
      html += '<div class="calendar-weekday">' + dayNames[d] + "</div>";
    }
    html += "</div>";
    html += '<div class="calendar-grid">';
    for (var p = firstDay - 1; p >= 0; p--) {
      html += '<div class="calendar-day other-month"><span class="calendar-day-num">' + (prevDays - p) + "</span></div>";
    }
    for (var day = 1; day <= daysInMonth; day++) {
      dateKey = year + "-" + String(month + 1).padStart(2, "0") + "-" + String(day).padStart(2, "0");
      var isToday = dateKey === todayKey;
      var dayClass = isToday ? "calendar-day today" : "calendar-day";
      var events = renewalMap[dateKey];
      html += '<div class="' + dayClass + '"';
      if (events && events.length > 0) {
        var tooltipText = events.map(function(e2) {
          return e2.platform;
        }).join(", ");
        html += ' title="' + esc(tooltipText) + '"';
      }
      html += ">";
      html += '<span class="calendar-day-num">' + day + "</span>";
      if (events && events.length > 0) {
        html += '<div class="calendar-pins">';
        for (var e = 0; e < Math.min(events.length, 4); e++) {
          html += '<span class="calendar-pin ' + esc(events[e].category) + '"></span>';
        }
        html += "</div>";
      }
      html += "</div>";
    }
    var totalCells = firstDay + daysInMonth;
    var remaining = 7 - totalCells % 7;
    if (remaining < 7) {
      for (var n = 1; n <= remaining; n++) {
        html += '<div class="calendar-day other-month"><span class="calendar-day-num">' + n + "</span></div>";
      }
    }
    html += "</div>";
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
        html += '<div class="calendar-legend-item"><span class="calendar-legend-dot ' + esc(catKeys[cl]) + '"></span>' + esc(catKeys[cl]) + "</div>";
      }
      html += "</div>";
    }
    html += "</div>";
    container.innerHTML = html;
  }
  function calendarNav(delta) {
    calendarViewDate.setMonth(calendarViewDate.getMonth() + delta);
    var el = document.getElementById("renewal-calendar-container");
    if (el) {
      document.dispatchEvent(new CustomEvent("niyantra:overview-refresh"));
    }
  }

  // internal/web/src/advanced/claude.ts
  function loadClaudeBridgeStatus() {
    fetch("/api/claude/status").then(function(r) {
      return r.json();
    }).then(function(data) {
      var statusEl = document.getElementById("claude-bridge-status");
      if (!statusEl) return;
      var bridgeOn = data.bridgeEnabled;
      var installed = data.installed;
      if (!bridgeOn) {
        statusEl.style.display = "none";
        return;
      }
      var msg = "";
      if (!installed) {
        msg = "\u26A0\uFE0F Claude Code not detected (~/.claude/ not found)";
      } else if (data.bridgeFresh) {
        msg = '<span class="claude-bridge-dot"></span> Bridge active';
        if (data.snapshot) {
          msg += " \xB7 5h: " + data.snapshot.fiveHourPct.toFixed(1) + "% used";
        }
      } else if (data.snapshot) {
        msg = '<span class="claude-bridge-dot stale"></span> Last data: ' + formatTimeAgo(data.snapshot.capturedAt);
      } else {
        msg = '<span class="claude-bridge-dot off"></span> Waiting for Claude Code statusline data...';
      }
      statusEl.innerHTML = msg;
      statusEl.style.display = "";
    }).catch(function() {
    });
  }
  function renderClaudeCodeCard() {
    return '<div class="claude-card" id="claude-code-card"><h3>\u{1F517} Claude Code</h3><div id="claude-card-body"><div class="empty-hint">Loading...</div></div><div id="claude-deep-usage" class="claude-deep-section"></div></div>';
  }
  function loadClaudeCardData() {
    fetch("/api/claude/status").then(function(r) {
      return r.json();
    }).then(function(data) {
      var body = document.getElementById("claude-card-body");
      if (!body) return;
      if (!data.snapshot) {
        body.innerHTML = '<div class="empty-hint">No Claude Code data yet. Start a Claude Code session to see rate limits.</div>';
        return;
      }
      var snap = data.snapshot;
      var html = "";
      var fiveColor = meterColor(snap.fiveHourPct);
      var fiveReset = snap.fiveHourReset ? "\u21BB " + formatResetTime(snap.fiveHourReset) : "";
      html += '<div class="claude-meter"><span class="claude-meter-label">5-Hour</span><div class="claude-meter-track"><div class="claude-meter-fill" style="width:' + snap.fiveHourPct + "%;background:" + fiveColor + '"></div></div><span class="claude-meter-pct" style="color:' + fiveColor + '">' + snap.fiveHourPct.toFixed(1) + '%</span><span class="claude-meter-reset">' + fiveReset + "</span></div>";
      if (snap.sevenDayPct !== void 0) {
        var sevenColor = meterColor(snap.sevenDayPct);
        var sevenReset = snap.sevenDayReset ? "\u21BB " + formatResetTime(snap.sevenDayReset) : "";
        html += '<div class="claude-meter"><span class="claude-meter-label">7-Day</span><div class="claude-meter-track"><div class="claude-meter-fill" style="width:' + snap.sevenDayPct + "%;background:" + sevenColor + '"></div></div><span class="claude-meter-pct" style="color:' + sevenColor + '">' + snap.sevenDayPct.toFixed(1) + '%</span><span class="claude-meter-reset">' + sevenReset + "</span></div>";
      }
      var dotCls = data.bridgeFresh ? "" : "stale";
      var agoStr = formatTimeAgo(snap.capturedAt);
      html += '<div class="claude-bridge-badge"><span class="claude-bridge-dot ' + dotCls + '"></span>Bridge ' + (data.bridgeFresh ? "active" : "stale") + " \xB7 Last: " + agoStr + "</div>";
      body.innerHTML = html;
    }).catch(function() {
    });
  }
  function meterColor(pct) {
    if (pct >= 80) return "var(--red)";
    if (pct >= 50) return "var(--amber)";
    return "var(--green)";
  }
  function loadClaudeDeepUsage() {
    fetch("/api/claude/usage?days=30").then(function(r) {
      return r.json();
    }).then(function(data) {
      var container = document.getElementById("claude-deep-usage");
      if (!container) return;
      if (!data || !data.days || data.days.length === 0) {
        container.innerHTML = '<div class="empty-hint">No Claude Code session data found. Start coding with Claude Code to see token analytics.</div>';
        return;
      }
      var html = "";
      html += '<div class="claude-deep-stats">';
      html += '<div class="claude-deep-stat"><span class="claude-deep-value">' + formatTokens(data.totalTokens) + '</span><span class="claude-deep-label">tokens (30d)</span></div>';
      html += '<div class="claude-deep-stat"><span class="claude-deep-value">$' + (data.totalCost || 0).toFixed(2) + '</span><span class="claude-deep-label">est. cost</span></div>';
      html += '<div class="claude-deep-stat"><span class="claude-deep-value">' + (data.totalSessions || 0) + '</span><span class="claude-deep-label">sessions</span></div>';
      html += '<div class="claude-deep-stat"><span class="claude-deep-value">' + ((data.cacheHitRate || 0) * 100).toFixed(0) + '%</span><span class="claude-deep-label">cache hit</span></div>';
      html += "</div>";
      var totalIn = data.totalInput || 0;
      var totalOut = data.totalOutput || 0;
      var totalAll = totalIn + totalOut;
      if (totalAll > 0) {
        var inPct = (totalIn / totalAll * 100).toFixed(0);
        var outPct = (totalOut / totalAll * 100).toFixed(0);
        html += '<div class="claude-token-bar"><div class="claude-token-in" style="width:' + inPct + '%"><span>In ' + formatTokens(totalIn) + '</span></div><div class="claude-token-out" style="width:' + outPct + '%"><span>Out ' + formatTokens(totalOut) + "</span></div></div>";
      }
      if (data.topModel) {
        html += '<div class="claude-deep-meta"><span class="claude-deep-chip">\u{1F3C6} ' + data.topModel + "</span></div>";
      }
      container.innerHTML = html;
    }).catch(function() {
    });
  }
  function formatTokens(n) {
    if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
    if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
    return n.toString();
  }

  // internal/web/src/advanced/codex.ts
  function loadCodexSettingsStatus() {
    var statusEl = document.getElementById("codex-status-settings");
    if (!statusEl) return;
    fetch("/api/codex/status").then(function(r) {
      return r.json();
    }).then(function(data) {
      statusEl.style.display = "";
      if (!data.installed) {
        statusEl.innerHTML = '<span style="color:var(--text-muted)">\u26A0\uFE0F Codex CLI not detected. Install <a href="https://github.com/openai/codex" target="_blank" style="color:var(--accent)">Codex</a> and run <code>codex auth</code> to enable.</span>';
        return;
      }
      var tokenStatus = data.tokenExpired ? '<span style="color:var(--warning)">\u26A0\uFE0F Token expired \u2014 will auto-refresh on next poll</span>' : '<span style="color:var(--success)">\u2705 Token valid (expires ' + (data.tokenExpiresIn || "?") + ")</span>";
      var displayId = data.email || (data.accountId && data.accountId.length > 12 ? data.accountId.substring(0, 6) + "\u2026" + data.accountId.slice(-6) : data.accountId || "unknown");
      statusEl.innerHTML = "\u{1F916} Codex detected \xB7 Account: <strong>" + esc(displayId) + "</strong><br>" + tokenStatus;
      if (data.snapshot) {
        statusEl.innerHTML += "<br>Latest: <strong>" + data.snapshot.fiveHourPct.toFixed(1) + '%</strong> used (5h) \xB7 <span style="color:var(--text-muted)">' + formatTimeAgo(data.snapshot.capturedAt) + "</span>";
      }
    }).catch(function() {
      statusEl.style.display = "none";
    });
  }
  function handleCodexSnap() {
    showToast("\u{1F916} Capturing Codex snapshot...", "info");
    fetch("/api/codex/snap", { method: "POST" }).then(function(r) {
      return r.json();
    }).then(function(data) {
      if (data.error) {
        showToast("\u274C " + data.error, "error");
        return;
      }
      showToast("\u{1F916} Codex snapshot captured! Plan: " + (data.plan || "unknown"), "success");
      loadCodexSettingsStatus();
      document.dispatchEvent(new CustomEvent("niyantra:overview-refresh"));
    }).catch(function() {
      showToast("\u274C Codex snap failed", "error");
    });
  }
  function renderSessionsTimeline(container) {
    fetch("/api/sessions?limit=10").then(function(r) {
      return r.json();
    }).then(function(data) {
      if (!data.sessions || data.sessions.length === 0) return;
      var html = '<div class="overview-card sessions-card">';
      html += '<div class="card-header"><h3>\u23F1\uFE0F Usage Sessions</h3>';
      html += '<span class="card-count">' + data.count + " sessions</span>";
      html += "</div>";
      html += '<div class="card-body">';
      html += '<div class="session-timeline">';
      for (var i = 0; i < data.sessions.length; i++) {
        var sess = data.sessions[i];
        var isActive = !sess.endedAt;
        var duration = isActive ? formatDurationSec(Math.floor((Date.now() - new Date(sess.startedAt).getTime()) / 1e3)) : formatDurationSec(sess.durationSec);
        var providerIcon = sess.provider === "codex" ? "\u{1F916}" : sess.provider === "claude" ? "\u{1F52E}" : "\u26A1";
        html += '<div class="session-item' + (isActive ? " active" : "") + '">';
        html += '<div class="session-dot' + (isActive ? " pulse" : "") + '"></div>';
        html += '<div class="session-content">';
        html += '<div class="session-top">';
        html += '<span class="session-provider">' + providerIcon + " " + esc(sess.provider) + "</span>";
        html += '<span class="session-duration">' + duration + "</span>";
        html += "</div>";
        html += '<div class="session-bottom">';
        html += '<span class="session-time">' + formatTimeAgo(sess.startedAt) + "</span>";
        html += '<span class="session-snaps">' + sess.snapCount + " snaps</span>";
        if (isActive) html += '<span class="session-active-badge">LIVE</span>';
        html += "</div>";
        html += "</div></div>";
      }
      html += "</div></div></div>";
      var codexCard = container.querySelector(".codex-card");
      var existing = container.querySelector(".sessions-card");
      if (existing) {
        existing.outerHTML = html;
      } else if (codexCard) {
        codexCard.insertAdjacentHTML("afterend", html);
      } else {
        container.insertAdjacentHTML("afterbegin", html);
      }
    }).catch(function() {
    });
  }

  // internal/web/src/overview/tokenAnalytics.ts
  function loadTokenAnalytics() {
    var container = document.getElementById("token-analytics-container");
    if (!container) return;
    var rangeSelector = document.getElementById("token-range-selector");
    var days = 30;
    if (rangeSelector) {
      days = parseInt(rangeSelector.value) || 30;
    }
    fetch("/api/token-usage?days=" + days).then(function(res) {
      return res.json();
    }).then(function(data) {
      renderTokenAnalytics(container, data, days);
    }).catch(function(err) {
      console.error("Token analytics fetch failed:", err);
      container.innerHTML = '<div class="token-analytics-empty">Failed to load token analytics</div>';
    });
  }
  function renderTokenAnalytics(container, data, days) {
    if (!data || !data.totals || data.totals.totalTokens === 0) {
      container.innerHTML = '<div class="overview-card full-width token-analytics-card"><h3>\u{1F525} Token Usage Analytics</h3><div class="token-analytics-empty"><p>No token usage data available yet.</p><p style="font-size:12px;color:var(--text-secondary)">Use Claude Code to generate token usage data. Session files are parsed from <code>~/.claude/projects/</code>.</p></div></div>';
      return;
    }
    var totals = data.totals;
    var kpis = data.kpis || {};
    var models = data.byModel || [];
    var dailyData = data.byDay || [];
    var rangeOptions = [
      { value: "7", label: "7d" },
      { value: "30", label: "30d" },
      { value: "90", label: "90d" },
      { value: "365", label: "1y" }
    ];
    var rangeHTML = '<div class="token-range-bar">';
    for (var i = 0; i < rangeOptions.length; i++) {
      var opt = rangeOptions[i];
      var activeClass = String(days) === opt.value ? " token-range-active" : "";
      rangeHTML += '<button class="token-range-btn' + activeClass + '" data-days="' + opt.value + '">' + opt.label + "</button>";
    }
    rangeHTML += "</div>";
    var kpiHTML = '<div class="token-kpi-row">';
    kpiHTML += buildKpiCard("Total Tokens", formatTokens2(totals.totalTokens), "\u{1F4CA}");
    kpiHTML += buildKpiCard("Est. Cost", "$" + (totals.estimatedCostUSD || 0).toFixed(2), "\u{1F4B0}");
    kpiHTML += buildKpiCard("Active Days", String(kpis.daysActive || 0), "\u{1F4C5}");
    kpiHTML += buildKpiCard("Avg/Day", formatTokens2(kpis.avgTokensPerDay || 0), "\u{1F4C8}");
    kpiHTML += buildKpiCard("Cache Rate", Math.round((kpis.cacheHitRate || 0) * 100) + "%", "\u26A1");
    kpiHTML += "</div>";
    var chipsHTML = '<div class="token-breakdown-chips">';
    chipsHTML += '<span class="token-chip token-chip-input">Input: ' + formatTokens2(totals.inputTokens) + "</span>";
    chipsHTML += '<span class="token-chip token-chip-output">Output: ' + formatTokens2(totals.outputTokens) + "</span>";
    chipsHTML += '<span class="token-chip token-chip-cache">Cache: ' + formatTokens2(totals.cacheTokens) + "</span>";
    if (totals.sessions > 0) {
      chipsHTML += '<span class="token-chip token-chip-sessions">Sessions: ' + totals.sessions + "</span>";
    }
    chipsHTML += "</div>";
    var modelHTML = "";
    if (models.length > 0) {
      modelHTML = '<div class="token-section">';
      modelHTML += "<h4>Model Distribution</h4>";
      modelHTML += '<div class="token-model-bars">';
      var colors = ["#6366f1", "#8b5cf6", "#ec4899", "#f59e0b", "#10b981", "#3b82f6", "#ef4444"];
      var topModels = models.slice(0, 7);
      for (var mi = 0; mi < topModels.length; mi++) {
        var model = topModels[mi];
        var color = colors[mi % colors.length];
        var pct = model.percentage || 0;
        var costLabel = model.costUSD > 0 ? " \xB7 $" + model.costUSD.toFixed(2) : "";
        modelHTML += '<div class="token-model-row"><div class="token-model-header"><span class="token-model-name" style="color:' + color + '">' + escapeHtml(model.model) + '</span><span class="token-model-stats">' + formatTokens2(model.totalTokens) + " (" + pct.toFixed(1) + "%)" + costLabel + '</span></div><div class="token-model-bar-track"><div class="token-model-bar-fill" style="width:' + pct + "%;background:" + color + '"></div></div></div>';
      }
      modelHTML += "</div></div>";
    }
    var chartHTML = "";
    if (dailyData.length > 0) {
      chartHTML = '<div class="token-section">';
      chartHTML += "<h4>Daily Token Burn</h4>";
      chartHTML += '<div class="token-daily-chart">';
      var maxTokens = 0;
      for (var di = 0; di < dailyData.length; di++) {
        if (dailyData[di].totalTokens > maxTokens) maxTokens = dailyData[di].totalTokens;
      }
      var displayDays = dailyData;
      if (displayDays.length > 60) {
        displayDays = displayDays.slice(displayDays.length - 60);
      }
      for (var dj = 0; dj < displayDays.length; dj++) {
        var day = displayDays[dj];
        var barHeight = maxTokens > 0 ? Math.max(2, day.totalTokens / maxTokens * 100) : 2;
        var inputPct = day.totalTokens > 0 ? day.inputTokens / day.totalTokens * barHeight : 0;
        var outputPct = barHeight - inputPct;
        var dayLabel = day.date.substring(5);
        chartHTML += '<div class="token-bar-col" title="' + day.date + ": " + formatTokens2(day.totalTokens) + " tokens, $" + (day.costUSD || 0).toFixed(2) + '"><div class="token-bar-stack" style="height:' + barHeight + '%"><div class="token-bar-output" style="height:' + outputPct + '%"></div><div class="token-bar-input" style="height:' + inputPct + '%"></div></div><span class="token-bar-label">' + dayLabel + "</span></div>";
      }
      chartHTML += "</div>";
      chartHTML += '<div class="token-chart-legend"><span class="token-legend-item"><span class="token-legend-dot" style="background:var(--token-input-color)"></span>Input</span><span class="token-legend-item"><span class="token-legend-dot" style="background:var(--token-output-color)"></span>Output</span></div>';
      chartHTML += "</div>";
    }
    var peakHTML = "";
    if (kpis.peakDay) {
      peakHTML = '<div class="token-peak-badge">\u{1F525} Peak: ' + kpis.peakDay + " \u2014 " + formatTokens2(kpis.peakDayTokens) + " tokens</div>";
    }
    container.innerHTML = '<div class="overview-card full-width token-analytics-card"><div class="token-analytics-header"><h3>\u{1F525} Token Usage Analytics</h3>' + rangeHTML + "</div>" + kpiHTML + chipsHTML + peakHTML + modelHTML + chartHTML + "</div>";
    var rangeBtns = container.querySelectorAll(".token-range-btn");
    for (var bi = 0; bi < rangeBtns.length; bi++) {
      rangeBtns[bi].addEventListener("click", function() {
        var newDays = this.getAttribute("data-days") || "30";
        var allBtns = container.querySelectorAll(".token-range-btn");
        for (var k = 0; k < allBtns.length; k++) allBtns[k].classList.remove("token-range-active");
        this.classList.add("token-range-active");
        fetch("/api/token-usage?days=" + newDays).then(function(res) {
          return res.json();
        }).then(function(d) {
          renderTokenAnalytics(container, d, parseInt(newDays));
        });
      });
    }
  }
  function buildKpiCard(label, value, icon) {
    return '<div class="token-kpi-card"><div class="token-kpi-icon">' + icon + '</div><div class="token-kpi-value">' + value + '</div><div class="token-kpi-label">' + label + "</div></div>";
  }
  function formatTokens2(n) {
    if (n >= 1e9) return (n / 1e9).toFixed(1) + "B";
    if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
    if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
    return String(n);
  }
  function escapeHtml(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }

  // internal/web/src/overview/gitCosts.ts
  function loadGitCosts() {
    var container = document.getElementById("git-costs-container");
    if (!container) return;
    fetch("/api/git-costs?days=30").then(function(res) {
      return res.json();
    }).then(function(data) {
      renderGitCosts(container, data);
    }).catch(function(err) {
      console.error("Git costs fetch failed:", err);
      container.innerHTML = "";
    });
  }
  function renderGitCosts(container, data) {
    if (!data || !data.commits || data.commits.length === 0) {
      container.innerHTML = '<div class="overview-card full-width git-costs-card"><h3>\u26A1 Git \xD7 AI Cost Correlation</h3><div class="git-costs-empty"><p>No git commit data available.</p><p style="font-size:12px;color:var(--text-secondary)">Ensure you are running Niyantra from within a git repository, or pass <code>?repo=/path</code> to the API.</p></div></div>';
      return;
    }
    var totals = data.totals || {};
    var commits = data.commits || [];
    var branches = data.branches || [];
    var hasAICosts = totals.totalTokens > 0;
    var kpiHTML = '<div class="git-kpi-row">';
    kpiHTML += buildKpi("Commits", String(totals.commitCount || 0), "\u{1F4DD}");
    kpiHTML += buildKpi("AI Cost", "$" + (totals.costUSD || 0).toFixed(2), "\u{1F4B0}");
    kpiHTML += buildKpi("Avg/Commit", "$" + (totals.avgPerCommit || 0).toFixed(2), "\u{1F4CA}");
    kpiHTML += buildKpi("Top Branch", truncate(totals.topBranch || "\u2014", 18), "\u{1F33F}");
    kpiHTML += "</div>";
    if (!hasAICosts) {
      kpiHTML += '<div class="git-no-ai-banner">No Claude Code session data found in the commit time windows. AI costs will appear when commits overlap with Claude Code usage.</div>';
    }
    var chartHTML = "";
    if (commits.length > 0 && hasAICosts) {
      chartHTML = '<div class="git-section">';
      chartHTML += "<h4>Cost per Commit</h4>";
      chartHTML += '<div class="git-commit-chart">';
      var maxCost = 0;
      for (var ci = 0; ci < commits.length; ci++) {
        if (commits[ci].costUSD > maxCost) maxCost = commits[ci].costUSD;
      }
      var displayCommits = commits;
      if (displayCommits.length > 40) {
        displayCommits = displayCommits.slice(0, 40);
      }
      for (var di = 0; di < displayCommits.length; di++) {
        var c = displayCommits[di];
        var barH = maxCost > 0 ? Math.max(3, c.costUSD / maxCost * 100) : 3;
        var barColor = c.costUSD > 0 ? "var(--accent)" : "var(--border)";
        chartHTML += '<div class="git-bar-col" title="' + escapeAttr(c.shortHash) + ": " + escapeAttr(c.message) + "\n$" + c.costUSD.toFixed(2) + " \xB7 " + formatTokens3(c.totalTokens) + ' tokens"><div class="git-bar" style="height:' + barH + "%;background:" + barColor + '"></div><span class="git-bar-hash">' + c.shortHash + "</span></div>";
      }
      chartHTML += "</div></div>";
    }
    var branchHTML = "";
    if (branches.length > 0 && hasAICosts) {
      branchHTML = '<div class="git-section">';
      branchHTML += "<h4>Branch Costs</h4>";
      branchHTML += '<div class="git-branch-table">';
      branchHTML += '<div class="git-branch-header"><span>Branch</span><span>Commits</span><span>Tokens</span><span>Cost</span><span>Avg</span></div>';
      var displayBranches = branches.slice(0, 10);
      for (var bi = 0; bi < displayBranches.length; bi++) {
        var b = displayBranches[bi];
        if (b.costUSD === 0 && b.totalTokens === 0) continue;
        branchHTML += '<div class="git-branch-row"><span class="git-branch-name">' + escapeHtml2(truncate(b.name, 30)) + '</span><span class="git-branch-val">' + b.commits + '</span><span class="git-branch-val">' + formatTokens3(b.totalTokens) + '</span><span class="git-branch-cost">$' + b.costUSD.toFixed(2) + '</span><span class="git-branch-val">$' + b.avgPerCommit.toFixed(2) + "</span></div>";
      }
      branchHTML += "</div></div>";
    }
    var commitsHTML = '<div class="git-section">';
    commitsHTML += "<h4>Recent Commits</h4>";
    commitsHTML += '<div class="git-commits-list">';
    var showCommits = commits.slice(0, 15);
    for (var ri = 0; ri < showCommits.length; ri++) {
      var rc = showCommits[ri];
      var costBadge = rc.costUSD > 0 ? '<span class="git-cost-badge">$' + rc.costUSD.toFixed(2) + "</span>" : '<span class="git-cost-badge git-cost-zero">\u2014</span>';
      var tokenBadge = rc.totalTokens > 0 ? '<span class="git-token-badge">' + formatTokens3(rc.totalTokens) + "</span>" : "";
      commitsHTML += '<div class="git-commit-item"><span class="git-commit-hash">' + rc.shortHash + '</span><span class="git-commit-msg">' + escapeHtml2(rc.message) + '</span><div class="git-commit-meta">' + tokenBadge + costBadge + "</div></div>";
    }
    commitsHTML += "</div></div>";
    container.innerHTML = '<div class="overview-card full-width git-costs-card"><div class="git-costs-header"><h3>\u26A1 Git \xD7 AI Cost Correlation</h3><span class="git-repo-path" title="' + escapeAttr(data.repoPath || "") + '">' + escapeHtml2(shortenPath(data.repoPath || "")) + "</span></div>" + kpiHTML + chartHTML + branchHTML + commitsHTML + "</div>";
  }
  function buildKpi(label, value, icon) {
    return '<div class="git-kpi-card"><div class="git-kpi-icon">' + icon + '</div><div class="git-kpi-value">' + value + '</div><div class="git-kpi-label">' + label + "</div></div>";
  }
  function formatTokens3(n) {
    if (n >= 1e6) return (n / 1e6).toFixed(1) + "M";
    if (n >= 1e3) return (n / 1e3).toFixed(1) + "K";
    return String(n);
  }
  function truncate(s, max) {
    return s.length > max ? s.substring(0, max - 1) + "\u2026" : s;
  }
  function shortenPath(p) {
    var parts = p.replace(/\\/g, "/").split("/");
    return parts.length > 2 ? "\u2026/" + parts.slice(-2).join("/") : p;
  }
  function escapeHtml2(s) {
    return s.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/"/g, "&quot;");
  }
  function escapeAttr(s) {
    return s.replace(/&/g, "&amp;").replace(/"/g, "&quot;").replace(/'/g, "&#39;");
  }

  // internal/web/src/overview/overview.ts
  function loadOverview() {
    Promise.all([fetchOverview(), fetchSubscriptions("", ""), fetchUsage()]).then(function(results) {
      var data = results[0];
      var subsData = results[1];
      var usageData = results[2];
      renderOverviewEnhanced(data, subsData.subscriptions || subsData || [], usageData);
    }).catch(function(err) {
      console.error("Failed to load overview:", err);
    });
  }
  function renderOverviewEnhanced(data, subs, usageData) {
    var el = document.getElementById("overview-content");
    if (!el) return;
    var stats = data.stats || { totalMonthlySpend: 0, totalAnnualSpend: 0, byCategory: {}, byStatus: {} };
    var renewals = data.renewals || [];
    var links = data.quickLinks || [];
    var quotas = data.quotaSummary;
    var serverInsights = data.insights || [];
    var advisorHTML = '<div id="advisor-card-container"></div>';
    var insightsHTML = renderServerInsights(serverInsights);
    var forecastHTML = "";
    if (usageData && usageData.budgetForecast) {
      var bf = usageData.budgetForecast;
      var forecastCls = bf.onTrack ? "forecast-ok" : "forecast-over";
      var forecastIcon = bf.onTrack ? "\u2705" : "\u26A0\uFE0F";
      var pct = Math.round(bf.currentSpend / bf.monthlyBudget * 100);
      var statusMsg = bf.onTrack ? "On track \u2014 $" + bf.currentSpend.toFixed(2) + " of $" + bf.monthlyBudget.toFixed(2) + " budget (" + pct + "%)" : "Over budget \u2014 $" + bf.currentSpend.toFixed(2) + " exceeds $" + bf.monthlyBudget.toFixed(2) + " by $" + (bf.currentSpend - bf.monthlyBudget).toFixed(2) + " (" + pct + "%)";
      forecastHTML = '<div class="overview-card full-width"><h3>Budget Status</h3><div class="budget-forecast ' + forecastCls + '"><div class="forecast-header">' + forecastIcon + " " + statusMsg + '</div><div class="forecast-details"><span class="forecast-chip">Monthly subs: $' + bf.currentSpend.toFixed(2) + '</span><span class="forecast-chip">Budget: $' + bf.monthlyBudget.toFixed(2) + "</span></div></div></div>";
    } else if (!getBudget()) {
      forecastHTML = '<div class="overview-card full-width"><div class="budget-forecast forecast-ok"><div class="forecast-header">\u{1F4B0} No monthly budget set</div><div class="forecast-details"><button class="btn-add-sm" onclick="openBudgetModal()">Set Budget</button></div></div></div>';
    }
    var cats = Object.keys(stats.byCategory);
    var spendHTML = '<div class="overview-card"><h3>Monthly AI Spend</h3><div class="overview-big-number">$' + stats.totalMonthlySpend.toFixed(2) + "</div>";
    if (cats.length > 1) {
      cats.sort(function(a, b) {
        return (stats.byCategory[b].monthlySpend || 0) - (stats.byCategory[a].monthlySpend || 0);
      });
      for (var i = 0; i < cats.length; i++) {
        var c = stats.byCategory[cats[i]];
        spendHTML += '<div class="overview-category-row"><span class="overview-category-name">' + esc(cats[i]) + '<span class="overview-category-count">' + c.count + ' subs</span></span><span class="overview-category-spend">$' + c.monthlySpend.toFixed(2) + "/mo</span></div>";
      }
    } else if (cats.length === 1) {
      var onlyCat = stats.byCategory[cats[0]];
      spendHTML += '<div class="overview-big-label">' + onlyCat.count + " " + cats[0] + " subscription" + (onlyCat.count !== 1 ? "s" : "") + "</div>";
    }
    spendHTML += "</div>";
    var claudeHTML = renderClaudeCodeCard();
    var calendarHTML = "";
    if (renewals.length > 0) {
      calendarHTML = '<div id="renewal-calendar-container" class="overview-card full-width"></div>';
    }
    var linksHTML = "";
    if (links.length > 0) {
      var platformLinks = {};
      for (var l = 0; l < links.length; l++) {
        var lnk = links[l];
        if (!platformLinks[lnk.platform]) {
          platformLinks[lnk.platform] = lnk;
        }
      }
      var platformKeys = Object.keys(platformLinks);
      if (platformKeys.length > 1 || platformKeys.length === 1 && platformKeys[0] !== "Antigravity") {
        linksHTML = '<div class="overview-card full-width"><h3>Quick Links</h3><div class="quick-links-grid">';
        for (var pk = 0; pk < platformKeys.length; pk++) {
          var pl = platformLinks[platformKeys[pk]];
          linksHTML += '<a class="quick-link" href="' + esc(pl.url) + '" target="_blank" rel="noopener">\u{1F517} ' + esc(pl.platform) + "</a>";
        }
        linksHTML += "</div></div>";
      }
    }
    var exportHTML = '<div class="overview-card full-width"><h3>Export</h3><p style="font-size:13px;color:var(--text-secondary);margin-bottom:12px">Download your data for expense tracking, tax reports, or backup.</p><div style="display:flex;gap:8px"><a class="btn-add" href="/api/export/csv" download style="text-decoration:none;display:inline-flex;padding:6px 12px;font-size:12px">\u{1F4E5} CSV</a><a class="btn-add" href="/api/export/json" download style="text-decoration:none;display:inline-flex;padding:6px 12px;font-size:12px">\u{1F4E6} JSON</a></div></div>';
    var providerHTML = '<div class="overview-card full-width"><h3>Provider Health</h3>';
    providerHTML += '<div class="provider-health-grid">';
    if (latestQuotaData && latestQuotaData.accounts && latestQuotaData.accounts.length > 0) {
      var accts = latestQuotaData.accounts;
      var readyCount = 0;
      for (var ai = 0; ai < accts.length; ai++) {
        if (accts[ai].isReady) readyCount++;
      }
      var healthPct = Math.round(readyCount / accts.length * 100);
      var healthCls = healthPct >= 80 ? "health-good" : healthPct >= 50 ? "health-warn" : "health-bad";
      providerHTML += '<div class="provider-health-row"><span class="ph-name">\u26A1 Antigravity</span><span class="ph-count">' + accts.length + ' accounts</span><span class="ph-bar"><span class="ph-fill ' + healthCls + '" style="width:' + healthPct + '%"></span></span><span class="ph-stat ' + healthCls + '">' + readyCount + "/" + accts.length + " ready</span></div>";
    }
    if (latestQuotaData && latestQuotaData.codexSnapshot) {
      var cs = latestQuotaData.codexSnapshot;
      var cxStatus = cs.status === "healthy" ? "health-good" : "health-bad";
      var cxLabel = cs.email || "Codex account";
      providerHTML += '<div class="provider-health-row"><span class="ph-name">\u{1F916} Codex</span><span class="ph-count">' + esc(cxLabel) + '</span><span class="ph-bar"><span class="ph-fill ' + cxStatus + '" style="width:' + (100 - (cs.sevenDayPct || 0)) + '%"></span></span><span class="ph-stat ' + cxStatus + '">' + esc(cs.planType || "free") + "</span></div>";
    }
    if (latestQuotaData && latestQuotaData.claudeSnapshot) {
      var cls2 = latestQuotaData.claudeSnapshot;
      var clStatus = cls2.status === "healthy" ? "health-good" : "health-bad";
      providerHTML += '<div class="provider-health-row"><span class="ph-name">\u{1F52E} Claude Code</span><span class="ph-count">Bridge</span><span class="ph-bar"><span class="ph-fill ' + clStatus + '" style="width:' + (100 - (cls2.fiveHourPct || 0)) + '%"></span></span><span class="ph-stat ' + clStatus + '">' + (cls2.status || "\u2014") + "</span></div>";
    }
    providerHTML += "</div></div>";
    var costKPIHTML = '<div id="cost-kpi-container"></div>';
    var tokenAnalyticsHTML = '<div id="token-analytics-container" class="overview-card full-width"></div>';
    var gitCostsHTML = '<div id="git-costs-container" class="overview-card full-width"></div>';
    var heatmapHTML = '<div id="heatmap-container" class="overview-card full-width"></div>';
    el.innerHTML = advisorHTML + forecastHTML + costKPIHTML + tokenAnalyticsHTML + gitCostsHTML + heatmapHTML + providerHTML + insightsHTML + claudeHTML + spendHTML + calendarHTML + linksHTML + exportHTML;
    if (serverConfig["claude_bridge"] === "true") {
      loadClaudeCardData();
    } else {
      var cardBody = document.getElementById("claude-card-body");
      if (cardBody) cardBody.innerHTML = "";
    }
    loadClaudeDeepUsage();
    loadAdvisorCard();
    loadCostKPI();
    loadHeatmap();
    loadTokenAnalytics();
    loadGitCosts();
    if (renewals.length > 0) {
      renderRenewalCalendar(renewals, subs);
    }
    renderSessionsTimeline(el);
  }

  // internal/web/src/advanced/snap.ts
  var snapDefault = localStorage.getItem("niyantra_snap_default") || "antigravity";
  function initSnapDropdown() {
    var caret = document.getElementById("snap-caret");
    var dropdown = document.getElementById("snap-dropdown");
    if (!caret || !dropdown) return;
    caret.addEventListener("click", function(e) {
      e.stopPropagation();
      dropdown.classList.toggle("open");
    });
    document.addEventListener("click", function() {
      dropdown.classList.remove("open");
    });
    dropdown.querySelectorAll(".snap-option").forEach(function(opt) {
      opt.addEventListener("click", function(e) {
        e.stopPropagation();
        var source = opt.dataset.source;
        dropdown.classList.remove("open");
        if (source === "all") {
          snapSource("all");
        } else {
          snapDefault = source;
          localStorage.setItem("niyantra_snap_default", source);
          updateSnapDropdownIndicators();
          snapSource(source);
        }
      });
    });
    updateSnapDropdownIndicators();
  }
  function updateSnapDropdownIndicators() {
    var dropdown = document.getElementById("snap-dropdown");
    if (!dropdown) return;
    dropdown.querySelectorAll(".snap-option").forEach(function(opt) {
      if (opt.dataset.source === "all") return;
      var isActive = opt.dataset.source === snapDefault;
      opt.textContent = (isActive ? "\u25C9 " : "\u25CB ") + opt.textContent.replace(/^[◉○] /, "");
      opt.classList.toggle("active", isActive);
    });
  }
  function handleSnap() {
    snapSource(snapDefault);
  }
  function snapSource(source) {
    var btn = document.getElementById("snap-btn");
    if (!btn || btn.disabled || snapInProgress) return;
    setSnapInProgress(true);
    btn.disabled = true;
    btn.classList.add("snapping");
    var orig = btn.innerHTML;
    btn.innerHTML = '<svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><circle cx="12" cy="12" r="3"/></svg> Capturing...';
    var promises = [];
    if (source === "antigravity" || source === "all") {
      promises.push(
        triggerSnap().then(function(data) {
          return { source: "Antigravity", data, label: data.email || "Antigravity" };
        }).catch(function(err) {
          return { source: "Antigravity", error: err.message };
        })
      );
    }
    if (source === "codex" || source === "all") {
      promises.push(
        fetch("/api/codex/snap", { method: "POST" }).then(function(r) {
          return r.json();
        }).then(function(d) {
          var label = d.plan ? "Codex \xB7 " + d.plan : "Codex";
          return { source: "Codex", data: d, label };
        }).catch(function() {
          return { source: "Codex", error: "capture failed" };
        })
      );
    }
    if (source === "cursor" || source === "all") {
      promises.push(
        fetch("/api/cursor/snap", { method: "POST" }).then(function(r) {
          return r.json();
        }).then(function(d) {
          if (d.error) return { source: "Cursor", error: d.error };
          var label = "Cursor \xB7 " + (d.premiumUsed || 0) + "/" + (d.premiumLimit || "?");
          return { source: "Cursor", data: d, label };
        }).catch(function() {
          return { source: "Cursor", error: "capture failed" };
        })
      );
    }
    if (source === "gemini" || source === "all") {
      promises.push(
        fetch("/api/gemini/snap", { method: "POST" }).then(function(r) {
          return r.json();
        }).then(function(d) {
          if (d.error) return { source: "Gemini", error: d.error };
          var label = "Gemini \xB7 " + (d.modelCount || 0) + " models";
          return { source: "Gemini", data: d, label };
        }).catch(function() {
          return { source: "Gemini", error: "capture failed" };
        })
      );
    }
    if (promises.length === 0) {
      btn.innerHTML = orig;
      btn.disabled = false;
      setSnapInProgress(false);
      showToast("No snap source selected", "warning");
      return;
    }
    Promise.all(promises).then(function(results) {
      var msgs = [];
      var antigravityData = null;
      for (var i = 0; i < results.length; i++) {
        var r = results[i];
        if (r.error) {
          msgs.push("\u274C " + r.source + ": " + r.error);
        } else {
          msgs.push("\u2705 " + r.label);
          if (r.source === "Antigravity") antigravityData = r.data;
        }
      }
      showToast(msgs.join(" \xB7 "), msgs.some(function(m) {
        return m.startsWith("\u274C");
      }) ? "warning" : "success");
      if (antigravityData) {
        renderAccounts(antigravityData);
        updateTimestamp();
      }
    }).finally(function() {
      btn.innerHTML = orig;
      btn.disabled = false;
      btn.classList.remove("snapping");
      setSnapInProgress(false);
    });
  }

  // internal/web/src/charts/history.ts
  var Chart = window.Chart;
  var historyChart = null;
  function updateChartTheme(theme) {
    if (!historyChart) return;
    var isDark = theme !== "light";
    var gridColor = isDark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";
    var textColor = isDark ? "#94a3b8" : "#64748b";
    if (historyChart.options.scales && historyChart.options.scales.y) {
      historyChart.options.scales.y.grid.color = gridColor;
      historyChart.options.scales.y.ticks.color = textColor;
    }
    if (historyChart.options.scales && historyChart.options.scales.x) {
      historyChart.options.scales.x.grid.color = gridColor;
      historyChart.options.scales.x.ticks.color = textColor;
    }
    historyChart.update("none");
  }
  function loadHistoryChart() {
    if (typeof Chart === "undefined") return;
    var accountId = parseInt(document.getElementById("chart-account").value) || 0;
    var limit = parseInt(document.getElementById("chart-range").value) || 20;
    var url = "/api/history?limit=" + limit;
    if (accountId > 0) url += "&account=" + accountId;
    fetch(url).then(function(res) {
      return res.json();
    }).then(function(data) {
      renderHistoryChart(data.snapshots || []);
    }).catch(function(err) {
      console.error("Failed to load history:", err);
    });
  }
  function renderHistoryChart(snapshots) {
    var container = document.querySelector(".chart-container");
    if (!container || typeof Chart === "undefined") return;
    if (snapshots.length === 0) {
      container.innerHTML = '<div class="chart-empty">No snapshot history yet. Click Snap Now to start tracking.</div>';
      return;
    }
    container.innerHTML = '<canvas id="history-chart"></canvas>';
    snapshots = snapshots.slice().reverse();
    var labels = snapshots.map(function(s) {
      var d = new Date(s.capturedAt);
      return d.toLocaleDateString(void 0, { month: "short", day: "numeric" }) + " " + d.toLocaleTimeString(void 0, { hour: "2-digit", minute: "2-digit" });
    });
    var groupData = {};
    var groupNames = { claude_gpt: "Claude + GPT", gemini_pro: "Gemini Pro", gemini_flash: "Gemini Flash" };
    var groupColors = { claude_gpt: "#D97757", gemini_pro: "#10B981", gemini_flash: "#3B82F6" };
    for (var i = 0; i < snapshots.length; i++) {
      var groups = snapshots[i].groups || [];
      for (var j = 0; j < groups.length; j++) {
        var g = groups[j];
        if (!groupData[g.groupKey]) groupData[g.groupKey] = [];
      }
    }
    var aiCreditsData = [];
    var hasAICredits = false;
    for (var i = 0; i < snapshots.length; i++) {
      var snap = snapshots[i];
      var groups = snap.groups || [];
      var seen = {};
      for (var j = 0; j < groups.length; j++) {
        var g = groups[j];
        if (!groupData[g.groupKey]) groupData[g.groupKey] = [];
        groupData[g.groupKey].push(Math.round(g.remainingPercent || 0));
        seen[g.groupKey] = true;
      }
      var keys = Object.keys(groupData);
      for (var k = 0; k < keys.length; k++) {
        if (!seen[keys[k]]) groupData[keys[k]].push(null);
      }
      if (snap.aiCredits && snap.aiCredits.length > 0) {
        aiCreditsData.push(snap.aiCredits[0].creditAmount);
        hasAICredits = true;
      } else {
        aiCreditsData.push(null);
      }
    }
    var datasets = [];
    var keys = Object.keys(groupData);
    for (var k = 0; k < keys.length; k++) {
      var key = keys[k];
      if (!key || !groupNames[key]) continue;
      datasets.push({
        label: groupNames[key],
        data: groupData[key],
        borderColor: groupColors[key] || "#94a3b8",
        backgroundColor: (groupColors[key] || "#94a3b8") + "20",
        yAxisID: "y",
        fill: true,
        tension: 0.3,
        pointRadius: 3,
        pointHoverRadius: 6,
        borderWidth: 2
      });
    }
    if (hasAICredits) {
      datasets.push({
        label: "AI Credits",
        data: aiCreditsData,
        borderColor: "#fbbf24",
        // Amber
        backgroundColor: "transparent",
        yAxisID: "yCredits",
        borderDash: [5, 5],
        tension: 0.3,
        pointRadius: 4,
        pointBackgroundColor: "#fbbf24",
        pointHoverRadius: 6,
        borderWidth: 3
      });
    }
    var isDark = document.documentElement.getAttribute("data-theme") !== "light";
    var gridColor = isDark ? "rgba(255,255,255,0.06)" : "rgba(0,0,0,0.06)";
    var textColor = isDark ? "#94a3b8" : "#64748b";
    if (historyChart) historyChart.destroy();
    var ctx = document.getElementById("history-chart");
    if (!ctx) return;
    historyChart = new Chart(ctx, {
      type: "line",
      data: { labels, datasets },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        interaction: { mode: "index", intersect: false },
        plugins: {
          legend: {
            position: "bottom",
            labels: { color: textColor, font: { family: "'Inter', sans-serif", size: 11 }, boxWidth: 12, padding: 16 }
          },
          tooltip: {
            backgroundColor: isDark ? "#1e293b" : "#fff",
            titleColor: isDark ? "#f1f5f9" : "#0f172a",
            bodyColor: isDark ? "#94a3b8" : "#475569",
            borderColor: isDark ? "#334155" : "#e2e8f0",
            borderWidth: 1,
            padding: 10,
            titleFont: { family: "'Inter', sans-serif", weight: "600" },
            bodyFont: { family: "'Inter', sans-serif" },
            callbacks: {
              label: function(ctx2) {
                if (ctx2.dataset.yAxisID === "yCredits") return ctx2.dataset.label + ": " + ctx2.parsed.y.toLocaleString();
                return ctx2.dataset.label + ": " + ctx2.parsed.y + "%";
              }
            }
          }
        },
        scales: {
          y: {
            type: "linear",
            display: true,
            position: "left",
            min: 0,
            max: 100,
            grid: { color: gridColor },
            ticks: { color: textColor, font: { family: "'Inter', sans-serif", size: 11 }, callback: function(v) {
              return v + "%";
            } },
            border: { display: false }
          },
          yCredits: {
            type: "linear",
            display: hasAICredits,
            position: "right",
            grid: { display: false },
            ticks: { color: isDark ? "#fbbf24" : "#d97706", font: { family: "'Inter', sans-serif", size: 11 } },
            border: { display: false }
          },
          x: {
            grid: { display: false },
            ticks: { color: textColor, font: { family: "'Inter', sans-serif", size: 10 }, maxRotation: 45, maxTicksLimit: 12 },
            border: { display: false }
          }
        }
      }
    });
  }
  function populateChartAccountSelect(data) {
    var sel = document.getElementById("chart-account");
    if (!sel || !data.accounts) return;
    while (sel.options.length > 1) sel.remove(1);
    for (var i = 0; i < data.accounts.length; i++) {
      var opt = document.createElement("option");
      opt.value = data.accounts[i].accountId;
      opt.textContent = data.accounts[i].email;
      sel.appendChild(opt);
    }
  }

  // internal/web/src/advanced/alerts.ts
  function loadSystemAlerts() {
    fetch("/api/alerts").then(function(r) {
      return r.json();
    }).then(function(data) {
      var container = document.getElementById("alert-banner-container");
      if (!container) return;
      var alerts = data.alerts || [];
      if (alerts.length === 0) {
        container.innerHTML = "";
        return;
      }
      var html = "";
      var shown = Math.min(alerts.length, 3);
      for (var i = 0; i < shown; i++) {
        var a = alerts[i];
        var icon = a.severity === "critical" ? "\u{1F6A8}" : a.severity === "warning" ? "\u26A0\uFE0F" : "\u2139\uFE0F";
        html += '<div class="alert-banner ' + esc(a.severity) + '"><span class="alert-banner-icon">' + icon + '</span><div class="alert-banner-content"><div class="alert-banner-title">' + esc(a.category) + '</div><div class="alert-banner-msg">' + esc(a.message) + '</div></div><button class="alert-banner-dismiss" onclick="dismissAlert(' + a.id + ')" title="Dismiss">&times;</button></div>';
      }
      if (alerts.length > 3) {
        html += `<div class="alert-more-link" onclick="switchToTab('overview')">+ ` + (alerts.length - 3) + " more alert(s)</div>";
      }
      container.innerHTML = html;
    }).catch(function() {
    });
  }
  function dismissAlert(id) {
    fetch("/api/alerts/dismiss", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ id })
    }).then(function() {
      loadSystemAlerts();
      showToast("Alert dismissed", "success");
    }).catch(function() {
      showToast("Failed to dismiss alert", "error");
    });
  }

  // internal/web/src/settings/activity.ts
  function loadActivityLog() {
    var filter = document.getElementById("activity-filter").value;
    var url = "/api/activity?limit=50";
    if (filter) url += "&type=" + filter;
    fetch(url).then(function(r) {
      return r.json();
    }).then(function(data) {
      var container = document.getElementById("activity-log");
      if (!data.entries || data.entries.length === 0) {
        container.innerHTML = '<div class="activity-empty">No activity' + (filter ? ' for "' + filter + '"' : "") + " yet</div>";
        return;
      }
      var html = "";
      data.entries.forEach(function(entry) {
        var time = entry.timestamp ? entry.timestamp.replace("T", " ").substring(5, 16) : "";
        var detail = formatActivityDetail(entry);
        html += '<div class="activity-entry"><span class="activity-time">' + time + '</span><span class="activity-type ' + esc(entry.eventType) + '">' + esc(entry.eventType.replace(/_/g, " ")) + '</span><span class="activity-detail">' + detail + "</span></div>";
      });
      container.innerHTML = html;
    }).catch(function() {
      document.getElementById("activity-log").innerHTML = '<div class="activity-empty">Failed to load activity log</div>';
    });
  }
  function formatActivityDetail(entry) {
    try {
      var d = JSON.parse(entry.details || "{}");
      switch (entry.eventType) {
        case "snap":
          return esc(entry.accountEmail || "") + (d.method ? " \xB7 " + d.method : "") + (d.source ? " via " + d.source : "");
        case "snap_failed":
          return esc(d.error || "Unknown error");
        case "config_change":
          return esc(d.key || "") + ": " + esc(d.from || '""') + " \u2192 " + esc(d.to || '""');
        case "server_start":
          return "Port " + (d.port || "?") + " \xB7 " + esc(d.mode || "manual") + " mode";
        case "sub_created":
        case "sub_deleted":
          return esc(d.platform || "");
        case "auto_link":
          return esc(entry.accountEmail || "") + " \u2192 " + esc(d.platform || "");
        case "codex_snap":
          var acctId = entry.accountEmail || "";
          if (acctId.length > 20) acctId = acctId.substring(0, 6) + ".." + acctId.slice(-6);
          return esc(acctId) + (d.plan ? " (" + esc(d.plan) + ")" : "");
        case "model_reset":
          return esc(entry.accountEmail || "");
        case "quota_alert":
          return "\u{1F514} " + esc(d.model || "") + " \u2014 " + (d.remainingPct != null ? d.remainingPct.toFixed(1) + "% remaining" : "");
        default:
          return entry.accountEmail ? esc(entry.accountEmail) : "";
      }
    } catch (e) {
      return "";
    }
  }

  // internal/web/src/settings/mode.ts
  var modeRefreshTimer = null;
  function loadMode() {
    fetch("/api/mode").then(function(r) {
      return r.json();
    }).then(function(data) {
      var badge = document.getElementById("mode-badge");
      var label = document.getElementById("mode-label");
      if (data.mode === "auto") {
        badge.className = "mode-badge mode-auto";
        label.textContent = "Auto";
      } else {
        badge.className = "mode-badge mode-manual";
        label.textContent = "Manual";
      }
      var statusEl = document.getElementById("polling-status");
      if (statusEl) {
        if (data.isPolling) {
          var lastMsg = "";
          if (data.lastPoll) {
            lastMsg = "Last: " + formatTimeAgo(data.lastPoll);
            if (data.lastPollOK === false) lastMsg += " (failed)";
          } else {
            lastMsg = "Starting...";
          }
          statusEl.innerHTML = '<span class="polling-dot"></span> Polling every ' + formatPollInterval(data.pollInterval) + " \xB7 " + lastMsg;
          statusEl.style.display = "";
        } else {
          statusEl.style.display = "none";
        }
      }
      var aboutEl = document.getElementById("s-about-info");
      if (aboutEl) {
        var srcCount = (data.sources || []).filter(function(s) {
          return s.enabled;
        }).length;
        var schemaV = data.schemaVersion ? "Schema v" + data.schemaVersion : "Schema";
        var presetCount = presetsData.length || 0;
        aboutEl.textContent = schemaV + " \xB7 " + presetCount + " presets \xB7 Mode: " + (data.mode === "auto" ? "Auto" : "Manual") + (data.isPolling ? " (polling)" : "") + " \xB7 " + srcCount + " active source" + (srcCount !== 1 ? "s" : "");
      }
      if (modeRefreshTimer) {
        clearInterval(modeRefreshTimer);
        modeRefreshTimer = null;
      }
      if (data.isPolling) {
        modeRefreshTimer = setInterval(function() {
          loadMode();
          loadSystemAlerts();
          var activeTab = document.querySelector(".tab-btn.active");
          if (activeTab && activeTab.getAttribute("data-tab") === "settings") {
            loadActivityLog();
          }
        }, 3e4);
      }
    }).catch(function() {
    });
  }

  // internal/web/src/settings/data.ts
  function loadDataSources() {
    fetch("/api/mode").then(function(r) {
      return r.json();
    }).then(function(data) {
      var container = document.getElementById("data-sources-list");
      if (!data.sources || data.sources.length === 0) {
        container.innerHTML = "";
        return;
      }
      var html = '<div style="font-size:12px;font-weight:600;color:var(--text-secondary);margin-bottom:4px;margin-top:4px">Data Sources</div>';
      data.sources.forEach(function(src) {
        var meta = src.captureCount + " captures";
        if (src.lastCapture) {
          meta += " \xB7 Last: " + formatTimeAgo(src.lastCapture);
        }
        html += '<div class="data-source-item"><div class="data-source-info"><span class="data-source-name">' + esc(src.name) + '</span><span class="data-source-meta">' + esc(src.sourceType) + " \xB7 " + meta + '</span></div><span class="data-source-status ' + (src.enabled ? "enabled" : "disabled") + '">' + (src.enabled ? "\u25CF Active" : "\u25CB Disabled") + "</span></div>";
      });
      container.innerHTML = html;
    }).catch(function() {
    });
  }

  // internal/web/src/settings/pricing.ts
  var pricingDataCache = null;
  function loadModelPricing() {
    fetch("/api/config/pricing").then(function(res) {
      return res.json();
    }).then(function(data) {
      pricingDataCache = data.pricing || [];
      renderPricingTable(pricingDataCache);
    }).catch(function(err) {
      console.error("Failed to load model pricing:", err);
    });
  }
  function renderPricingTable(pricing) {
    var tbody = document.getElementById("pricing-tbody");
    if (!tbody) return;
    var providerIcons = { anthropic: "\u{1F7E4}", openai: "\u{1F7E2}", google: "\u{1F535}" };
    var html = "";
    for (var i = 0; i < pricing.length; i++) {
      var p = pricing[i];
      var providerCls = p.provider || "custom";
      var providerLabel = p.provider ? p.provider.charAt(0).toUpperCase() + p.provider.slice(1) : "Custom";
      var icon = providerIcons[p.provider] || "\u26AA";
      html += '<tr data-pricing-idx="' + i + '"><td><span class="pricing-model-name">' + esc(p.displayName) + '</span></td><td><span class="pricing-provider ' + esc(providerCls) + '">' + icon + " " + esc(providerLabel) + '</span></td><td style="text-align:right"><input type="number" class="pricing-input" data-field="inputPer1M" step="0.01" min="0" value="' + p.inputPer1M + '"></td><td style="text-align:right"><input type="number" class="pricing-input" data-field="outputPer1M" step="0.01" min="0" value="' + p.outputPer1M + '"></td><td style="text-align:right"><input type="number" class="pricing-input" data-field="cachePer1M" step="0.001" min="0" value="' + p.cachePer1M + '"></td><td><button class="pricing-delete-btn" data-pricing-del="' + i + '" title="Remove this model">\u2715</button></td></tr>';
    }
    tbody.innerHTML = html;
    tbody.querySelectorAll(".pricing-input").forEach(function(input) {
      input.addEventListener("change", function() {
        var tr = input.closest("tr");
        var idx = parseInt(tr.dataset.pricingIdx);
        var field = input.dataset.field;
        var val = parseFloat(input.value) || 0;
        if (val < 0) val = 0;
        input.value = String(val);
        if (pricingDataCache && pricingDataCache[idx]) {
          pricingDataCache[idx][field] = val;
          savePricingFromTable();
        }
      });
    });
    tbody.querySelectorAll(".pricing-delete-btn").forEach(function(btn) {
      btn.addEventListener("click", function() {
        var idx = parseInt(btn.dataset.pricingDel);
        deletePricingRow(idx);
      });
    });
  }
  function savePricingFromTable() {
    if (!pricingDataCache || pricingDataCache.length === 0) return;
    fetch("/api/config/pricing", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pricing: pricingDataCache })
    }).then(function(res) {
      return res.json();
    }).then(function(data) {
      if (data.error) {
        showToast("\u274C " + data.error, "error");
        return;
      }
      showToast("\u{1F4B0} Pricing saved", "success");
    }).catch(function() {
      showToast("\u274C Failed to save pricing", "error");
    });
  }
  function addPricingRow() {
    if (!pricingDataCache) pricingDataCache = [];
    var newModel = {
      modelId: "custom-" + Date.now(),
      displayName: "New Model",
      provider: "custom",
      inputPer1M: 1,
      outputPer1M: 5,
      cachePer1M: 0.1
    };
    pricingDataCache.push(newModel);
    renderPricingTable(pricingDataCache);
    var tbody = document.getElementById("pricing-tbody");
    var lastRow = tbody.lastElementChild;
    if (lastRow) {
      var nameCell = lastRow.querySelector(".pricing-model-name");
      if (nameCell) {
        nameCell.contentEditable = "true";
        nameCell.focus();
        var range = document.createRange();
        range.selectNodeContents(nameCell);
        var sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(range);
        nameCell.addEventListener("blur", function() {
          nameCell.contentEditable = "false";
          var idx = parseInt(lastRow.dataset.pricingIdx);
          var newName = nameCell.textContent.trim();
          if (newName && pricingDataCache[idx]) {
            pricingDataCache[idx].displayName = newName;
            pricingDataCache[idx].modelId = newName.toLowerCase().replace(/[^a-z0-9]+/g, "-");
            savePricingFromTable();
          }
        }, { once: true });
        nameCell.addEventListener("keydown", function(e) {
          if (e.key === "Enter") {
            e.preventDefault();
            nameCell.blur();
          }
        });
      }
    }
    showToast("\u{1F4B0} New model added \u2014 edit the name and prices", "info");
  }
  function deletePricingRow(idx) {
    if (!pricingDataCache || idx < 0 || idx >= pricingDataCache.length) return;
    var name = pricingDataCache[idx].displayName;
    if (!confirm('Remove pricing for "' + name + '"?')) return;
    pricingDataCache.splice(idx, 1);
    renderPricingTable(pricingDataCache);
    savePricingFromTable();
    showToast("\u{1F5D1}\uFE0F Removed " + name, "success");
  }
  function resetPricingDefaults() {
    if (!confirm("Reset all model pricing to current market defaults? This will overwrite your custom prices.")) return;
    var defaults = [
      { modelId: "claude-opus-4.6", displayName: "Claude Opus 4.6", provider: "anthropic", inputPer1M: 5, outputPer1M: 25, cachePer1M: 0.5 },
      { modelId: "claude-sonnet-4.6", displayName: "Claude Sonnet 4.6", provider: "anthropic", inputPer1M: 3, outputPer1M: 15, cachePer1M: 0.3 },
      { modelId: "claude-haiku-4.5", displayName: "Claude Haiku 4.5", provider: "anthropic", inputPer1M: 1, outputPer1M: 5, cachePer1M: 0.1 },
      { modelId: "gpt-4o", displayName: "GPT-4o", provider: "openai", inputPer1M: 2.5, outputPer1M: 10, cachePer1M: 1.25 },
      { modelId: "gemini-3.1-pro", displayName: "Gemini 3.1 Pro", provider: "google", inputPer1M: 2, outputPer1M: 12, cachePer1M: 0.5 },
      { modelId: "gemini-2.5-flash", displayName: "Gemini 2.5 Flash", provider: "google", inputPer1M: 0.3, outputPer1M: 2.5, cachePer1M: 0.075 }
    ];
    pricingDataCache = defaults;
    renderPricingTable(pricingDataCache);
    fetch("/api/config/pricing", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ pricing: defaults })
    }).then(function(res) {
      return res.json();
    }).then(function(data) {
      if (data.error) {
        showToast("\u274C " + data.error, "error");
        return;
      }
      showToast("\u21BB Pricing reset to defaults", "success");
    }).catch(function() {
      showToast("\u274C Failed to reset pricing", "error");
    });
  }

  // internal/web/src/settings/settings.ts
  function initSettings() {
    var themeEl = document.getElementById("s-theme");
    var savedTheme = localStorage.getItem("niyantra-theme") || "dark";
    themeEl.value = savedTheme;
    if (!themeEl) return;
    themeEl.addEventListener("change", function() {
      var val = themeEl.value;
      if (val === "system") {
        localStorage.removeItem("niyantra-theme");
        var prefer = window.matchMedia("(prefers-color-scheme: light)").matches ? "light" : "dark";
        document.documentElement.setAttribute("data-theme", prefer);
      } else {
        localStorage.setItem("niyantra-theme", val);
        document.documentElement.setAttribute("data-theme", val);
      }
      var applied = document.documentElement.getAttribute("data-theme");
      updateChartTheme(applied);
    });
    loadConfig().then(function() {
      var cfg = serverConfig;
      migrateLocalStorage(cfg);
      var budgetEl = document.getElementById("s-budget");
      var currencyEl = document.getElementById("s-currency");
      var autoCaptureEl = document.getElementById("s-auto-capture");
      var autoLinkEl = document.getElementById("s-auto-link");
      var pollEl = document.getElementById("s-poll-interval");
      var retentionEl = document.getElementById("s-retention");
      budgetEl.value = String(parseFloat(cfg["budget_monthly"] || "0") || "");
      currencyEl.value = cfg["currency"] || "USD";
      autoCaptureEl.checked = cfg["auto_capture"] === "true";
      autoLinkEl.checked = cfg["auto_link_subs"] !== "false";
      pollEl.value = cfg["poll_interval"] || "300";
      retentionEl.value = cfg["retention_days"] || "365";
      document.getElementById("poll-interval-row").style.display = autoCaptureEl.checked ? "" : "none";
      budgetEl.addEventListener("change", function() {
        var val = parseFloat(budgetEl.value) || 0;
        setBudget(val);
        if (val > 0) showToast("\u2705 Budget: $" + val.toFixed(0) + "/mo", "success");
      });
      currencyEl.addEventListener("change", function() {
        updateConfig("currency", currencyEl.value);
        showToast("\u2705 Currency: " + currencyEl.value, "success");
      });
      autoCaptureEl.addEventListener("change", function() {
        var val = autoCaptureEl.checked ? "true" : "false";
        updateConfig("auto_capture", val).then(function() {
          loadMode();
          showToast(autoCaptureEl.checked ? "\u{1F7E2} Auto-capture started" : "\u23F8\uFE0F Auto-capture stopped", "success");
        });
        document.getElementById("poll-interval-row").style.display = autoCaptureEl.checked ? "" : "none";
      });
      autoLinkEl.addEventListener("change", function() {
        updateConfig("auto_link_subs", autoLinkEl.checked ? "true" : "false");
      });
      pollEl.addEventListener("change", function() {
        var v = pollEl.value;
        updateConfig("poll_interval", v).then(function() {
          var label = pollEl.options[pollEl.selectedIndex].text;
          showToast("\u23F1\uFE0F Interval updated to " + label + " \u2014 takes effect on next cycle.", "success");
          loadMode();
        });
      });
      retentionEl.addEventListener("change", function() {
        var v = parseInt(retentionEl.value);
        if (v >= 30 && v <= 3650) updateConfig("retention_days", v.toString());
      });
      var claudeBridgeEl = document.getElementById("s-claude-bridge");
      if (claudeBridgeEl) {
        claudeBridgeEl.checked = cfg["claude_bridge"] === "true";
        claudeBridgeEl.addEventListener("change", function() {
          var val = claudeBridgeEl.checked ? "true" : "false";
          updateConfig("claude_bridge", val).then(function() {
            showToast(claudeBridgeEl.checked ? "\u{1F517} Claude Code bridge enabled" : "\u{1F517} Bridge disabled", "success");
            loadClaudeBridgeStatus();
          });
        });
        loadClaudeBridgeStatus();
      }
      var notifyEl = document.getElementById("s-notify-enabled");
      var thresholdEl = document.getElementById("s-notify-threshold");
      var thresholdRow = document.getElementById("notify-threshold-row");
      var testRow = document.getElementById("notify-test-row");
      if (notifyEl) {
        notifyEl.checked = cfg["notify_enabled"] === "true";
        thresholdEl.value = cfg["notify_threshold"] || "10";
        thresholdRow.style.display = notifyEl.checked ? "" : "none";
        testRow.style.display = notifyEl.checked ? "" : "none";
        notifyEl.addEventListener("change", function() {
          var val = notifyEl.checked ? "true" : "false";
          updateConfig("notify_enabled", val).then(function() {
            showToast(notifyEl.checked ? "\u{1F514} Notifications enabled" : "\u{1F515} Notifications disabled", "success");
          });
          thresholdRow.style.display = notifyEl.checked ? "" : "none";
          testRow.style.display = notifyEl.checked ? "" : "none";
        });
        thresholdEl.addEventListener("change", function() {
          var v = parseInt(thresholdEl.value);
          if (v >= 5 && v <= 50) {
            updateConfig("notify_threshold", v.toString());
            showToast("\u{1F514} Threshold: " + v + "%", "success");
          }
        });
        document.getElementById("notify-test-btn").addEventListener("click", function() {
          fetch("/api/notify/test", { method: "POST" }).then(function(r) {
            return r.json();
          }).then(function(data) {
            if (data.error) showToast("\u274C " + data.error, "error");
            else showToast("\u{1F514} Test notification sent!", "success");
          }).catch(function() {
            showToast("\u274C Failed to send test", "error");
          });
        });
      }
      var codexCaptureEl = document.getElementById("s-codex-capture");
      if (codexCaptureEl) {
        codexCaptureEl.checked = cfg["codex_capture"] === "true";
        codexCaptureEl.addEventListener("change", function() {
          var val = codexCaptureEl.checked ? "true" : "false";
          updateConfig("codex_capture", val).then(function() {
            showToast(codexCaptureEl.checked ? "\u{1F916} Codex capture enabled" : "\u{1F916} Codex capture disabled", "success");
            loadCodexSettingsStatus();
            loadDataSources();
          });
        });
        loadCodexSettingsStatus();
      }
      var cursorCaptureEl = document.getElementById("s-cursor-capture");
      if (cursorCaptureEl) {
        cursorCaptureEl.checked = cfg["cursor_capture"] === "true";
        cursorCaptureEl.addEventListener("change", function() {
          var val = cursorCaptureEl.checked ? "true" : "false";
          updateConfig("cursor_capture", val).then(function() {
            showToast(cursorCaptureEl.checked ? "\u{1F5B1}\uFE0F Cursor capture enabled" : "\u{1F5B1}\uFE0F Cursor capture disabled", "success");
            loadDataSources();
          });
        });
      }
      var geminiCaptureEl = document.getElementById("s-gemini-capture");
      if (geminiCaptureEl) {
        geminiCaptureEl.checked = cfg["gemini_capture"] === "true";
        geminiCaptureEl.addEventListener("change", function() {
          var val = geminiCaptureEl.checked ? "true" : "false";
          updateConfig("gemini_capture", val).then(function() {
            showToast(geminiCaptureEl.checked ? "\u2728 Gemini capture enabled" : "\u2728 Gemini capture disabled", "success");
            loadDataSources();
          });
        });
      }
      var importBtn = document.getElementById("import-json-btn");
      var importFile = document.getElementById("import-file");
      if (importBtn && importFile) {
        importBtn.addEventListener("click", function() {
          importFile.click();
        });
        importFile.addEventListener("change", function() {
          if (!importFile.files || !importFile.files[0]) return;
          var file = importFile.files[0];
          showToast("\u{1F4E5} Importing " + file.name + "...", "info");
          var reader = new FileReader();
          reader.onload = function(e) {
            fetch("/api/import/json", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: e.target.result
            }).then(function(r) {
              return r.json();
            }).then(function(data) {
              if (data.error) {
                showToast("\u274C Import failed: " + data.error, "error");
                return;
              }
              var msg = "\u2705 Imported: " + (data.accountsCreated || 0) + " accounts, " + (data.subsCreated || 0) + " subs, " + (data.snapshotsImported || 0) + " snapshots";
              showToast(msg, "success");
              var resultEl = document.getElementById("import-result");
              if (resultEl) {
                resultEl.style.display = "";
                resultEl.innerHTML = '<span style="color:var(--accent)">' + msg + "</span>" + (data.accountsSkipped ? "<br>Accounts skipped (existing): " + data.accountsSkipped : "") + (data.subsSkipped ? "<br>Subs skipped (existing): " + data.subsSkipped : "") + (data.snapshotsDuped ? "<br>Snapshots deduped: " + data.snapshotsDuped : "") + (data.errors && data.errors.length ? "<br>\u26A0\uFE0F Errors: " + data.errors.length : "");
              }
              fetchStatus().then(renderAccounts);
              loadSubscriptions();
            }).catch(function() {
              showToast("\u274C Import failed", "error");
            });
          };
          reader.readAsText(file);
          importFile.value = "";
        });
      }
    });
    loadModelPricing();
    document.getElementById("pricing-add-btn").addEventListener("click", addPricingRow);
    document.getElementById("pricing-reset-btn").addEventListener("click", resetPricingDefaults);
    loadMode();
    loadDataSources();
    document.getElementById("activity-refresh").addEventListener("click", loadActivityLog);
    document.getElementById("activity-filter").addEventListener("change", loadActivityLog);
    loadActivityLog();
  }
  function migrateLocalStorage(cfg) {
    var lsBudget = localStorage.getItem("niyantra-budget");
    var lsCurrency = localStorage.getItem("niyantra-currency");
    if (lsBudget && (!cfg["budget_monthly"] || cfg["budget_monthly"] === "0")) {
      updateConfig("budget_monthly", lsBudget);
      serverConfig["budget_monthly"] = lsBudget;
      localStorage.removeItem("niyantra-budget");
    }
    if (lsCurrency && cfg["currency"] === "USD") {
      updateConfig("currency", lsCurrency);
      serverConfig["currency"] = lsCurrency;
      localStorage.removeItem("niyantra-currency");
    }
  }

  // internal/web/src/advanced/palette.ts
  var PALETTE_COMMANDS = [
    { name: "Snap Now", key: "S", icon: "\u{1F4F8}", action: function() {
      handleSnap();
    } },
    { name: "Show Quotas", key: "1", icon: "\u{1F4CA}", action: function() {
      switchToTab("quotas");
    } },
    { name: "Show Subscriptions", key: "2", icon: "\u{1F4B3}", action: function() {
      switchToTab("subscriptions");
    } },
    { name: "Show Overview", key: "3", icon: "\u{1F4CB}", action: function() {
      switchToTab("overview");
    } },
    { name: "Show Settings", key: "4", icon: "\u2699\uFE0F", action: function() {
      switchToTab("settings");
    } },
    { name: "New Subscription", key: "N", icon: "\u2795", action: function() {
      openModal();
    } },
    { name: "Toggle Auto-Capture", icon: "\u{1F504}", action: function() {
      var el = document.getElementById("s-auto-capture");
      if (el) {
        el.checked = !el.checked;
        el.dispatchEvent(new Event("change"));
      }
    } },
    { name: "Export CSV", icon: "\u{1F4E5}", action: function() {
      window.location.href = "/api/export/csv";
    } },
    { name: "Export JSON", icon: "\u{1F4E6}", action: function() {
      window.location.href = "/api/export/json";
    } },
    { name: "Download Backup", icon: "\u{1F4BE}", action: function() {
      window.location.href = "/api/backup";
    } },
    { name: "Search Subscriptions", key: "/", icon: "\u{1F50D}", action: function() {
      switchToTab("subscriptions");
      setTimeout(function() {
        var s = document.getElementById("search-subs");
        if (s) s.focus();
      }, 100);
    } },
    { name: "Set Budget", icon: "\u{1F4B0}", action: function() {
      openBudgetModal();
    } },
    { name: "Toggle Theme", icon: "\u{1F313}", action: function() {
      var cur = document.documentElement.getAttribute("data-theme");
      var next = cur === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      localStorage.setItem("niyantra-theme", next);
      var themeEl = document.getElementById("s-theme");
      if (themeEl) themeEl.value = next;
      updateChartTheme(next);
    } },
    { name: "Codex Snap", icon: "\u{1F916}", action: function() {
      handleCodexSnap();
    } },
    { name: "Import JSON", icon: "\u{1F4E5}", action: function() {
      var f = document.getElementById("import-file");
      if (f) f.click();
    } }
  ];
  var paletteSelectedIndex = 0;
  var paletteFilteredCommands = PALETTE_COMMANDS;
  function initCommandPalette() {
    var overlay = document.getElementById("command-palette-overlay");
    var search = document.getElementById("command-palette-search");
    if (!overlay || !search) return;
    overlay.addEventListener("click", function(e) {
      if (e.target === overlay) closeCommandPalette();
    });
    search.addEventListener("input", function() {
      var query = search.value.toLowerCase().trim();
      paletteFilteredCommands = PALETTE_COMMANDS.filter(function(cmd) {
        return cmd.name.toLowerCase().indexOf(query) >= 0;
      });
      paletteSelectedIndex = 0;
      renderPaletteList();
    });
    search.addEventListener("keydown", function(e) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        paletteSelectedIndex = Math.min(paletteSelectedIndex + 1, paletteFilteredCommands.length - 1);
        renderPaletteList();
      } else if (e.key === "ArrowUp") {
        e.preventDefault();
        paletteSelectedIndex = Math.max(paletteSelectedIndex - 1, 0);
        renderPaletteList();
      } else if (e.key === "Enter") {
        e.preventDefault();
        if (paletteFilteredCommands[paletteSelectedIndex]) {
          closeCommandPalette();
          paletteFilteredCommands[paletteSelectedIndex].action();
        }
      } else if (e.key === "Escape") {
        closeCommandPalette();
      }
    });
  }
  function toggleCommandPalette() {
    var overlay = document.getElementById("command-palette-overlay");
    if (overlay.hidden) {
      openCommandPalette();
    } else {
      closeCommandPalette();
    }
  }
  function openCommandPalette() {
    var overlay = document.getElementById("command-palette-overlay");
    var search = document.getElementById("command-palette-search");
    overlay.hidden = false;
    search.value = "";
    paletteFilteredCommands = PALETTE_COMMANDS;
    paletteSelectedIndex = 0;
    renderPaletteList();
    setTimeout(function() {
      search.focus();
    }, 50);
  }
  function closeCommandPalette() {
    document.getElementById("command-palette-overlay").hidden = true;
  }
  function renderPaletteList() {
    var list = document.getElementById("command-palette-list");
    if (paletteFilteredCommands.length === 0) {
      list.innerHTML = '<div class="command-palette-empty">No matching commands</div>';
      return;
    }
    var html = "";
    for (var i = 0; i < paletteFilteredCommands.length; i++) {
      var cmd = paletteFilteredCommands[i];
      var sel = i === paletteSelectedIndex ? " selected" : "";
      html += '<div class="command-palette-item' + sel + '" data-idx="' + i + '"><span class="cp-icon">' + cmd.icon + '</span><span class="cp-name">' + esc(cmd.name) + "</span>" + (cmd.key ? '<span class="cp-shortcut">' + cmd.key + "</span>" : "") + "</div>";
    }
    list.innerHTML = html;
    list.querySelectorAll(".command-palette-item").forEach(function(el) {
      el.addEventListener("click", function() {
        var idx = parseInt(el.getAttribute("data-idx"));
        closeCommandPalette();
        paletteFilteredCommands[idx].action();
      });
    });
    var selected = list.querySelector(".selected");
    if (selected) selected.scrollIntoView({ block: "nearest" });
  }

  // internal/web/src/advanced/keyboard.ts
  function initKeyboardShortcuts() {
    document.addEventListener("keydown", function(e) {
      var tag = document.activeElement?.tagName;
      if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") {
        if (e.key === "Escape") {
          document.activeElement?.blur();
          closeModal();
          closeDelete();
          closeBudget();
        }
        return;
      }
      var anyModal = !document.getElementById("modal-overlay").hidden || !document.getElementById("delete-overlay").hidden || !document.getElementById("budget-overlay").hidden;
      if (e.key === "Escape") {
        closeModal();
        closeDelete();
        closeBudget();
        return;
      }
      if (anyModal) return;
      switch (e.key) {
        case "1":
          switchToTab("quotas");
          break;
        case "2":
          switchToTab("subscriptions");
          break;
        case "3":
          switchToTab("overview");
          break;
        case "4":
          switchToTab("settings");
          break;
        case "n":
        case "N":
          openModal();
          e.preventDefault();
          break;
        case "s":
        case "S":
          handleSnap();
          e.preventDefault();
          break;
        case "/":
          e.preventDefault();
          switchToTab("subscriptions");
          setTimeout(function() {
            var search = document.getElementById("search-subs");
            if (search) search.focus();
          }, 100);
          break;
      }
    });
    document.addEventListener("keydown", function(e) {
      if ((e.ctrlKey || e.metaKey) && e.key === "k") {
        e.preventDefault();
        toggleCommandPalette();
      }
    });
  }

  // internal/web/src/main.ts
  document.addEventListener("DOMContentLoaded", function() {
    initTheme();
    initTabs();
    setRenderAccounts(renderAccounts);
    initQuotas();
    setupToggle();
    initModal();
    initBudget();
    initSettings();
    initSearch();
    initKeyboardShortcuts();
    initAccountMetaHandlers();
    document.addEventListener("niyantra:tab-change", function(e) {
      var tab = e.detail.tab;
      if (tab === "overview") loadOverview();
      if (tab === "settings") {
        loadActivityLog();
        loadMode();
        loadDataSources();
      }
    });
    document.addEventListener("niyantra:theme-change", function(e) {
      updateChartTheme(e.detail.theme);
    });
    document.addEventListener("niyantra:chart-refresh", function() {
      loadHistoryChart();
    });
    document.addEventListener("niyantra:overview-refresh", function() {
      loadOverview();
    });
    document.getElementById("snap-btn").addEventListener("click", handleSnap);
    initSnapDropdown();
    document.getElementById("chart-account").addEventListener("change", loadHistoryChart);
    document.getElementById("chart-range").addEventListener("change", loadHistoryChart);
    Promise.all([fetchStatus(), fetchUsage()]).then(function(results) {
      var data = results[0];
      renderAccounts(data);
      updateTimestamp();
      populateChartAccountSelect(data);
      loadHistoryChart();
      if (!data.codexSnapshot || !data.claudeSnapshot) {
        setTimeout(function() {
          fetchStatus().then(function(data2) {
            if (data2.codexSnapshot || data2.claudeSnapshot) {
              renderAccounts(data2);
            }
          }).catch(function() {
          });
        }, 3e3);
      }
    }).catch(function(err) {
      console.error("Failed to load status:", err);
    });
    loadSubscriptions();
    fetchPresets().then(function(data) {
      setPresetsData(data.presets || []);
      var list = document.getElementById("preset-list");
      for (var i = 0; i < presetsData.length; i++) {
        var opt = document.createElement("option");
        opt.value = presetsData[i].platform;
        list.appendChild(opt);
      }
    });
    loadMode();
    initCommandPalette();
    loadSystemAlerts();
    setInterval(refreshTimestampDisplay, 3e4);
  });
  window.openBudgetModal = openBudgetModal;
  window.dismissAlert = dismissAlert;
  window.switchToTab = switchToTab;
  window.calendarNav = calendarNav;
  window.handleCodexSnap = handleCodexSnap;
})();
