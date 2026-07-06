(() => {
  let pollTimer = null;

  const qtypeNames = { 1: "A", 28: "AAAA", 5: "CNAME", 15: "MX", 16: "TXT", 2: "NS", 6: "SOA", 33: "SRV", 65: "HTTPS" };
  const domainTypeNames = { 0: "Whitelist (esatto)", 1: "Blacklist (esatto)", 2: "Whitelist (regex)", 3: "Blacklist (regex)" };
  const statusNames = {
    0: ["Sconosciuto", "badge-allowed"],
    1: ["Bloccato (blocklist)", "badge-blocked"],
    2: ["Consentito", "badge-allowed"],
    4: ["Bloccato (regex)", "badge-blocked"],
    5: ["Bloccato (manuale)", "badge-blocked"],
    6: ["Errore upstream", "badge-blocked"],
  };

  async function api(path, opts = {}) {
    const headers = Object.assign({ "Content-Type": "application/json" }, opts.headers || {});
    const res = await fetch(path, Object.assign({}, opts, { headers }));
    if (!res.ok) {
      const body = await res.json().catch(() => ({}));
      throw new Error(body.error || `request failed: ${res.status}`);
    }
    if (res.status === 204) return null;
    return res.json();
  }

  // --- Tabs ---
  function initTabs() {
    document.querySelectorAll(".tab").forEach((tab) => {
      tab.addEventListener("click", () => {
        document.querySelectorAll(".tab").forEach((t) => t.classList.remove("active"));
        document.querySelectorAll(".tab-content").forEach((c) => c.classList.remove("active"));
        tab.classList.add("active");
        document.getElementById(`tab-${tab.dataset.tab}`).classList.add("active");
        onTabShown(tab.dataset.tab);
      });
    });
  }

  function onTabShown(tab) {
    if (tab === "dashboard") loadDashboard();
    if (tab === "querylog") loadQueryLog();
    if (tab === "domains") loadDomains();
    if (tab === "lists") loadLists();
    if (tab === "groups") loadGroups();
    if (tab === "clients") loadClients();
  }

  // --- Blocking control ---
  async function refreshBlockingStatus() {
    const data = await api("/api/dns/blocking");
    const pill = document.getElementById("blocking-status");
    const enableBtn = document.getElementById("btn-enable");
    if (data.blocking) {
      pill.textContent = "Attivo";
      pill.className = "status-pill status-on";
      enableBtn.classList.add("hidden");
    } else {
      const until = data.disabled_until
        ? ` (fino alle ${new Date(data.disabled_until * 1000).toLocaleTimeString()})`
        : "";
      pill.textContent = "Disattivato" + until;
      pill.className = "status-pill status-off";
      enableBtn.classList.remove("hidden");
    }
  }

  document.querySelectorAll(".btn-toggle:not(.enable)").forEach((btn) => {
    btn.addEventListener("click", async () => {
      const timer = parseInt(btn.dataset.timer, 10);
      await api("/api/dns/blocking", { method: "POST", body: JSON.stringify({ blocking: false, timer }) });
      refreshBlockingStatus();
    });
  });
  document.getElementById("btn-enable").addEventListener("click", async () => {
    await api("/api/dns/blocking", { method: "POST", body: JSON.stringify({ blocking: true }) });
    refreshBlockingStatus();
  });

  // --- Dashboard ---
  async function loadDashboard() {
    const s = await api("/api/stats/summary?since=86400");
    document.getElementById("stat-total").textContent = s.total_queries;
    document.getElementById("stat-blocked").textContent = s.blocked_queries;
    document.getElementById("stat-percent").textContent = s.percent_blocked.toFixed(1) + "%";
    document.getElementById("stat-gravity").textContent = s.gravity_size;

    renderCountTable("top-domains", s.top_domains);
    renderCountTable("top-blocked", s.top_blocked_domains);
    refreshBlockingStatus();
  }

  function renderCountTable(elId, rows) {
    const body = document.getElementById(elId);
    body.innerHTML = "";
    (rows || []).forEach((r) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `<td>${escapeHtml(r.Domain)}</td><td>${r.Count}</td>`;
      body.appendChild(tr);
    });
  }

  // --- Query log ---
  async function loadQueryLog() {
    const filter = document.getElementById("querylog-filter").value;
    const params = new URLSearchParams({ limit: "150" });
    if (filter) params.set("domain", filter);
    const data = await api(`/api/queries?${params}`);
    const body = document.getElementById("querylog-body");
    body.innerHTML = "";
    (data.queries || []).forEach((q) => {
      const [label, cls] = statusNames[q.Status] || ["?", "badge-allowed"];
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${new Date(q.Timestamp * 1000).toLocaleTimeString()}</td>
        <td>${escapeHtml(q.Domain)}</td>
        <td>${qtypeNames[q.QType] || q.QType}</td>
        <td>${escapeHtml(q.Client)}</td>
        <td><span class="badge ${cls}">${label}</span></td>`;
      body.appendChild(tr);
    });
  }
  document.getElementById("querylog-filter").addEventListener("input", debounce(loadQueryLog, 300));

  // --- Domains ---
  async function loadDomains() {
    const data = await api("/api/domains");
    const body = document.getElementById("domains-body");
    body.innerHTML = "";
    (data.domains || []).forEach((d) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${escapeHtml(d.Domain)}</td>
        <td>${domainTypeNames[d.Type] || d.Type}</td>
        <td>${d.Enabled ? "Attivo" : "Disattivo"}</td>
        <td>
          <button class="link-btn" data-action="toggle" data-id="${d.ID}" data-enabled="${d.Enabled}">${d.Enabled ? "Disattiva" : "Attiva"}</button>
          <button class="link-btn danger" data-action="delete" data-id="${d.ID}">Elimina</button>
        </td>`;
      body.appendChild(tr);
    });
    body.querySelectorAll("[data-action=toggle]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        await api(`/api/domains/${btn.dataset.id}`, { method: "PATCH", body: JSON.stringify({ enabled: btn.dataset.enabled !== "true" }) });
        loadDomains();
      });
    });
    body.querySelectorAll("[data-action=delete]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        await api(`/api/domains/${btn.dataset.id}`, { method: "DELETE" });
        loadDomains();
      });
    });
  }

  document.getElementById("domain-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const domain = document.getElementById("domain-input").value.trim();
    const type = parseInt(document.getElementById("domain-type").value, 10);
    const comment = document.getElementById("domain-comment").value.trim();
    if (!domain) return;
    await api("/api/domains", { method: "POST", body: JSON.stringify({ domain, type, comment }) });
    document.getElementById("domain-input").value = "";
    document.getElementById("domain-comment").value = "";
    loadDomains();
  });

  // --- Lists ---
  async function loadLists() {
    const data = await api("/api/lists");
    const body = document.getElementById("lists-body");
    body.innerHTML = "";
    (data.lists || []).forEach((l) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${escapeHtml(l.Address)}</td>
        <td>${l.Number}</td>
        <td>${escapeHtml(l.Status)}</td>
        <td><input type="checkbox" data-action="toggle" data-id="${l.ID}" ${l.Enabled ? "checked" : ""}></td>
        <td><button class="link-btn danger" data-action="delete" data-id="${l.ID}">Elimina</button></td>`;
      body.appendChild(tr);
    });
    body.querySelectorAll("[data-action=toggle]").forEach((el) => {
      el.addEventListener("change", async () => {
        await api(`/api/lists/${el.dataset.id}`, { method: "PATCH", body: JSON.stringify({ enabled: el.checked }) });
        loadLists();
      });
    });
    body.querySelectorAll("[data-action=delete]").forEach((btn) => {
      btn.addEventListener("click", async () => {
        await api(`/api/lists/${btn.dataset.id}`, { method: "DELETE" });
        loadLists();
      });
    });
  }

  document.getElementById("list-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const address = document.getElementById("list-address").value.trim();
    const comment = document.getElementById("list-comment").value.trim();
    if (!address) return;
    await api("/api/lists", { method: "POST", body: JSON.stringify({ address, type: 0, comment }) });
    document.getElementById("list-address").value = "";
    document.getElementById("list-comment").value = "";
    loadLists();
  });

  document.getElementById("btn-update-gravity").addEventListener("click", async (e) => {
    e.preventDefault();
    e.target.textContent = "Aggiornamento in corso…";
    await api("/api/lists/update", { method: "POST" });
    setTimeout(() => {
      e.target.textContent = "Aggiorna tutte le blocklist";
      loadLists();
    }, 4000);
  });

  // --- Groups ---
  async function loadGroups() {
    const data = await api("/api/groups");
    const body = document.getElementById("groups-body");
    body.innerHTML = "";
    (data.groups || []).forEach((g) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `
        <td>${escapeHtml(g.Name)}</td>
        <td>${escapeHtml(g.Description || "")}</td>
        <td><input type="checkbox" data-action="toggle" data-id="${g.ID}" ${g.Enabled ? "checked" : ""}></td>`;
      body.appendChild(tr);
    });
    body.querySelectorAll("[data-action=toggle]").forEach((el) => {
      el.addEventListener("change", async () => {
        await api(`/api/groups/${el.dataset.id}`, { method: "PATCH", body: JSON.stringify({ enabled: el.checked }) });
        loadGroups();
      });
    });
  }

  document.getElementById("group-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const name = document.getElementById("group-name").value.trim();
    const description = document.getElementById("group-description").value.trim();
    if (!name) return;
    await api("/api/groups", { method: "POST", body: JSON.stringify({ name, description }) });
    document.getElementById("group-name").value = "";
    document.getElementById("group-description").value = "";
    loadGroups();
  });

  // --- Clients ---
  async function loadClients() {
    const data = await api("/api/clients");
    const body = document.getElementById("clients-body");
    body.innerHTML = "";
    (data.clients || []).forEach((c) => {
      const tr = document.createElement("tr");
      tr.innerHTML = `<td>${escapeHtml(c.IP)}</td><td>${escapeHtml(c.Comment || "")}</td>`;
      body.appendChild(tr);
    });
  }

  document.getElementById("client-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const ip = document.getElementById("client-ip").value.trim();
    const comment = document.getElementById("client-comment").value.trim();
    if (!ip) return;
    await api("/api/clients", { method: "POST", body: JSON.stringify({ ip, comment }) });
    document.getElementById("client-ip").value = "";
    document.getElementById("client-comment").value = "";
    loadClients();
  });

  // --- Helpers ---
  function escapeHtml(s) {
    return String(s).replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));
  }
  function debounce(fn, ms) {
    let t;
    return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
  }

  function initApp() {
    initTabs();
    loadDashboard();
    if (pollTimer) clearInterval(pollTimer);
    pollTimer = setInterval(() => {
      const active = document.querySelector(".tab.active");
      if (active) onTabShown(active.dataset.tab);
    }, 5000);
  }

  // --- Boot ---
  initApp();
})();
