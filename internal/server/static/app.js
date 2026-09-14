// kgraph web frontend: a project picker (hub mode) plus the Obsidian-style
// force-directed graph viewer. Vanilla JS + <canvas>, no build step, no
// charting library. The server injects window.KGRAPH (mode, page, apiBase,
// eventsPath, ...) so the same assets serve single-project mode (/api/...)
// and hub mode (/api/projects/<key>/...) — see kgraph-project-hub's
// design.md Decision 5.
(() => {
  "use strict";

  const config = Object.assign(
    { mode: "single", page: "viewer", apiBase: "/api", eventsPath: "/events", homeURL: "" },
    window.KGRAPH || {}
  );

  function apiURL(path) {
    return config.apiBase + path;
  }

  async function fetchJSON(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error((await res.json().catch(() => ({}))).error || res.statusText);
    return res.json();
  }

  function escapeHTML(s) {
    const d = document.createElement("div");
    d.textContent = s || "";
    return d.innerHTML;
  }

  if (config.page === "picker") {
    initPicker();
  } else {
    initViewer();
  }

  // ---- Hub mode: project picker ----

  const FRESHNESS_LABELS = {
    current: "up to date",
    stale: "behind HEAD",
    missing: "repository missing",
    unknown: "unknown",
  };

  function relativeTime(unixSeconds) {
    if (!unixSeconds) return "never";
    const diff = Math.max(0, Date.now() / 1000 - unixSeconds);
    const units = [
      [60, "second"], [60, "minute"], [24, "hour"], [7, "day"], [5, "week"], [12, "month"],
    ];
    let value = diff, name = "second";
    for (const [factor, unit] of units) {
      if (value < factor) { name = unit; break; }
      value = value / factor;
      name = unit;
    }
    value = Math.floor(value);
    return value + " " + name + (value === 1 ? "" : "s") + " ago";
  }

  function badge(text, cls) {
    return `<span class="badge badge-${cls}">${escapeHTML(text)}</span>`;
  }

  function projectCard(p) {
    const card = document.createElement("article");
    const selectable = p.status === "built";
    card.className = "project-card" + (selectable ? "" : " disabled");

    let badges = "";
    if (p.status === "built") {
      const fresh = FRESHNESS_LABELS[p.freshness] || p.freshness;
      badges = badge(fresh, p.freshness === "current" ? "ok" : p.freshness === "missing" ? "danger" : "warn");
      if (p.freshness === "missing") badges += badge("prune candidate", "danger");
    } else if (p.status === "never-built") {
      badges = badge("never built", "warn") + badge("prune candidate", "warn");
    } else {
      badges = badge("unreadable", "danger");
    }

    card.innerHTML = `
      <h2>${escapeHTML(p.name)}</h2>
      <div class="project-path">${escapeHTML(p.repo_path || "(no recorded repository path)")}</div>
      <div class="project-meta">${p.node_count} nodes · ${p.edge_count} edges · built ${relativeTime(p.last_build_at)}</div>
      <div class="project-badges">${badges}</div>
    `;
    if (selectable) {
      card.classList.add("selectable");
      card.setAttribute("role", "link");
      card.setAttribute("tabindex", "0");
      const open = () => { location.href = "/p/" + encodeURIComponent(p.key) + "/"; };
      card.addEventListener("click", open);
      card.addEventListener("keydown", (e) => {
        if (e.key === "Enter" || e.key === " ") { e.preventDefault(); open(); }
      });
    }
    return card;
  }

  async function initPicker() {
    const statusEl = document.getElementById("picker-status");
    const grid = document.getElementById("picker-grid");
    try {
      const projects = await fetchJSON(apiURL("/projects"));
      const built = projects.filter((p) => p.status === "built");
      statusEl.textContent = projects.length
        ? projects.length + " project" + (projects.length === 1 ? "" : "s") + " found"
        : "";
      if (built.length === 0) {
        grid.insertAdjacentHTML("beforeend", `<div class="picker-empty">
          <p>No graphs have been built yet.</p>
          <p>Run <code>kgraph build</code> in a repository to create one.</p>
        </div>`);
      }
      for (const p of projects) grid.appendChild(projectCard(p));
    } catch (e) {
      statusEl.textContent = "error: " + e.message;
    }
  }

  // ---- Viewer (single-project mode, and hub-mode per-project pages) ----

  function initViewer() {
    const NODE_TYPES = [
      "Package", "Struct", "Interface", "Function", "Field",
      "Table", "Column", "Endpoint", "ExternalDependency",
      "Enum", "Decorator", "Variable", "TypeAlias",
    ];
    const HIDDEN_BY_DEFAULT = new Set(["Field", "Column"]);
    const COLORS = {
      Package: "#4c6ef5", Struct: "#12b886", Interface: "#f59f00",
      Function: "#7048e8", Field: "#adb5bd", Table: "#e64980",
      Column: "#ced4da", Endpoint: "#fa5252", ExternalDependency: "#495057",
      Enum: "#845ef7", Decorator: "#ff922b", Variable: "#20c997",
      TypeAlias: "#339af0",
    };

    const canvas = document.getElementById("graph");
    const ctx = canvas.getContext("2d");
    const searchInput = document.getElementById("search");
    const searchResults = document.getElementById("search-results");
    const backGlobalBtn = document.getElementById("back-global");
    const filtersEl = document.getElementById("filters");
    const statusEl = document.getElementById("status");
    const detailEl = document.getElementById("detail");
    const detailBody = document.getElementById("detail-body");
    const detailClose = document.getElementById("detail-close");

    const state = {
      mode: "global", // "global" | "local"
      centerId: null,
      types: new Set(NODE_TYPES.filter((t) => !HIDDEN_BY_DEFAULT.has(t))),
      nodes: new Map(), // id -> sim node {id,type,signature,file,degree,x,y,vx,vy}
      edges: [],
      hoverId: null,
      selectedId: null,
      transform: { x: 0, y: 0, scale: 1 },
      dragNode: null,
      panning: false,
      lastMouse: null,
    };

    function resize() {
      canvas.width = window.innerWidth * devicePixelRatio;
      canvas.height = window.innerHeight * devicePixelRatio;
      canvas.style.width = window.innerWidth + "px";
      canvas.style.height = window.innerHeight + "px";
    }
    window.addEventListener("resize", resize);
    resize();
    state.transform.x = window.innerWidth / 2;
    state.transform.y = window.innerHeight / 2;

    function typesParam() {
      return Array.from(state.types).join(",");
    }

    function radiusFor(degree) {
      return Math.min(22, 5 + Math.sqrt(degree || 0) * 3);
    }

    // mergeGraph replaces state.nodes/edges with a new payload while keeping
    // the existing x/y/vx/vy for any node that persists across the refresh,
    // so a live-refresh (SSE) or filter change doesn't reset the layout.
    function mergeGraph(dto) {
      const next = new Map();
      for (const n of dto.nodes) {
        const prev = state.nodes.get(n.id);
        next.set(n.id, {
          id: n.id, type: n.type, signature: n.signature, file: n.file,
          degree: n.degree,
          x: prev ? prev.x : (Math.random() - 0.5) * 200,
          y: prev ? prev.y : (Math.random() - 0.5) * 200,
          vx: prev ? prev.vx : 0,
          vy: prev ? prev.vy : 0,
        });
      }
      state.nodes = next;
      state.edges = dto.edges;
    }

    async function loadGlobal() {
      statusEl.textContent = "loading…";
      const dto = await fetchJSON(apiURL("/graph?types=" + encodeURIComponent(typesParam())));
      mergeGraph(dto);
      state.mode = "global";
      state.centerId = null;
      backGlobalBtn.hidden = true;
      statusEl.textContent = `${dto.nodes.length} nodes, ${dto.edges.length} edges`;
    }

    async function loadLocal(id, hops) {
      statusEl.textContent = "loading…";
      const dto = await fetchJSON(
        apiURL("/graph/local?id=" + encodeURIComponent(id) + "&hops=" + (hops || 2))
      );
      mergeGraph(dto);
      state.mode = "local";
      state.centerId = id;
      backGlobalBtn.hidden = false;
      statusEl.textContent = `local graph: ${dto.nodes.length} nodes, ${dto.edges.length} edges`;
    }

    async function refreshCurrentView() {
      if (state.mode === "local" && state.centerId) {
        await loadLocal(state.centerId);
      } else {
        await loadGlobal();
      }
    }

    // ---- Detail panel ----

    function noteHTML(note, label) {
      if (!note || note.state === "none") {
        return `<div class="note state-none">${label ? label + ": " : ""}no note yet</div>`;
      }
      const staleFlag = note.state === "stale" ? `<span class="stale-flag">⚠ stale — code changed since this note was written</span>` : "";
      return `<div class="note state-${note.state}">${staleFlag}${escapeHTML(note.summary)}</div>`;
    }

    async function openDetail(id) {
      let detail;
      try {
        detail = await fetchJSON(apiURL("/node?id=" + encodeURIComponent(id)));
      } catch (e) {
        statusEl.textContent = "error: " + e.message;
        return;
      }
      state.selectedId = id;

      const rels = detail.relations
        .map((r) => `<div class="relation" data-id="${escapeHTML(r.node_id)}">
          <span class="edge-type">${r.direction === "out" ? "→" : "←"} ${escapeHTML(r.edge_type)}</span>
          ${escapeHTML(r.node_id)} <span style="color:var(--text-dim)">(${escapeHTML(r.node_type)})</span>
        </div>`)
        .join("");

      detailBody.innerHTML = `
        <h2>${escapeHTML(detail.id)}</h2>
        <span class="type-badge" style="background:${COLORS[detail.type] || "#666"}22;color:${COLORS[detail.type] || "#ccc"}">${escapeHTML(detail.type)}</span>
        ${detail.signature ? `<div class="signature">${escapeHTML(detail.signature)}</div>` : ""}
        ${detail.file ? `<div class="file-line">${escapeHTML(detail.file)}${detail.line_start ? ":" + detail.line_start : ""}</div>` : ""}
        ${noteHTML(detail.note)}
        ${detail.file_note ? `<h3>File note</h3>${noteHTML(detail.file_note)}` : ""}
        ${rels ? `<h3>Relations (${detail.relations.length})</h3>${rels}` : ""}
      `;
      detailEl.hidden = false;

      detailBody.querySelectorAll(".relation").forEach((el) => {
        el.addEventListener("click", () => {
          const id = el.getAttribute("data-id");
          loadLocal(id).then(() => openDetail(id));
        });
      });
    }

    detailClose.addEventListener("click", () => {
      detailEl.hidden = true;
      state.selectedId = null;
    });

    // ---- Filters ----

    function renderFilters() {
      filtersEl.innerHTML = "";
      for (const t of NODE_TYPES) {
        const label = document.createElement("label");
        label.innerHTML = `<span class="swatch" style="background:${COLORS[t]}"></span>${t}`;
        const cb = document.createElement("input");
        cb.type = "checkbox";
        cb.checked = state.types.has(t);
        cb.style.display = "none";
        label.prepend(cb);
        label.style.opacity = cb.checked ? "1" : "0.5";
        cb.addEventListener("change", () => {
          if (cb.checked) state.types.add(t);
          else state.types.delete(t);
          label.style.opacity = cb.checked ? "1" : "0.5";
          if (state.mode === "global") loadGlobal();
        });
        filtersEl.appendChild(label);
      }
    }
    renderFilters();

    backGlobalBtn.addEventListener("click", () => {
      loadGlobal();
    });

    // ---- Search ----

    let searchTimer = null;
    searchInput.addEventListener("input", () => {
      clearTimeout(searchTimer);
      const q = searchInput.value.trim();
      if (!q) {
        searchResults.classList.remove("open");
        return;
      }
      searchTimer = setTimeout(async () => {
        let results;
        try {
          results = await fetchJSON(apiURL("/search?q=" + encodeURIComponent(q) + "&topK=10"));
        } catch {
          return;
        }
        searchResults.innerHTML = results
          .map((r) => `<div class="result" data-id="${escapeHTML(r.id)}">${escapeHTML(r.id)}<span class="type">${escapeHTML(r.type)}</span></div>`)
          .join("") || `<div class="result" style="color:var(--text-dim)">no matches</div>`;
        searchResults.classList.add("open");
        searchResults.querySelectorAll(".result[data-id]").forEach((el) => {
          el.addEventListener("click", async () => {
            const id = el.getAttribute("data-id");
            searchResults.classList.remove("open");
            searchInput.value = "";
            await loadLocal(id);
            await openDetail(id);
          });
        });
      }, 150);
    });
    document.addEventListener("click", (e) => {
      if (!searchResults.contains(e.target) && e.target !== searchInput) {
        searchResults.classList.remove("open");
      }
    });

    // ---- Physics simulation ----

    const REPULSION = 2200;
    const SPRING_LENGTH = 70;
    const SPRING_K = 0.02;
    const CENTER_K = 0.002;
    const DAMPING = 0.85;

    function step() {
      const nodes = Array.from(state.nodes.values());
      const n = nodes.length;
      if (n === 0) return;

      for (let i = 0; i < n; i++) {
        const a = nodes[i];
        if (a === state.dragNode) continue;
        let fx = -a.x * CENTER_K;
        let fy = -a.y * CENTER_K;
        for (let j = 0; j < n; j++) {
          if (i === j) continue;
          const b = nodes[j];
          let dx = a.x - b.x, dy = a.y - b.y;
          let d2 = dx * dx + dy * dy || 0.01;
          const f = REPULSION / d2;
          const d = Math.sqrt(d2);
          fx += (dx / d) * f;
          fy += (dy / d) * f;
        }
        a.vx = (a.vx + fx) * DAMPING;
        a.vy = (a.vy + fy) * DAMPING;
      }

      for (const e of state.edges) {
        const a = state.nodes.get(e.src), b = state.nodes.get(e.dst);
        if (!a || !b) continue;
        let dx = b.x - a.x, dy = b.y - a.y;
        const d = Math.sqrt(dx * dx + dy * dy) || 0.01;
        const stretch = d - SPRING_LENGTH;
        const f = stretch * SPRING_K;
        const ux = dx / d, uy = dy / d;
        if (a !== state.dragNode) { a.vx += ux * f; a.vy += uy * f; }
        if (b !== state.dragNode) { b.vx -= ux * f; b.vy -= uy * f; }
      }

      for (const a of nodes) {
        if (a === state.dragNode) continue;
        a.x += a.vx * 0.02;
        a.y += a.vy * 0.02;
      }
    }

    function neighborsOf(id) {
      const set = new Set([id]);
      for (const e of state.edges) {
        if (e.src === id) set.add(e.dst);
        if (e.dst === id) set.add(e.src);
      }
      return set;
    }

    function draw() {
      ctx.save();
      ctx.setTransform(devicePixelRatio, 0, 0, devicePixelRatio, 0, 0);
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.translate(state.transform.x, state.transform.y);
      ctx.scale(state.transform.scale, state.transform.scale);

      const dim = state.hoverId ? neighborsOf(state.hoverId) : null;

      ctx.lineWidth = 1 / state.transform.scale;
      for (const e of state.edges) {
        const a = state.nodes.get(e.src), b = state.nodes.get(e.dst);
        if (!a || !b) continue;
        const faded = dim && !(dim.has(e.src) && dim.has(e.dst));
        ctx.strokeStyle = faded ? "rgba(255,255,255,0.06)" : "rgba(255,255,255,0.25)";
        ctx.beginPath();
        ctx.moveTo(a.x, a.y);
        ctx.lineTo(b.x, b.y);
        ctx.stroke();
      }

      for (const a of state.nodes.values()) {
        const faded = dim && !dim.has(a.id);
        const r = radiusFor(a.degree);
        ctx.globalAlpha = faded ? 0.15 : 1;
        ctx.fillStyle = COLORS[a.type] || "#999";
        ctx.beginPath();
        ctx.arc(a.x, a.y, r, 0, Math.PI * 2);
        ctx.fill();
        if (a.id === state.selectedId) {
          ctx.lineWidth = 2 / state.transform.scale;
          ctx.strokeStyle = "#fff";
          ctx.stroke();
        }
        if (!faded && state.transform.scale > 0.6) {
          ctx.globalAlpha = faded ? 0.15 : 0.85;
          ctx.fillStyle = "#fff";
          ctx.font = `${11 / state.transform.scale}px sans-serif`;
          ctx.fillText(shortLabel(a.id), a.x + r + 3, a.y + 3);
        }
        ctx.globalAlpha = 1;
      }
      ctx.restore();
    }

    function shortLabel(id) {
      const parts = id.split(/[./]/);
      return parts[parts.length - 1].slice(0, 40);
    }

    function loop() {
      step();
      draw();
      requestAnimationFrame(loop);
    }
    requestAnimationFrame(loop);

    // ---- Interaction: pan, zoom, drag, hover, click ----

    function screenToWorld(sx, sy) {
      return {
        x: (sx - state.transform.x) / state.transform.scale,
        y: (sy - state.transform.y) / state.transform.scale,
      };
    }

    function nodeAt(sx, sy) {
      const w = screenToWorld(sx, sy);
      for (const n of state.nodes.values()) {
        const r = radiusFor(n.degree);
        const dx = w.x - n.x, dy = w.y - n.y;
        if (dx * dx + dy * dy <= r * r) return n;
      }
      return null;
    }

    canvas.addEventListener("mousedown", (e) => {
      const n = nodeAt(e.clientX, e.clientY);
      if (n) {
        state.dragNode = n;
        canvas.classList.add("dragging");
      } else {
        state.panning = true;
        canvas.classList.add("dragging");
      }
      state.lastMouse = { x: e.clientX, y: e.clientY };
    });

    window.addEventListener("mousemove", (e) => {
      if (state.dragNode) {
        const w = screenToWorld(e.clientX, e.clientY);
        state.dragNode.x = w.x;
        state.dragNode.y = w.y;
        state.dragNode.vx = 0;
        state.dragNode.vy = 0;
      } else if (state.panning && state.lastMouse) {
        state.transform.x += e.clientX - state.lastMouse.x;
        state.transform.y += e.clientY - state.lastMouse.y;
        state.lastMouse = { x: e.clientX, y: e.clientY };
      } else {
        const n = nodeAt(e.clientX, e.clientY);
        state.hoverId = n ? n.id : null;
        canvas.style.cursor = n ? "pointer" : "grab";
      }
    });

    window.addEventListener("mouseup", (e) => {
      if (state.dragNode === null && state.panning === false) return;
      const wasDrag = state.dragNode !== null;
      const moved = state.lastMouse && (Math.abs(e.clientX - (state.lastMouse.x || 0)) > 3);
      state.dragNode = null;
      state.panning = false;
      canvas.classList.remove("dragging");
    });

    canvas.addEventListener("click", (e) => {
      const n = nodeAt(e.clientX, e.clientY);
      if (n) {
        loadLocal(n.id).then(() => openDetail(n.id));
      }
    });

    canvas.addEventListener("wheel", (e) => {
      e.preventDefault();
      const before = screenToWorld(e.clientX, e.clientY);
      const factor = e.deltaY < 0 ? 1.1 : 0.9;
      state.transform.scale = Math.max(0.05, Math.min(6, state.transform.scale * factor));
      const after = screenToWorld(e.clientX, e.clientY);
      state.transform.x += (after.x - before.x) * state.transform.scale;
      state.transform.y += (after.y - before.y) * state.transform.scale;
    }, { passive: false });

    // ---- Live refresh (SSE) ----

    function connectEvents() {
      if (!config.eventsPath) return;
      const es = new EventSource(config.eventsPath);
      es.addEventListener("graph-updated", () => {
        refreshCurrentView();
      });
      es.onerror = () => {
        // EventSource auto-reconnects on its own; nothing to do here.
      };
    }

    loadGlobal().then(connectEvents);
  }
})();
