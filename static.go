package main

var indexHTML = []byte(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Swap-rithms Algorithm Lab</title>
  <style>
    :root {
      color-scheme: light;
      --bg: #f6f7f9;
      --panel: #ffffff;
      --text: #17202a;
      --muted: #5f6b7a;
      --line: #d8dee8;
      --accent: #1666c1;
      --accent-dark: #0f4d93;
      --danger: #ad2f2f;
      --ok: #20734d;
    }
    * { box-sizing: border-box; }
    body {
      margin: 0;
      background: var(--bg);
      color: var(--text);
      font: 14px/1.4 system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    }
    header {
      border-bottom: 1px solid var(--line);
      background: var(--panel);
      padding: 16px 24px;
    }
    h1 {
      margin: 0;
      font-size: 20px;
      letter-spacing: 0;
    }
    main {
      max-width: 1180px;
      margin: 0 auto;
      padding: 20px;
    }
    .grid {
      display: grid;
      gap: 16px;
    }
    .top-grid {
      grid-template-columns: minmax(0, 1.2fr) minmax(320px, 0.8fr);
    }
    .panel {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 16px;
    }
    .panel h2 {
      margin: 0 0 12px;
      font-size: 15px;
    }
    .row {
      display: flex;
      align-items: center;
      gap: 10px;
      flex-wrap: wrap;
    }
    .control-row {
      align-items: end;
    }
    .control-row #profileCount {
      min-height: 36px;
      display: inline-flex;
      align-items: center;
    }
    label {
      display: grid;
      gap: 5px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 600;
    }
    select, input {
      min-height: 36px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #fff;
      color: var(--text);
      padding: 0 10px;
      font: inherit;
    }
    input[type="number"] { width: 110px; }
    button {
      min-height: 36px;
      border: 1px solid var(--accent);
      border-radius: 6px;
      background: var(--accent);
      color: #fff;
      padding: 0 14px;
      font-weight: 650;
      cursor: pointer;
    }
    button.secondary {
      background: #fff;
      color: var(--accent);
    }
    button.danger {
      border-color: var(--danger);
      background: var(--danger);
    }
    button:disabled {
      opacity: 0.55;
      cursor: not-allowed;
    }
    .algorithm-list {
      display: grid;
      gap: 8px;
      margin-top: 12px;
    }
    .algorithm {
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 10px;
      display: grid;
      gap: 4px;
      overflow: hidden;
      transition: border-color 180ms ease, box-shadow 180ms ease, background 180ms ease;
    }
    .algorithm.selected {
      border-color: var(--accent);
      box-shadow: inset 3px 0 0 var(--accent);
      background: #fbfdff;
    }
    .algorithm-head {
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 8px;
    }
    .algorithm strong {
      font-size: 13px;
    }
    .complexity code {
      color: var(--accent-dark);
      font-weight: 700;
    }
    .badge {
      border: 1px solid var(--line);
      border-radius: 999px;
      color: var(--muted);
      font-size: 11px;
      font-weight: 700;
      line-height: 1;
      padding: 4px 7px;
      white-space: nowrap;
    }
    .badge.empty {
      display: none;
    }
    .code-expander {
      display: grid;
      grid-template-rows: 0fr;
      margin-top: 0;
      opacity: 0;
      transition: grid-template-rows 1000ms ease, margin-top 1000ms ease, opacity 1000ms ease;
    }
    .algorithm.selected .code-expander {
      grid-template-rows: 1fr;
      margin-top: 8px;
      opacity: 1;
    }
    .code-expander-inner {
      min-height: 0;
      overflow: hidden;
      transform: translateY(-6px);
      transition: transform 1000ms ease;
    }
    .algorithm.selected .code-expander-inner {
      transform: translateY(0);
    }
    pre {
      margin: 0;
      max-height: 360px;
      overflow: auto;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #f8fafc;
      color: var(--text);
      padding: 12px;
      font: 12px/1.45 ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      white-space: pre;
    }
    .muted { color: var(--muted); }
    .metrics {
      display: grid;
      grid-template-columns: repeat(4, minmax(0, 1fr));
      gap: 12px;
    }
    .metric {
      background: var(--panel);
      border: 1px solid var(--line);
      border-radius: 8px;
      padding: 14px;
      min-width: 0;
    }
    .metric .label {
      color: var(--muted);
      font-size: 12px;
      font-weight: 650;
    }
    .metric .value {
      margin-top: 6px;
      font-size: 24px;
      font-weight: 750;
      overflow-wrap: anywhere;
    }
    .charts {
      grid-template-columns: repeat(2, minmax(0, 1fr));
    }
    canvas {
      width: 100%;
      height: 240px;
      display: block;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fff;
    }
    .status {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      color: var(--muted);
      font-weight: 650;
    }
    .dot {
      width: 9px;
      height: 9px;
      border-radius: 50%;
      background: var(--muted);
    }
    .dot.running { background: var(--ok); }
    footer {
      color: var(--muted);
      padding: 8px 0 0;
      font-size: 12px;
    }
    @media (max-width: 860px) {
      main { padding: 12px; }
      .top-grid, .charts { grid-template-columns: 1fr; }
      .metrics { grid-template-columns: repeat(2, minmax(0, 1fr)); }
    }
    @media (max-width: 520px) {
      .metrics { grid-template-columns: 1fr; }
      header { padding: 14px 12px; }
    }
  </style>
</head>
<body>
  <header>
    <h1>Swap-rithms Algorithm Lab</h1>
  </header>
  <main class="grid">
    <section class="grid top-grid">
      <div class="panel">
        <h2>Implementation</h2>
        <div class="row control-row">
          <label>
            Active algorithm
            <select id="algorithm"></select>
          </label>
          <button id="applyAlgorithm">Apply</button>
          <span class="muted" id="profileCount"></span>
        </div>
        <div class="algorithm-list" id="algorithmList"></div>
      </div>

      <div class="panel">
        <h2>Load generator</h2>
        <div class="grid">
          <div class="row">
            <label>
              Rate (requests/sec)
              <input id="rate" type="number" min="1" max="2000" value="50">
            </label>
            <label>
              Duration seconds
              <input id="duration" type="number" min="1" max="600" value="60">
            </label>
            <label>
              Recent window seconds
              <input id="window" type="number" min="1" max="86400" value="300">
            </label>
          </div>
          <label class="row" style="display:flex;font-size:13px;font-weight:500;color:var(--text)">
            <input id="includeIds" type="checkbox" style="min-height:auto">
            Include IDs in load responses
          </label>
          <div class="row">
            <button id="startLoad">Start</button>
            <button id="stopLoad" class="danger">Stop</button>
            <span class="status"><span id="loadDot" class="dot"></span><span id="loadStatus">idle</span></span>
          </div>
          <footer>Load requests hit <code>/profiles/recent</code> on this service, so the shown latency is measured from real algorithm executions.</footer>
        </div>
      </div>
    </section>

    <section class="metrics">
      <div class="metric"><div class="label">Throughput</div><div class="value" id="rps">0 rps</div></div>
      <div class="metric"><div class="label">p95 latency</div><div class="value" id="p95">0 ms</div></div>
      <div class="metric"><div class="label">p99 latency</div><div class="value" id="p99">0 ms</div></div>
      <div class="metric"><div class="label">Lab service memory / CPU</div><div class="value" id="runtime">0 MB</div></div>
    </section>

    <section class="grid charts">
      <div class="panel">
        <h2>Latency samples</h2>
        <canvas id="latencyChart" width="600" height="240"></canvas>
      </div>
      <div class="panel">
        <h2>Recent request rate</h2>
        <canvas id="rpsChart" width="600" height="240"></canvas>
      </div>
    </section>
  </main>

  <script>
    const state = {
      activeAlgorithm: "slice_scan",
      selectedAlgorithm: "slice_scan",
      algorithms: [],
      rpsHistory: [],
      latencyHistory: []
    };

    const $ = (id) => document.getElementById(id);
    const fmt = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });

    async function api(path, options = {}) {
      const res = await fetch(path, {
        headers: { "Content-Type": "application/json" },
        ...options
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.error || res.statusText);
      }
      return res.json();
    }

    async function loadState() {
      const data = await api("/api/state");
      state.activeAlgorithm = data.activeAlgorithm;
      state.algorithms = data.algorithms;
      state.selectedAlgorithm = data.activeAlgorithm;
      $("profileCount").textContent = fmt.format(data.profileCount) + " profiles";
      renderAlgorithmControls();
      renderLoad(data.load);
    }

    function renderAlgorithmControls() {
      const select = $("algorithm");
      select.innerHTML = "";
      for (const algo of state.algorithms) {
        const option = document.createElement("option");
        option.value = algo.name;
        option.textContent = algo.label + " (" + algo.complexity + ")";
        option.selected = algo.name === state.selectedAlgorithm;
        select.appendChild(option);
      }
      renderAlgorithmList();
    }

    function renderAlgorithmList() {
      const list = $("algorithmList");
      const existing = list.querySelectorAll(".algorithm");
      if (existing.length === state.algorithms.length) {
        updateAlgorithmCards();
        return;
      }

      list.innerHTML = "";
      for (const algo of state.algorithms) {
        const div = document.createElement("div");
        div.className = "algorithm";
        div.dataset.algorithm = algo.name;
        div.innerHTML = [
          "<div class='algorithm-head'><strong></strong><span class='badge empty'></span></div>",
          "<span class='complexity'></span>",
          "<span class='muted'></span>",
          "<div class='code-expander'><div class='code-expander-inner'><pre><code></code></pre></div></div>"
        ].join("");
        div.querySelector("strong").textContent = algo.label;
        div.querySelector(".complexity").innerHTML = "<code>" + algo.complexity + "</code>";
        div.querySelector(".muted").textContent = algo.description;
        div.querySelector("pre code").textContent = algo.code || "Source snippet unavailable.";
        list.appendChild(div);
      }
      requestAnimationFrame(updateAlgorithmCards);
    }

    function updateAlgorithmCards() {
      for (const card of $("algorithmList").querySelectorAll(".algorithm")) {
        const isSelected = card.dataset.algorithm === state.selectedAlgorithm;
        const isRunning = card.dataset.algorithm === state.activeAlgorithm;
        card.classList.toggle("selected", isSelected);

        const badge = card.querySelector(".badge");
        badge.textContent = isRunning ? "Running" : "";
        badge.classList.toggle("empty", !isRunning);
      }
    }

    function renderLoad(load) {
      const running = Boolean(load.running);
      $("loadDot").className = "dot" + (running ? " running" : "");
      $("loadStatus").textContent = running
        ? "running, " + load.completed + " complete, " + load.inFlight + " in flight"
        : "idle, " + (load.completed || 0) + " complete";
      $("startLoad").disabled = running;
      $("stopLoad").disabled = !running;
    }

    async function pollStats() {
      const data = await api("/api/stats");
      state.activeAlgorithm = data.activeAlgorithm;
      const active = data.algorithms[data.activeAlgorithm] || {};
      $("rps").textContent = fmt.format(active.recentRps || 0) + " rps";
      $("p95").textContent = fmt.format(active.p95Ms || 0) + " ms";
      $("p99").textContent = fmt.format(active.p99Ms || 0) + " ms";
      const cpu = data.runtime.cpuPercent >= 0 ? ", " + fmt.format(data.runtime.cpuPercent) + "% CPU" : "";
      $("runtime").textContent = fmt.format(data.runtime.heapMb || 0) + " MB" + cpu;
      renderLoad(data.load);

      state.rpsHistory.push(active.recentRps || 0);
      if (state.rpsHistory.length > 80) state.rpsHistory.shift();

      const recent = data.recentEvents || [];
      state.latencyHistory = recent.slice(-80).map((event) => event.latencyMs);
      drawLineChart($("latencyChart"), state.latencyHistory, "ms");
      drawLineChart($("rpsChart"), state.rpsHistory, "rps");
    }

    function drawLineChart(canvas, values, suffix) {
      const ctx = canvas.getContext("2d");
      const w = canvas.width;
      const h = canvas.height;
      ctx.clearRect(0, 0, w, h);
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, w, h);
      ctx.strokeStyle = "#d8dee8";
      ctx.lineWidth = 1;
      for (let i = 1; i < 4; i++) {
        const y = Math.round((h / 4) * i);
        ctx.beginPath();
        ctx.moveTo(0, y);
        ctx.lineTo(w, y);
        ctx.stroke();
      }

      if (!values.length) {
        ctx.fillStyle = "#5f6b7a";
        ctx.fillText("No samples yet", 18, 28);
        return;
      }

      const max = Math.max(...values, 1);
      const pad = 18;
      ctx.strokeStyle = "#1666c1";
      ctx.lineWidth = 2;
      ctx.beginPath();
      values.forEach((value, i) => {
        const x = pad + (i / Math.max(values.length - 1, 1)) * (w - pad * 2);
        const y = h - pad - (value / max) * (h - pad * 2);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      });
      ctx.stroke();
      ctx.fillStyle = "#17202a";
      ctx.fillText("max " + fmt.format(max) + " " + suffix, 18, 24);
    }

    $("applyAlgorithm").addEventListener("click", async () => {
      await api("/api/algorithm", {
        method: "POST",
        body: JSON.stringify({ name: $("algorithm").value })
      });
      await loadState();
    });

    $("algorithm").addEventListener("change", () => {
      state.selectedAlgorithm = $("algorithm").value;
      updateAlgorithmCards();
    });

    $("startLoad").addEventListener("click", async () => {
      await api("/api/load/start", {
        method: "POST",
        body: JSON.stringify({
          rate: Number($("rate").value),
          durationSeconds: Number($("duration").value),
          windowSeconds: Number($("window").value),
          includeIds: $("includeIds").checked
        })
      });
      await pollStats();
    });

    $("stopLoad").addEventListener("click", async () => {
      await api("/api/load/stop", { method: "POST", body: "{}" });
      await pollStats();
    });

    loadState().then(pollStats).catch((err) => {
      console.error(err);
      alert(err.message);
    });
    setInterval(() => pollStats().catch(console.error), 1000);
  </script>
</body>
</html>`)
