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
    .focus-row {
      margin-bottom: 12px;
    }
    .focus-label {
      color: var(--muted);
      font-size: 12px;
      font-weight: 700;
    }
    .focus-control {
      display: inline-flex;
      border: 1px solid var(--line);
      border-radius: 6px;
      overflow: hidden;
      background: #fff;
    }
    .segment {
      min-height: 32px;
      border: 0;
      border-right: 1px solid var(--line);
      border-radius: 0;
      background: #fff;
      color: var(--muted);
      padding: 0 10px;
      font-size: 12px;
    }
    .segment:last-child {
      border-right: 0;
    }
    .segment.selected {
      background: var(--accent);
      color: #fff;
    }
    .segment:disabled {
      background: #f3f5f8;
      color: var(--muted);
      cursor: not-allowed;
      opacity: 0.65;
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
    input[type="text"] { width: 180px; }
    [hidden] { display: none !important; }
    .field {
      position: relative;
    }
    .field input[aria-invalid="true"] {
      border-color: var(--danger);
      box-shadow: 0 0 0 2px rgba(173, 47, 47, 0.12);
    }
    .field-hint {
      position: absolute;
      z-index: 6;
      top: calc(100% + 4px);
      left: 0;
      min-width: 190px;
      max-width: 240px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #fff;
      color: var(--muted);
      padding: 4px 6px;
      font-size: 11px;
      font-weight: 550;
      opacity: 0;
      pointer-events: none;
      transform: translateY(-4px);
      transition:
        opacity 180ms ease,
        transform 180ms ease,
        color 180ms ease,
        border-color 180ms ease;
    }
    .field:focus-within .field-hint,
    .field.invalid .field-hint {
      opacity: 1;
      transform: translateY(0);
    }
    .field.invalid .field-hint {
      border-color: rgba(173, 47, 47, 0.45);
      color: var(--danger);
    }
    .duration-option {
      align-self: end;
      position: relative;
      color: var(--muted);
      font-size: 12px;
      font-weight: 600;
      min-height: 36px;
    }
    .duration-toggle {
      min-height: 36px;
      display: inline-flex;
      align-items: center;
      gap: 8px;
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #fff;
      color: var(--text);
      padding: 0 10px;
      font-size: 12px;
      font-weight: 650;
      cursor: pointer;
      user-select: none;
    }
    .duration-toggle::before {
      content: "+";
      color: var(--accent);
      font-weight: 800;
    }
    .duration-option.open .duration-toggle::before {
      content: "-";
    }
    .duration-panel {
      position: absolute;
      z-index: 5;
      top: calc(100% + 8px);
      left: 0;
      width: 180px;
      max-height: 0;
      opacity: 0;
      overflow: hidden;
      pointer-events: none;
      transform: translateY(-8px);
      transition:
        max-height 500ms cubic-bezier(0.22, 1, 0.36, 1),
        opacity 500ms cubic-bezier(0.22, 1, 0.36, 1),
        transform 500ms cubic-bezier(0.22, 1, 0.36, 1);
    }
    .duration-option.open .duration-panel {
      max-height: 120px;
      opacity: 1;
      overflow: visible;
      pointer-events: auto;
      transform: translateY(0);
    }
    .duration-panel label {
      border: 1px solid var(--line);
      border-radius: 6px;
      background: #fff;
      padding: 8px;
      box-shadow: 0 8px 20px rgba(23, 32, 42, 0.12);
    }
    .duration-panel .field-hint {
      position: static;
      min-width: 0;
      max-width: none;
      border: 0;
      background: transparent;
      padding: 0;
      opacity: 1;
      transform: none;
      box-shadow: none;
      pointer-events: none;
    }
    .form-message {
      min-height: 17px;
      color: var(--muted);
      font-size: 12px;
      font-weight: 600;
      transition: color 180ms ease;
    }
    .form-message.error {
      color: var(--danger);
    }
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
    .scenario-detail {
      margin-top: 12px;
      border: 1px solid var(--line);
      border-radius: 8px;
      background: #fbfdff;
      padding: 10px 12px;
      display: grid;
      gap: 5px;
    }
    .scenario-detail strong {
      font-size: 13px;
    }
    .scenario-business {
      color: var(--text);
      font-size: 13px;
    }
    .business-label {
      color: var(--muted);
      font-weight: 700;
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
    .implementation-meta {
      display: flex;
      gap: 6px;
      flex-wrap: wrap;
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
    #runtime {
      display: flex;
      flex-direction: column;
      align-items: baseline;
      gap: 2px;
      overflow-wrap: normal;
    }
    #runtime .runtime-part {
      white-space: nowrap;
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
        <div class="row focus-row">
          <span class="focus-label">Focus</span>
          <div class="focus-control" role="group" aria-label="Focus mode">
            <button class="segment selected" type="button" data-focus="implementation">Implementation</button>
            <button class="segment" type="button" data-focus="algorithms">Algorithms</button>
            <button class="segment" type="button" data-focus="data_structures">Data structures</button>
          </div>
        </div>
        <div class="row control-row">
          <label>
            Scenario
            <select id="scenario"></select>
          </label>
          <label>
            Language
            <select id="language"></select>
          </label>
          <label>
            Algorithm
            <select id="algorithm"></select>
          </label>
          <label>
            Data structure
            <select id="dataStructure"></select>
          </label>
          <button id="applyAlgorithm">Apply</button>
          <span class="muted" id="profileCount"></span>
        </div>
        <div class="scenario-detail">
          <strong id="scenarioTitle">Lookup and indexing</strong>
          <span class="muted" id="scenarioDescription">Find profiles updated within a recent time window.</span>
          <span class="scenario-business"><span class="business-label">Business case:</span> <span id="scenarioBusinessCase"></span></span>
        </div>
        <div class="algorithm-list" id="algorithmList"></div>
      </div>

      <div class="panel">
        <h2>Load generator</h2>
        <div class="grid">
          <div class="row">
            <label id="rateField" class="field">
              Rate (requests/sec)
              <input id="rate" type="number" min="1" max="10000" value="50" aria-describedby="rateHint" aria-invalid="false">
              <span id="rateHint" class="field-hint">Allowed range: 1 to 10,000 requests/sec.</span>
            </label>
            <label id="windowField" class="field">
              <span id="requestValueLabel">Recent window seconds</span>
              <input id="window" type="number" min="1" max="86400" value="300" aria-describedby="windowHint" aria-invalid="false">
              <span id="windowHint" class="field-hint">Allowed range: 1 to 86,400 seconds.</span>
            </label>
            <label id="queryField" class="field" hidden>
              <span id="queryLabel">Search query</span>
              <input id="query" type="text" value="platform" aria-describedby="queryHint" aria-invalid="false">
              <span id="queryHint" class="field-hint">Searches normalized profile text.</span>
            </label>
            <div id="durationOption" class="duration-option">
              <button id="durationToggle" class="duration-toggle" type="button" aria-expanded="false" aria-controls="durationPanel">Run for set time</button>
              <div id="durationPanel" class="duration-panel" aria-hidden="true">
                <label id="durationField" class="field">
                  Duration seconds
                  <input id="duration" type="number" min="1" max="600" value="60" aria-describedby="durationHint" aria-invalid="false" disabled>
                  <span id="durationHint" class="field-hint">Allowed range: 1 to 600 seconds.</span>
                </label>
              </div>
            </div>
          </div>
          <div id="loadFormMessage" class="form-message" role="status" aria-live="polite"></div>
          <label class="row" style="display:flex;font-size:13px;font-weight:500;color:var(--text)">
            <input id="includeIds" type="checkbox" style="min-height:auto">
            Include IDs in load responses
          </label>
          <div class="row">
            <button id="startLoad">Start</button>
            <button id="stopLoad" class="danger">Stop</button>
            <span class="status"><span id="loadDot" class="dot"></span><span id="loadStatus">idle</span></span>
          </div>
          <footer id="loadHint">Recent window sets the <code>/profiles/recent?window=</code> lookup and the chart history horizon. The shown latency is measured from real implementation executions.</footer>
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
        <h2>p95 latency trend</h2>
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
      activeScenario: "lookup",
      activeLanguage: "go",
      activeAlgorithm: "slice_scan",
      activeDataStructure: "default",
      activeImplementation: "lookup:go:slice_scan:default",
      selectedScenario: "lookup",
      selectedLanguage: "go",
      selectedAlgorithm: "slice_scan",
      selectedDataStructure: "default",
      focusMode: "implementation",
      scenarios: [],
      languages: [],
      implementations: [],
      algorithms: [],
      rpsHistory: [],
      latencyHistory: [],
      chartWindowSeconds: 300,
      loadRunning: false,
      rateUpdateTimer: 0
    };

    const $ = (id) => document.getElementById(id);
    const fmt = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });
    const axisFmt = new Intl.NumberFormat(undefined, { maximumFractionDigits: 2 });
    const numericFields = {
      rate: { label: "Rate", min: 1, max: 10000, unit: "requests/sec" },
      window: { label: "Recent window", min: 1, max: 86400, unit: "seconds" },
      duration: { label: "Duration", min: 1, max: 600, unit: "seconds" }
    };

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
      state.activeScenario = data.activeScenario;
      state.activeLanguage = data.activeLanguage;
      state.activeAlgorithm = data.activeAlgorithm;
      state.activeDataStructure = data.activeDataStructure || "default";
      state.activeImplementation = data.activeImplementation;
      state.scenarios = data.scenarios || [];
      state.languages = data.languages || [];
      state.implementations = data.implementations || data.algorithms || [];
      state.algorithms = data.algorithms || state.implementations;
      state.selectedScenario = data.activeScenario;
      state.selectedLanguage = data.activeLanguage;
      state.selectedAlgorithm = data.activeAlgorithm;
      state.selectedDataStructure = state.activeDataStructure;
      $("profileCount").textContent = fmt.format(data.profileCount) + " profiles";
      renderScenarioControls();
      renderScenarioRequestControls(false);
      renderLanguageControls();
      renderFocusControls();
      renderAlgorithmControls();
      renderLoad(data.load);
    }

    function currentScenario() {
      return state.scenarios.find((scenario) => scenario.name === state.selectedScenario) || state.scenarios[0] || {
        name: "lookup",
        label: "Lookup and indexing",
        description: "Find profiles updated within a recent time window.",
        businessCase: "Example: power an operations dashboard that asks which accounts changed in the last 5 minutes.",
        endpoint: "/profiles/recent",
        defaultAlgorithm: "slice_scan",
        defaultDataStructure: "default",
        axes: {
          algorithms: [{ name: "slice_scan", label: "Slice scan", description: "Scan profiles in order." }],
          dataStructures: [{ name: "default", label: "Default backing structure", description: "Default backing structure." }]
        },
        implementations: state.implementations,
        algorithms: state.algorithms,
        request: { label: "Recent window seconds", unit: "seconds", min: 1, max: 86400, default: 300, help: "Allowed range: 1 to 86,400 seconds." }
      };
    }

    function scenarioImplementations() {
      const scenario = currentScenario();
      return scenario.implementations || scenario.algorithms || state.implementations || state.algorithms || [];
    }

    function scenarioAlgorithms() {
      const scenario = currentScenario();
      const axis = scenario.axes && scenario.axes.algorithms;
      if (axis && axis.length) return axis;
      return uniqueAxisOptions("algorithm");
    }

    function scenarioDataStructures() {
      const scenario = currentScenario();
      const axis = scenario.axes && scenario.axes.dataStructures;
      if (axis && axis.length) return axis;
      return uniqueAxisOptions("dataStructure");
    }

    function uniqueAxisOptions(field) {
      const seen = new Set();
      const options = [];
      for (const implementation of scenarioImplementations()) {
        const name = implementation[field];
        if (!name || seen.has(name)) continue;
        seen.add(name);
        options.push({
          name,
          label: field === "dataStructure" ? name.replaceAll("_", " ") : implementation.label,
          description: implementation.description || ""
        });
      }
      return options;
    }

    function findImplementation(algorithm = state.selectedAlgorithm, dataStructure = state.selectedDataStructure) {
      return scenarioImplementations().find((implementation) => {
        return implementation.algorithm === algorithm && implementation.dataStructure === dataStructure;
      });
    }

    function implementationsForAlgorithm(algorithm) {
      return scenarioImplementations().filter((implementation) => implementation.algorithm === algorithm);
    }

    function implementationsForDataStructure(dataStructure) {
      return scenarioImplementations().filter((implementation) => implementation.dataStructure === dataStructure);
    }

    function ensureSelectedPair(changed = "") {
      const scenario = currentScenario();
      const implementations = scenarioImplementations();
      if (!implementations.length) return;

      if (!state.selectedAlgorithm) state.selectedAlgorithm = scenario.defaultAlgorithm || implementations[0].algorithm;
      if (!state.selectedDataStructure) state.selectedDataStructure = scenario.defaultDataStructure || implementations[0].dataStructure;
      if (findImplementation()) return;

      let candidate;
      if (changed === "algorithm") {
        candidate = implementations.find((implementation) => {
          return implementation.algorithm === state.selectedAlgorithm && implementation.dataStructure === scenario.defaultDataStructure;
        }) || implementationsForAlgorithm(state.selectedAlgorithm)[0];
      } else if (changed === "dataStructure") {
        candidate = implementations.find((implementation) => {
          return implementation.dataStructure === state.selectedDataStructure && implementation.algorithm === scenario.defaultAlgorithm;
        }) || implementationsForDataStructure(state.selectedDataStructure)[0];
      } else {
        candidate = implementations.find((implementation) => {
          return implementation.algorithm === scenario.defaultAlgorithm && implementation.dataStructure === scenario.defaultDataStructure;
        }) || implementations[0];
      }

      if (candidate) {
        state.selectedAlgorithm = candidate.algorithm;
        state.selectedDataStructure = candidate.dataStructure;
      }
    }

    function visibleImplementations() {
      const implementations = scenarioImplementations();
      if (state.focusMode === "algorithms") {
        return implementations.filter((implementation) => implementation.dataStructure === state.selectedDataStructure);
      }
      if (state.focusMode === "data_structures") {
        return implementations.filter((implementation) => implementation.algorithm === state.selectedAlgorithm);
      }
      return implementations;
    }

    function hasDataStructureAxis() {
      const names = new Set(scenarioImplementations().map((implementation) => implementation.dataStructure));
      return names.size > 1;
    }

    function normalizeFocusMode() {
      if (!hasDataStructureAxis() && state.focusMode !== "implementation") {
        state.focusMode = "implementation";
      }
    }

    function renderScenarioControls() {
      const select = $("scenario");
      select.innerHTML = "";
      for (const scenario of state.scenarios) {
        const option = document.createElement("option");
        option.value = scenario.name;
        option.textContent = scenario.label;
        option.selected = scenario.name === state.selectedScenario;
        select.appendChild(option);
      }
    }

    function renderScenarioDetails() {
      const scenario = currentScenario();
      $("scenarioTitle").textContent = scenario.label || "Scenario";
      $("scenarioDescription").textContent = scenario.description || "";
      $("scenarioBusinessCase").textContent = scenario.businessCase || "Choose a scenario to see the product workflow it models.";
    }

    function renderLanguageControls() {
      const select = $("language");
      select.innerHTML = "";
      for (const language of state.languages) {
        const option = document.createElement("option");
        option.value = language.name;
        option.textContent = language.label + (language.available ? "" : " (unavailable)");
        option.disabled = !language.available;
        option.title = language.error || "";
        option.selected = language.name === state.selectedLanguage;
        select.appendChild(option);
      }
    }

    function renderFocusControls() {
      normalizeFocusMode();
      const hasAxis = hasDataStructureAxis();
      for (const button of document.querySelectorAll("[data-focus]")) {
        button.disabled = button.dataset.focus !== "implementation" && !hasAxis;
        button.title = button.disabled ? "This scenario only has one backing data structure." : "";
        button.classList.toggle("selected", button.dataset.focus === state.focusMode);
      }
    }

    function renderAlgorithmControls() {
      normalizeFocusMode();
      const algorithms = scenarioAlgorithms();
      const dataStructures = scenarioDataStructures();
      state.implementations = scenarioImplementations();
      state.algorithms = state.implementations;
      ensureSelectedPair();

      const select = $("algorithm");
      select.innerHTML = "";
      for (const algo of algorithms) {
        const option = document.createElement("option");
        option.value = algo.name;
        option.textContent = algo.label;
        option.title = algo.description || "";
        option.disabled = implementationsForAlgorithm(algo.name).length === 0 || (state.focusMode === "algorithms" && !findImplementation(algo.name, state.selectedDataStructure));
        option.selected = algo.name === state.selectedAlgorithm;
        select.appendChild(option);
      }

      const dataStructureSelect = $("dataStructure");
      dataStructureSelect.innerHTML = "";
      for (const dataStructure of dataStructures) {
        const option = document.createElement("option");
        option.value = dataStructure.name;
        option.textContent = dataStructure.label;
        option.title = dataStructure.description || "";
        option.disabled = !findImplementation(state.selectedAlgorithm, dataStructure.name);
        option.selected = dataStructure.name === state.selectedDataStructure;
        dataStructureSelect.appendChild(option);
      }
      dataStructureSelect.disabled = !hasDataStructureAxis();
      renderAlgorithmList();
    }

    function renderAlgorithmList() {
      const list = $("algorithmList");
      const existing = list.querySelectorAll(".algorithm");
      const implementations = visibleImplementations();
      const listKey = [state.selectedScenario, state.focusMode, state.selectedAlgorithm, state.selectedDataStructure].join("|");
      if (existing.length === implementations.length && list.dataset.key === listKey) {
        updateAlgorithmCards();
        return;
      }

      list.innerHTML = "";
      list.dataset.key = listKey;
      for (const implementation of implementations) {
        const div = document.createElement("div");
        div.className = "algorithm";
        div.dataset.implementation = implementation.name;
        div.dataset.algorithm = implementation.algorithm;
        div.dataset.dataStructure = implementation.dataStructure;
        div.innerHTML = [
          "<div class='algorithm-head'><strong></strong><span class='badge empty'></span></div>",
          "<div class='implementation-meta'><span class='badge'></span><span class='badge'></span></div>",
          "<span class='complexity'></span>",
          "<span class='muted'></span>",
          "<div class='code-expander'><div class='code-expander-inner'><pre><code></code></pre></div></div>"
        ].join("");
        div.querySelector("strong").textContent = implementation.label;
        const meta = div.querySelectorAll(".implementation-meta .badge");
        meta[0].textContent = implementation.algorithm.replaceAll("_", " ");
        meta[1].textContent = implementation.dataStructure.replaceAll("_", " ");
        const complexityCode = document.createElement("code");
        complexityCode.textContent = implementation.complexity;
        div.querySelector(".complexity").replaceChildren(complexityCode);
        div.querySelector(".muted").textContent = implementation.description;
        list.appendChild(div);
      }
      requestAnimationFrame(updateAlgorithmCards);
    }

    function updateAlgorithmCards() {
      for (const card of $("algorithmList").querySelectorAll(".algorithm")) {
        const isSelected = card.dataset.algorithm === state.selectedAlgorithm && card.dataset.dataStructure === state.selectedDataStructure;
        const isRunning = state.selectedScenario === state.activeScenario && card.dataset.algorithm === state.activeAlgorithm && card.dataset.dataStructure === state.activeDataStructure && state.selectedLanguage === state.activeLanguage;
        card.classList.toggle("selected", isSelected);

        const implementation = scenarioImplementations().find((candidate) => candidate.name === card.dataset.implementation) || {};
        const codes = implementation.codeByLanguage || {};
        card.querySelector("pre code").textContent = codes[state.selectedLanguage] || implementation.code || "Source snippet unavailable.";

        const badge = card.querySelector(".badge");
        badge.textContent = isRunning ? "Running" : "";
        badge.classList.toggle("empty", !isRunning);
      }
    }

    function renderLoad(load) {
      const running = Boolean(load.running);
      state.loadRunning = running;
      $("loadDot").className = "dot" + (running ? " running" : "");
      const timed = Number(load.durationSeconds || 0) > 0;
      state.chartWindowSeconds = state.activeScenario === "lookup" ? normalizeWindowSeconds(load.windowSeconds || $("window").value) : 300;
      if (load.rate && document.activeElement !== $("rate") && !hasFieldError("rate")) {
        $("rate").value = load.rate;
      }
      if (running) {
        setTimedRunOpen(timed);
        if (timed) $("duration").value = load.durationSeconds;
        if (load.windowSeconds) $("window").value = load.windowSeconds;
        if (load.query && document.activeElement !== $("query")) $("query").value = load.query;
      }
      $("loadStatus").textContent = running
        ? "running " + (timed ? "for " + load.durationSeconds + "s" : "until stopped") + ", " + load.completed + " complete, " + load.inFlight + " in flight"
        : "idle, " + (load.completed || 0) + " complete";
      $("startLoad").disabled = running;
      $("stopLoad").disabled = !running;
    }

    function validateNumberInput(id, options = {}) {
      const show = options.show !== false;
      const input = $(id);
      const config = numericFields[id];
      const raw = input.value.trim();
      let value = Number(raw);
      let message = "";

      if (raw === "") {
        message = config.label + " is required.";
      } else if (!Number.isFinite(value)) {
        message = config.label + " must be a number.";
      } else if (!Number.isInteger(value)) {
        message = config.label + " must be a whole number.";
      } else if (value < config.min) {
        message = config.label + " must be at least " + fmt.format(config.min) + " " + config.unit + ".";
      } else if (value > config.max) {
        message = config.label + " must be at most " + fmt.format(config.max) + " " + config.unit + ".";
      }

      if (show) setFieldValidity(id, message);
      return { ok: message === "", value, message };
    }

    function validateLoadForm() {
      const rate = validateNumberInput("rate");
      const windowValue = validateNumberInput("window");
      const duration = $("durationOption").classList.contains("open")
        ? validateNumberInput("duration")
        : { ok: true, value: 0, message: "" };
      const invalid = [rate, windowValue, duration].find((result) => !result.ok);
      if (invalid) {
        setLoadFormMessage(invalid.message, true);
        return null;
      }
      setLoadFormMessage("");
      return {
        rate: rate.value,
        windowSeconds: windowValue.value,
        durationSeconds: duration.value,
        query: $("query").value.trim()
      };
    }

    function renderScenarioRequestControls(resetValue = true) {
      const scenario = currentScenario();
      renderScenarioDetails();
      const request = scenario.request || {};
      numericFields.window = {
        label: request.label || "Request value",
        min: Number(request.min || 1),
        max: Number(request.max || 86400),
        unit: request.unit || "units"
      };
      $("requestValueLabel").textContent = numericFields.window.label;
      $("window").min = numericFields.window.min;
      $("window").max = numericFields.window.max;
      if (resetValue || !$("window").value) $("window").value = request.default || numericFields.window.min;
      $("windowHint").textContent = request.help || fieldRangeMessage("window");
      clearFieldValidity("window");

      const hasQuery = Boolean(request.queryLabel);
      $("queryField").hidden = !hasQuery;
      if (hasQuery) {
        $("queryLabel").textContent = request.queryLabel;
        if (resetValue || !$("query").value) $("query").value = request.queryDefault || "";
        $("queryHint").textContent = request.queryHelp || "Search query.";
      }

      const endpoint = scenario.endpoint || "/profiles/recent";
      const endpointCode = document.createElement("code");
      endpointCode.textContent = endpoint;
      $("loadHint").replaceChildren(
        "Load calls ",
        endpointCode,
        " for the selected scenario. The shown latency is measured from real implementation executions."
      );
    }

    function setFieldValidity(id, message) {
      const input = $(id);
      const field = $(id + "Field");
      const hint = $(id + "Hint");
      field.classList.toggle("invalid", Boolean(message));
      input.setAttribute("aria-invalid", String(Boolean(message)));
      hint.textContent = message || fieldRangeMessage(id);
    }

    function clearFieldValidity(id) {
      setFieldValidity(id, "");
    }

    function hasFieldError(id) {
      return $(id).getAttribute("aria-invalid") === "true";
    }

    function anyFieldErrors() {
      return ["rate", "window", "duration"].some((id) => hasFieldError(id));
    }

    function fieldRangeMessage(id) {
      const config = numericFields[id];
      return "Allowed range: " + fmt.format(config.min) + " to " + fmt.format(config.max) + " " + config.unit + ".";
    }

    function setLoadFormMessage(message, isError = false) {
      const element = $("loadFormMessage");
      element.textContent = message;
      element.classList.toggle("error", isError);
    }

    function handleNumericInput(event) {
      const result = validateNumberInput(event.target.id);
      if (result.ok && !anyFieldErrors()) setLoadFormMessage("");
      if (event.target.id === "rate") scheduleRateUpdate();
    }

    async function pollStats() {
      const data = await api("/api/stats");
      state.activeScenario = data.activeScenario;
      state.activeLanguage = data.activeLanguage;
      state.activeAlgorithm = data.activeAlgorithm;
      state.activeDataStructure = data.activeDataStructure || "default";
      state.activeImplementation = data.activeImplementation;
      const active = data.algorithms[data.activeImplementation] || {};
      $("rps").textContent = fmt.format(active.recentRps || 0) + " rps";
      $("p95").textContent = fmt.format(active.p95Ms || 0) + " ms";
      $("p99").textContent = fmt.format(active.p99Ms || 0) + " ms";
      renderRuntimeStats(data.runtime);
      renderLoad(data.load);

      const now = Date.now();
      state.rpsHistory = appendHistory(state.rpsHistory, { at: now, value: active.recentRps || 0 }, state.chartWindowSeconds);
      state.latencyHistory = appendHistory(state.latencyHistory, { at: now, value: active.p95Ms || 0 }, state.chartWindowSeconds);

      drawLineChart($("latencyChart"), state.latencyHistory, "ms");
      drawLineChart($("rpsChart"), state.rpsHistory, "rps");
    }

    function appendHistory(history, point, windowSeconds) {
      const cutoff = point.at - windowSeconds * 1000;
      return history.concat(point).filter((entry) => entry.at >= cutoff);
    }

    function normalizeWindowSeconds(value) {
      const seconds = Number(value);
      if (!Number.isFinite(seconds) || seconds <= 0) return 300;
      return Math.min(Math.max(Math.round(seconds), 1), 24 * 60 * 60);
    }

    function setTimedRunOpen(open) {
      $("durationOption").classList.toggle("open", open);
      $("durationToggle").setAttribute("aria-expanded", String(open));
      $("durationPanel").setAttribute("aria-hidden", String(!open));
      $("duration").disabled = !open;
      if (open) validateNumberInput("duration");
      else {
        clearFieldValidity("duration");
        if (!anyFieldErrors()) setLoadFormMessage("");
      }
    }

    function scheduleRateUpdate() {
      if (!state.loadRunning) return;
      window.clearTimeout(state.rateUpdateTimer);

      const result = validateNumberInput("rate");
      if (!result.ok) {
        setLoadFormMessage(result.message, true);
        return;
      }
      setLoadFormMessage("");

      state.rateUpdateTimer = window.setTimeout(() => updateLoadRate(result.value), 250);
    }

    async function updateLoadRate(rate) {
      if (!state.loadRunning) return;
      const latest = validateNumberInput("rate");
      if (!latest.ok || latest.value !== rate) return;
      try {
        const load = await api("/api/load/rate", {
          method: "POST",
          body: JSON.stringify({ rate })
        });
        clearFieldValidity("rate");
        setLoadFormMessage("Target rate updated to " + fmt.format(rate) + " requests/sec.");
        renderLoad(load);
      } catch (err) {
        console.error(err);
        setFieldValidity("rate", err.message);
        setLoadFormMessage(err.message, true);
        await pollStats();
      }
    }

    function renderRuntimeStats(runtime) {
      const value = $("runtime");
      const memory = document.createElement("span");
      memory.className = "runtime-part";
      memory.textContent = fmt.format(runtime.heapMb || 0) + " MB" + (runtime.cpuPercent >= 0 ? "," : "");

      value.replaceChildren(memory);
      if (runtime.cpuPercent >= 0) {
        const cpu = document.createElement("span");
        cpu.className = "runtime-part";
        cpu.textContent = fmt.format(runtime.cpuPercent) + "% CPU";
        value.appendChild(cpu);
      }
    }

    function drawLineChart(canvas, values, suffix) {
      const ctx = canvas.getContext("2d");
      const w = canvas.width;
      const h = canvas.height;
      const points = normalizeSeries(values);
      const chart = { top: 18, right: 14, bottom: 34, left: 56 };
      const plotW = Math.max(1, w - chart.left - chart.right);
      const plotH = Math.max(1, h - chart.top - chart.bottom);
      const bottom = h - chart.bottom;
      const maxValue = Math.max(...points.map((point) => point.value), 0);
      const yTicks = yAxisTicks(maxValue);
      const yMax = yTicks[yTicks.length - 1] || 1;

      ctx.clearRect(0, 0, w, h);
      ctx.fillStyle = "#fff";
      ctx.fillRect(0, 0, w, h);
      ctx.font = "11px system-ui, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif";
      ctx.textBaseline = "middle";
      ctx.strokeStyle = "#d8dee8";
      ctx.lineWidth = 1;

      for (const tick of yTicks) {
        const y = bottom - (tick / yMax) * plotH;
        ctx.beginPath();
        ctx.moveTo(chart.left, y);
        ctx.lineTo(w - chart.right, y);
        ctx.stroke();
        ctx.fillStyle = "#5f6b7a";
        ctx.textAlign = "right";
        ctx.fillText(formatAxisNumber(tick), chart.left - 8, y);
      }

      const xIndexes = xTickIndexes(points.length, Math.max(2, Math.min(5, Math.floor(plotW / 110) + 1)));
      for (let i = 0; i < xIndexes.length; i++) {
        const index = xIndexes[i];
        const x = xForIndex(index, points.length, chart.left, plotW);
        ctx.strokeStyle = "#eef2f7";
        ctx.beginPath();
        ctx.moveTo(x, chart.top);
        ctx.lineTo(x, bottom);
        ctx.stroke();
        ctx.strokeStyle = "#d8dee8";
        ctx.beginPath();
        ctx.moveTo(x, bottom);
        ctx.lineTo(x, bottom + 4);
        ctx.stroke();
        ctx.fillStyle = "#5f6b7a";
        ctx.textAlign = i === 0 ? "left" : i === xIndexes.length - 1 ? "right" : "center";
        ctx.textBaseline = "top";
        ctx.fillText(formatXTick(points[index]), x, bottom + 8);
      }

      ctx.strokeStyle = "#5f6b7a";
      ctx.beginPath();
      ctx.moveTo(chart.left, chart.top);
      ctx.lineTo(chart.left, bottom);
      ctx.lineTo(w - chart.right, bottom);
      ctx.stroke();

      if (!points.length) {
        ctx.fillStyle = "#5f6b7a";
        ctx.textAlign = "left";
        ctx.textBaseline = "middle";
        ctx.fillText("No samples yet", chart.left + 10, chart.top + 12);
        return;
      }

      ctx.strokeStyle = "#1666c1";
      ctx.lineWidth = 2;
      ctx.beginPath();
      points.forEach((point, i) => {
        const x = xForIndex(i, points.length, chart.left, plotW);
        const y = bottom - (point.value / yMax) * plotH;
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      });
      ctx.stroke();
      if (points.length === 1) {
        const x = xForIndex(0, points.length, chart.left, plotW);
        const y = bottom - (points[0].value / yMax) * plotH;
        ctx.fillStyle = "#1666c1";
        ctx.beginPath();
        ctx.arc(x, y, 3, 0, Math.PI * 2);
        ctx.fill();
      }
      ctx.fillStyle = "#17202a";
      ctx.textAlign = "right";
      ctx.textBaseline = "top";
      ctx.fillText("max " + formatAxisNumber(maxValue) + " " + suffix, w - chart.right, 6);
    }

    function normalizeSeries(series) {
      return series.map((point, index) => {
        if (typeof point === "number") {
          return { value: Number.isFinite(point) ? point : 0, at: NaN, index };
        }
        const value = Number(point.value);
        const at = typeof point.at === "number" ? point.at : Date.parse(point.at);
        return {
          value: Number.isFinite(value) ? value : 0,
          at: Number.isFinite(at) ? at : NaN,
          index
        };
      });
    }

    function yAxisTicks(maxValue) {
      const safeMax = Number.isFinite(maxValue) && maxValue > 0 ? maxValue : 1;
      const step = niceStep(safeMax / 4);
      const maxTick = Math.max(step, Math.ceil(safeMax / step) * step);
      const ticks = [];
      for (let value = 0; value <= maxTick + step / 2; value += step) {
        ticks.push(roundTick(value));
      }
      return ticks;
    }

    function niceStep(value) {
      const magnitude = Math.pow(10, Math.floor(Math.log10(value)));
      const fraction = value / magnitude;
      const niceFraction = fraction <= 1 ? 1 : fraction <= 2 ? 2 : fraction <= 5 ? 5 : 10;
      return niceFraction * magnitude;
    }

    function roundTick(value) {
      if (value === 0) return 0;
      const precision = Math.max(0, 2 - Math.floor(Math.log10(Math.abs(value))));
      return Number(value.toFixed(precision));
    }

    function xTickIndexes(length, target) {
      if (length <= 0) return [];
      if (length === 1) return [0];
      const count = Math.min(length, target);
      const indexes = [];
      for (let i = 0; i < count; i++) {
        const index = Math.round((i * (length - 1)) / (count - 1));
        if (!indexes.includes(index)) indexes.push(index);
      }
      return indexes;
    }

    function xForIndex(index, length, left, width) {
      if (length <= 1) return left + width / 2;
      return left + (index / (length - 1)) * width;
    }

    function formatAxisNumber(value) {
      if (!Number.isFinite(value)) return "0";
      const abs = Math.abs(value);
      if (abs > 0 && abs < 0.01) return value.toExponential(1);
      if (abs > 0 && abs < 1) return value.toFixed(2).replace(/\.?0+$/, "");
      return axisFmt.format(value);
    }

    function formatXTick(point) {
      if (point && Number.isFinite(point.at)) {
        const date = new Date(point.at);
        return [date.getHours(), date.getMinutes(), date.getSeconds()]
          .map((part) => String(part).padStart(2, "0"))
          .join(":");
      }
      return String((point && point.index + 1) || 1);
    }

    $("applyAlgorithm").addEventListener("click", async () => {
      ensureSelectedPair();
      await api("/api/implementation", {
        method: "POST",
        body: JSON.stringify({
          scenario: $("scenario").value,
          language: $("language").value,
          algorithm: $("algorithm").value,
          dataStructure: $("dataStructure").value
        })
      });
      await loadState();
    });

    $("scenario").addEventListener("change", () => {
      state.selectedScenario = $("scenario").value;
      const scenario = currentScenario();
      const implementations = scenarioImplementations();
      const defaultImplementation = implementations.find((implementation) => {
        return implementation.algorithm === scenario.defaultAlgorithm && implementation.dataStructure === scenario.defaultDataStructure;
      }) || implementations[0] || {};
      state.selectedAlgorithm = defaultImplementation.algorithm || scenario.defaultAlgorithm || "";
      state.selectedDataStructure = defaultImplementation.dataStructure || scenario.defaultDataStructure || "default";
      renderScenarioRequestControls(true);
      renderAlgorithmControls();
      renderFocusControls();
    });

    $("language").addEventListener("change", () => {
      state.selectedLanguage = $("language").value;
      updateAlgorithmCards();
    });

    $("algorithm").addEventListener("change", () => {
      state.selectedAlgorithm = $("algorithm").value;
      ensureSelectedPair("algorithm");
      renderAlgorithmControls();
    });

    $("dataStructure").addEventListener("change", () => {
      state.selectedDataStructure = $("dataStructure").value;
      ensureSelectedPair("dataStructure");
      renderAlgorithmControls();
    });

    for (const button of document.querySelectorAll("[data-focus]")) {
      button.addEventListener("click", () => {
        state.focusMode = button.dataset.focus;
        renderFocusControls();
        renderAlgorithmControls();
      });
    }

    $("durationToggle").addEventListener("click", () => {
      setTimedRunOpen(!$("durationOption").classList.contains("open"));
    });

    for (const id of ["rate", "window", "duration"]) {
      $(id).addEventListener("input", handleNumericInput);
      $(id).addEventListener("change", handleNumericInput);
    }

    $("startLoad").addEventListener("click", async () => {
      const form = validateLoadForm();
      if (!form) return;
      try {
        await api("/api/load/start", {
          method: "POST",
          body: JSON.stringify({
            rate: form.rate,
            durationSeconds: form.durationSeconds,
            windowSeconds: form.windowSeconds,
            includeIds: $("includeIds").checked,
            query: form.query
          })
        });
        setLoadFormMessage("");
        await pollStats();
      } catch (err) {
        console.error(err);
        setLoadFormMessage(err.message, true);
      }
    });

    $("stopLoad").addEventListener("click", async () => {
      window.clearTimeout(state.rateUpdateTimer);
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
