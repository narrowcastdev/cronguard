(function() {
  "use strict";

  let checks = [];
  let currentCheckId = null;
  let refreshTimer = null;

  // DOM refs
  const checkList = document.getElementById("check-list");
  const checksBody = document.getElementById("checks-body");
  const noChecks = document.getElementById("no-checks");
  const checkForm = document.getElementById("check-form");
  const checkDetail = document.getElementById("check-detail");
  const settingsPanel = document.getElementById("settings-panel");

  // Init
  document.addEventListener("DOMContentLoaded", function() {
    refreshTimer = setInterval(function() {
      if (!checkList.hidden) loadChecks();
    }, 30000);
    setInterval(updateRelativeTimes, 5000);
    loadChecks().then(function() { routeFromHash(); });

    document.getElementById("logo").addEventListener("click", function(e) { e.preventDefault(); showList(); });
    document.getElementById("filter-name").addEventListener("input", renderChecks);
    document.getElementById("filter-status").addEventListener("change", renderChecks);
    document.getElementById("btn-refresh").addEventListener("click", loadChecks);
    document.getElementById("btn-add-check").addEventListener("click", showAddForm);
    document.getElementById("btn-settings").addEventListener("click", showSettings);
    document.getElementById("form-cancel").addEventListener("click", showList);
    document.getElementById("form").addEventListener("submit", handleFormSubmit);
    document.getElementById("detail-back").addEventListener("click", showList);
    document.getElementById("detail-edit").addEventListener("click", editCurrentCheck);
    document.getElementById("detail-delete").addEventListener("click", deleteCurrentCheck);
    document.getElementById("add-alert-form").addEventListener("submit", addAlertDest);
    document.getElementById("alert-type").addEventListener("change", function() {
      var target = document.getElementById("alert-target");
      if (this.value === "email") {
        target.placeholder = "ops@example.com";
        target.type = "email";
      } else {
        target.placeholder = "https://hooks.slack.com/...";
        target.type = "url";
      }
    });
    document.getElementById("settings-form").addEventListener("submit", handleSettingsSubmit);
    document.getElementById("settings-cancel").addEventListener("click", showList);
    document.getElementById("copy-ping-url").addEventListener("click", copyPingUrl);
  });

  // API helpers
  function api(method, path, body) {
    var opts = { method: method, headers: {} };
    if (body) {
      opts.headers["Content-Type"] = "application/json";
      opts.body = JSON.stringify(body);
    }
    return fetch(path, opts).then(function(resp) {
      if (resp.status === 204) return null;
      if (!resp.ok) return resp.text().then(function(t) { throw new Error(t); });
      return resp.json();
    });
  }

  // Load & render
  function loadChecks() {
    return api("GET", "/api/checks").then(function(data) {
      checks = data || [];
      renderChecks();
      var now = new Date();
      document.getElementById("last-updated").textContent =
        "Updated " + now.toLocaleTimeString();
    }).catch(function(err) {
      console.error("Failed to load checks:", err);
    });
  }

  function renderChecks() {
    checksBody.innerHTML = "";
    if (checks.length === 0) {
      noChecks.hidden = false;
      return;
    }
    noChecks.hidden = true;

    var nameFilter = document.getElementById("filter-name").value.toLowerCase();
    var statusFilter = document.getElementById("filter-status").value;

    var filtered = checks.filter(function(c) {
      if (nameFilter && c.name.toLowerCase().indexOf(nameFilter) === -1) return false;
      if (statusFilter && c.status !== statusFilter) return false;
      return true;
    });

    filtered.forEach(function(c) {
      var tr = document.createElement("tr");
      tr.addEventListener("click", function() { showDetail(c.id); });

      tr.innerHTML =
        '<td data-label="Status"><span class="status-dot ' + c.status + '"></span> ' + c.status + (c.alerted ? ' <span class="alerted-badge">alerted</span>' : '') + '</td>' +
        '<td data-label="Name">' + esc(c.name) + '</td>' +
        '<td data-label="Schedule"><code>' + esc(c.schedule) + '</code></td>' +
        '<td data-label="Last Ping" data-time="' + (c.last_ping || '') + '">' + relTime(c.last_ping) + '</td>' +
        '<td data-label="Next Expected" data-time="' + (c.next_expected || '') + '" data-future="1">' + relTime(c.next_expected, true) + '</td>' +
        '<td data-label="Ping URL"><code>' + esc(c.ping_url) + '</code></td>';

      checksBody.appendChild(tr);
    });
  }

  // Navigation
  function navigate(hash, skipPush) {
    if (!skipPush) {
      history.pushState(null, "", hash || "/");
    }
  }

  window.addEventListener("popstate", function() {
    routeFromHash();
  });

  function routeFromHash() {
    var hash = location.hash;
    if (hash.indexOf("#check/") === 0) {
      var id = parseInt(hash.substring(7));
      if (id) {
        showDetailView(id);
        return;
      }
    }
    if (hash === "#add") {
      showAddFormView();
      return;
    }
    if (hash === "#settings") {
      showSettingsView();
      return;
    }
    showListView();
  }

  // Views
  function showList() {
    navigate("/");
    showListView();
  }

  function showListView() {
    checkList.hidden = false;
    checkForm.hidden = true;
    checkDetail.hidden = true;
    settingsPanel.hidden = true;
    loadChecks();
  }

  function showAddForm() {
    navigate("#add");
    showAddFormView();
  }

  function showAddFormView() {
    document.getElementById("form-title").textContent = "Add check";
    document.getElementById("form-id").value = "";
    document.getElementById("form-name").value = "";
    document.getElementById("form-schedule").value = "";
    document.getElementById("form-grace").value = "5m";
    document.getElementById("ping-info").hidden = true;

    checkList.hidden = true;
    checkForm.hidden = false;
    checkDetail.hidden = true;
    settingsPanel.hidden = true;
  }

  function showEditForm(check) {
    document.getElementById("form-title").textContent = "Edit check";
    document.getElementById("form-id").value = check.id;
    document.getElementById("form-name").value = check.name;
    document.getElementById("form-schedule").value = check.schedule;
    document.getElementById("form-grace").value = check.grace;
    document.getElementById("ping-info").hidden = true;

    checkList.hidden = true;
    checkForm.hidden = false;
    checkDetail.hidden = true;
    settingsPanel.hidden = true;
  }

  function showDetail(id) {
    navigate("#check/" + id);
    showDetailView(id);
  }

  function showDetailView(id) {
    currentCheckId = id;
    var check = checks.find(function(c) { return c.id === id; });
    if (!check) {
      api("GET", "/api/checks").then(function(data) {
        checks = data || [];
        var c = checks.find(function(c) { return c.id === id; });
        if (c) { showDetailView(id); }
        else { showList(); }
      });
      return;
    }

    document.getElementById("detail-name").textContent = check.name;
    var statusEl = document.getElementById("detail-status");
    statusEl.textContent = check.status;
    statusEl.className = "status-badge " + check.status;
    document.getElementById("detail-schedule").textContent = check.schedule;

    var outputSection = document.getElementById("detail-output-section");
    if (check.last_output) {
      document.getElementById("detail-output").textContent = check.last_output;
      outputSection.hidden = false;
    } else {
      outputSection.hidden = true;
    }

    loadAlertDests(id);

    checkList.hidden = true;
    checkForm.hidden = true;
    checkDetail.hidden = false;
    settingsPanel.hidden = true;
  }

  function showSettings() {
    navigate("#settings");
    showSettingsView();
  }

  function showSettingsView() {
    api("GET", "/api/settings").then(function(s) {
      document.getElementById("smtp-host").value = s.smtp_host || "";
      document.getElementById("smtp-port").value = s.smtp_port || 587;
      document.getElementById("smtp-user").value = s.smtp_user || "";
      document.getElementById("smtp-password").value = "";
      document.getElementById("smtp-from").value = s.smtp_from || "";
    });

    checkList.hidden = true;
    checkForm.hidden = true;
    checkDetail.hidden = true;
    settingsPanel.hidden = false;
  }

  // Form handlers
  function handleFormSubmit(e) {
    e.preventDefault();
    var id = document.getElementById("form-id").value;
    var data = {
      name: document.getElementById("form-name").value,
      schedule: document.getElementById("form-schedule").value,
      grace: document.getElementById("form-grace").value
    };

    var promise;
    if (id) {
      promise = api("PUT", "/api/checks/" + id, data);
    } else {
      promise = api("POST", "/api/checks", data);
    }

    promise.then(function(check) {
      if (!id && check) {
        checks.push(check);
        showDetail(check.id);
        return;
      }
      showList();
    }).catch(function(err) {
      alert("Error: " + err.message);
    });
  }

  function showPingInfo(check) {
    var baseUrl = window.location.origin;
    var pingUrl = baseUrl + check.ping_url;
    document.getElementById("ping-url").textContent = pingUrl;
    document.getElementById("ping-snippets").textContent =
      "Add to your cron job:\n" +
      "  && curl -fsS " + pingUrl + "\n\n" +
      "Or to capture output:\n" +
      "  my-command 2>&1 | curl -fsS -d @- " + pingUrl + "\n\n" +
      "Or to report failures:\n" +
      "  my-command || curl -fsS -X POST " + pingUrl + "/fail";
    document.getElementById("ping-info").hidden = false;
  }

  function copyPingUrl() {
    var url = document.getElementById("ping-url").textContent;
    navigator.clipboard.writeText(url).then(function() {
      var btn = document.getElementById("copy-ping-url");
      btn.textContent = "Copied!";
      setTimeout(function() { btn.textContent = "Copy"; }, 2000);
    });
  }

  function editCurrentCheck() {
    var check = checks.find(function(c) { return c.id === currentCheckId; });
    if (check) showEditForm(check);
  }

  function deleteCurrentCheck() {
    if (!confirm("Delete this check? This cannot be undone.")) return;
    api("DELETE", "/api/checks/" + currentCheckId).then(showList).catch(function(err) {
      alert("Error: " + err.message);
    });
  }

  // Alert destinations
  function loadAlertDests(checkId) {
    api("GET", "/api/checks/" + checkId + "/alerts").then(function(dests) {
      var list = document.getElementById("detail-alerts-list");
      list.innerHTML = "";
      (dests || []).forEach(function(d) {
        var li = document.createElement("li");
        li.innerHTML =
          '<span>' + esc(d.type) + ': ' + esc(d.target) + '</span>' +
          '<span>' +
          '<button class="btn btn-small" onclick="testAlert(' + checkId + ',' + d.id + ')">Test</button> ' +
          '<button class="btn btn-small btn-danger" onclick="removeAlert(' + d.id + ')">Remove</button>' +
          '</span>';
        list.appendChild(li);
      });
    });
  }

  function addAlertDest(e) {
    e.preventDefault();
    var data = {
      type: document.getElementById("alert-type").value,
      target: document.getElementById("alert-target").value
    };
    api("POST", "/api/checks/" + currentCheckId + "/alerts", data).then(function() {
      document.getElementById("alert-target").value = "";
      loadAlertDests(currentCheckId);
    }).catch(function(err) {
      alert("Error: " + err.message);
    });
  }

  // Globals for inline onclick handlers
  window.testAlert = function(checkId, alertId) {
    api("POST", "/api/checks/" + checkId + "/alerts/" + alertId + "/test")
      .then(function() { alert("Test alert sent!"); })
      .catch(function(err) { alert("Error: " + err.message); });
  };

  window.removeAlert = function(alertId) {
    if (!confirm("Remove this alert destination?")) return;
    api("DELETE", "/api/alerts/" + alertId).then(function() {
      loadAlertDests(currentCheckId);
    }).catch(function(err) {
      alert("Error: " + err.message);
    });
  };

  // Settings
  function handleSettingsSubmit(e) {
    e.preventDefault();
    var password = document.getElementById("smtp-password").value;
    var data = {
      smtp_host: document.getElementById("smtp-host").value,
      smtp_port: parseInt(document.getElementById("smtp-port").value) || 587,
      smtp_user: document.getElementById("smtp-user").value,
      smtp_password: password || "********",
      smtp_from: document.getElementById("smtp-from").value
    };
    api("PUT", "/api/settings", data).then(function() {
      showList();
    }).catch(function(err) {
      alert("Error: " + err.message);
    });
  }

  function updateRelativeTimes() {
    var cells = checksBody.querySelectorAll("td[data-time]");
    cells.forEach(function(td) {
      var iso = td.getAttribute("data-time");
      var future = td.hasAttribute("data-future");
      td.textContent = relTime(iso, future);
    });
  }

  // Helpers
  function relTime(isoStr, future) {
    if (!isoStr) return "-";
    var date = new Date(isoStr);
    var now = new Date();
    var diff = (now - date) / 1000;

    if (future && diff < 0) {
      diff = Math.abs(diff);
      return "in " + humanDuration(diff);
    }

    if (diff < 0) return "just now";
    return humanDuration(diff) + " ago";
  }

  function humanDuration(seconds) {
    if (seconds < 60) return Math.floor(seconds) + "s";
    if (seconds < 3600) return Math.floor(seconds / 60) + "m";
    if (seconds < 86400) return Math.floor(seconds / 3600) + "h";
    return Math.floor(seconds / 86400) + "d";
  }

  function esc(str) {
    if (!str) return "";
    var div = document.createElement("div");
    div.appendChild(document.createTextNode(str));
    return div.innerHTML;
  }
})();
