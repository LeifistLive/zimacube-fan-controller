package webui

// IndexHTML, StyleCSS and ScriptJS are served from separate routes so that the
// Content-Security-Policy no longer needs 'unsafe-inline'.
const IndexHTML = `<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>ZimaCube Fan Controller</title>
<link rel="stylesheet" href="/app.css">
</head>
<body>
<header class="topbar">
<div class="brand">
<h1>ZimaCube <span class="brand-accent">Fan Controller</span></h1>
<div class="sub">Live-Status &middot; Profile &middot; Verlauf <span id="version" class="muted"></span></div>
</div>
<div class="controls-row">
<input id="token" class="input token-input" type="password" placeholder="API-Token" autocomplete="off">
<button id="refresh" class="btn" type="button">Aktualisieren</button>
</div>
</header>

<div id="banner" class="banner" hidden></div>

<main>

<section class="stat-grid">
<div class="stat-card"><div class="stat-label">Modus</div><div class="stat-value" id="mode">-</div></div>
<div class="stat-card"><div class="stat-label">Profil</div><div class="stat-value" id="profile">-</div></div>
<div class="stat-card">
<div class="stat-label">Lüfter</div>
<div class="stat-value" id="fan">-</div>
<div class="meter"><div class="meter-fill" id="fanMeter"></div></div>
</div>
<div class="stat-card"><div class="stat-label">Max. HDD</div><div class="stat-value" id="temp">-</div></div>
<div class="stat-card"><div class="stat-label">Array</div><div class="stat-value" id="array">-</div></div>
<div class="stat-card">
<div class="stat-label">Controller</div>
<div class="stat-value"><span class="dot" id="controllerDot"></span><span id="controller">-</span></div>
</div>
</section>

<section class="panel">
<div class="panel-head"><h2>Grund</h2></div>
<div id="reason" class="reason">-</div>
</section>

<section class="panel">
<div class="panel-head"><h2>Steuerung</h2></div>
<div class="controls-row">
<div class="segmented" id="profiles"></div>
</div>
<div class="controls-row">
<button type="button" class="btn" data-post="/api/mode/auto">Automatik</button>
<button type="button" class="btn btn-danger" data-post="/api/mode/emergency">Notfall</button>
<span class="spacer"></span>
<input id="percent" class="input num-input" type="number" min="1" max="100" value="75">
<button id="setManual" class="btn btn-primary" type="button">Manuell setzen</button>
</div>
<div class="controls-row">
<span class="muted small">Test</span>
<button type="button" class="btn btn-ghost" data-test="25">25 %</button>
<button type="button" class="btn btn-ghost" data-test="50">50 %</button>
<button type="button" class="btn btn-ghost" data-test="75">75 %</button>
<button type="button" class="btn btn-ghost" data-test="100">100 %</button>
</div>
<pre id="message" class="message"></pre>
</section>

<section class="chart-grid">
<div class="panel">
<div class="panel-head"><h2>Temperatur</h2><div class="muted small">Maximale HDD-Temperatur</div></div>
<canvas id="chartTemp" height="220"></canvas>
</div>
<div class="panel">
<div class="panel-head"><h2>Lüfterdrehzahl</h2><div class="muted small">Geschriebener Prozentwert</div></div>
<canvas id="chartFan" height="220"></canvas>
</div>
</section>

<section class="panel">
<div class="panel-head panel-head-row">
<h2>Ereignisse</h2>
<input id="eventFilter" class="input filter-input" type="text" placeholder="Filter...">
</div>
<div class="table-wrap">
<table>
<thead><tr><th>Zeit</th><th>Typ</th><th>Meldung</th></tr></thead>
<tbody id="events"></tbody>
</table>
</div>
</section>

<section class="panel">
<div class="panel-head"><h2>Konfiguration</h2></div>
<textarea id="config" class="input config-editor" spellcheck="false"></textarea>
<div class="controls-row mt">
<button id="saveConfig" class="btn btn-primary" type="button">Speichern</button>
<button id="reloadConfig" class="btn" type="button">Verwerfen und neu laden</button>
</div>
</section>

</main>

<script src="/app.js"></script>
</body>
</html>`

const StyleCSS = `:root{
color-scheme:dark;
--bg:#09090b;
--card:#121215;
--border:#1f1f24;
--border-soft:#2a2a31;
--text:#f4f4f5;
--text-muted:#a1a1aa;
--text-dim:#71717a;
--green:#22c55e;
--yellow:#eab308;
--red:#ef4444;
--blue:#3b82f6;
--orange:#f59e0b;
--radius-card:14px;
--radius-control:8px;
}
*{box-sizing:border-box}
html,body{background:var(--bg)}
body{
margin:0;color:var(--text);
font-family:Inter,-apple-system,"Segoe UI",system-ui,sans-serif;
font-size:14px;line-height:1.45;
-webkit-font-smoothing:antialiased;
}
h1,h2{margin:0;font-weight:650;letter-spacing:-.01em}
h1{font-size:1.3rem}
.brand-accent{color:var(--text-muted);font-weight:500}
h2{font-size:.95rem}
.muted{color:var(--text-muted)}
.small{font-size:.78rem}

.topbar{
display:flex;justify-content:space-between;align-items:center;gap:16px;flex-wrap:wrap;
padding:18px 28px;border-bottom:1px solid var(--border);
position:sticky;top:0;background:rgba(9,9,11,.92);backdrop-filter:blur(6px);z-index:10;
}
.sub{color:var(--text-muted);font-size:.8rem;margin-top:4px}

main{max-width:1360px;margin:0 auto;padding:22px 28px 60px;display:flex;flex-direction:column;gap:20px}

.stat-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(170px,1fr));gap:12px}
.stat-card{
background:var(--card);border:1px solid var(--border);border-radius:var(--radius-card);
padding:16px 18px;
}
.stat-label{color:var(--text-muted);font-size:.78rem;text-transform:uppercase;letter-spacing:.04em}
.stat-value{font-size:1.5rem;font-weight:650;margin-top:6px;letter-spacing:-.01em}
.stat-value.ok{color:var(--green)}
.stat-value.warn{color:var(--yellow)}
.stat-value.bad{color:var(--red)}

.dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:8px;vertical-align:middle;background:var(--text-dim)}
.dot-ok{background:var(--green);box-shadow:0 0 0 3px rgba(34,197,94,.16)}
.dot-bad{background:var(--red);box-shadow:0 0 0 3px rgba(239,68,68,.16)}

.meter{margin-top:10px;height:6px;border-radius:999px;background:var(--border);overflow:hidden}
.meter-fill{height:100%;width:0;border-radius:999px;background:var(--blue);transition:width .3s ease}

.panel{background:var(--card);border:1px solid var(--border);border-radius:var(--radius-card);padding:18px 20px}
.panel-head{margin-bottom:12px}
.panel-head-row{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap}
.reason{color:var(--text-muted)}

.banner{
margin:0 28px;padding:12px 16px;border-radius:10px;
border:1px solid rgba(239,68,68,.35);background:rgba(239,68,68,.1);color:#fca5a5;
}

.controls-row{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:10px}
.controls-row:first-child{margin-top:0}
.controls-row.mt{margin-top:14px}
.spacer{flex:1 1 auto}

.segmented{display:flex;gap:6px;flex-wrap:wrap}
.seg-btn{
border-radius:var(--radius-control);border:1px solid var(--border-soft);background:transparent;
color:var(--text-muted);padding:8px 14px;cursor:pointer;font-size:.85rem;font-weight:550;
}
.seg-btn:hover{border-color:var(--text-dim);color:var(--text)}
.seg-btn.active{background:var(--text);color:#09090b;border-color:var(--text)}

.input{
border-radius:var(--radius-control);border:1px solid var(--border-soft);background:#0d0d10;
color:var(--text);padding:9px 12px;font:inherit;
}
.input:focus{outline:2px solid var(--blue);outline-offset:-1px}
.token-input{min-width:190px}
.num-input{width:90px}
.filter-input{width:200px}

.btn{
border-radius:var(--radius-control);border:1px solid var(--border-soft);background:#141417;
color:var(--text);padding:9px 14px;font:inherit;font-weight:550;cursor:pointer;
}
.btn:hover{background:#1b1b20;border-color:var(--text-dim)}
.btn-primary{background:var(--text);color:#09090b;border-color:var(--text)}
.btn-primary:hover{background:#d4d4d8;border-color:#d4d4d8}
.btn-danger{color:#fca5a5;border-color:rgba(239,68,68,.35)}
.btn-danger:hover{background:rgba(239,68,68,.12);border-color:var(--red)}
.btn-ghost{background:transparent;color:var(--text-muted)}
.btn-ghost:hover{background:#16161a;color:var(--text)}

.message{white-space:pre-wrap;overflow:auto;color:var(--text-muted);margin:12px 0 0;font-size:.85rem}

.chart-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}
@media (max-width:860px){.chart-grid{grid-template-columns:1fr}}
canvas{width:100%;display:block}

.table-wrap{overflow-x:auto}
table{width:100%;border-collapse:collapse;font-size:.85rem}
th{
text-align:left;color:var(--text-muted);font-weight:600;font-size:.78rem;
text-transform:uppercase;letter-spacing:.03em;padding:0 10px 10px;border-bottom:1px solid var(--border);
}
td{padding:10px;border-bottom:1px solid var(--border);color:var(--text)}
tbody tr:hover{background:#151519}
tbody tr:last-child td{border-bottom:none}

.config-editor{width:100%;min-height:220px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.82rem;resize:vertical}

@media (max-width:640px){
.topbar{padding:14px 16px}
main{padding:16px 16px 40px}
.banner{margin:0 16px}
}`

const ScriptJS = `"use strict";
var byId = function (id) { return document.getElementById(id); };
var tokenKey = "zimafan-token";
var configDirty = false;
var lastEvents = [];
var lastProfiles = [];
var activeProfileName = "";

var themeColors = getComputedStyle(document.documentElement);
var colorFan = themeColors.getPropertyValue("--blue").trim() || "#3b82f6";
var colorTemp = themeColors.getPropertyValue("--orange").trim() || "#f59e0b";

function token() { return byId("token").value.trim(); }

function headers(withBody) {
  var result = {};
  var value = token();
  if (value) { result["X-API-Token"] = value; }
  if (withBody) { result["Content-Type"] = "application/json"; }
  return result;
}

function say(text) { byId("message").textContent = text; }

async function getJSON(url) {
  var response = await fetch(url, { cache: "no-store" });
  var text = await response.text();
  if (!response.ok) { throw new Error(response.status + ": " + text); }
  return JSON.parse(text);
}

async function post(path, body) {
  try {
    var response = await fetch(path, {
      method: "POST",
      headers: headers(Boolean(body)),
      body: body ? JSON.stringify(body) : undefined
    });
    say(await response.text());
    if (response.status === 401) { say("Nicht autorisiert: API-Token prüfen."); }
    setTimeout(refreshAll, 400);
    return response.ok;
  } catch (error) {
    say(String(error));
    return false;
  }
}

function setBanner(status) {
  var banner = byId("banner");
  var problems = [];
  if (!status.controller_online) { problems.push("I2C-Controller nicht erreichbar"); }
  if (!status.temperature_valid) { problems.push("HDD-Temperatur nicht lesbar, Sicherheitsdrehzahl aktiv"); }
  if (status.last_error) { problems.push(status.last_error); }
  if (problems.length === 0) { banner.hidden = true; banner.textContent = ""; return; }
  banner.hidden = false;
  banner.textContent = problems.join(" | ");
}

function labelForMode(mode) {
  switch (mode) {
    case "automatic": return "Automatik";
    case "manual": return "Manuell";
    case "emergency": return "Notfall";
    case "array-boost": return "Array-Boost";
    case "failsafe": return "Sicherheitsmodus";
    default: return mode || "-";
  }
}

function classForMode(mode) {
  switch (mode) {
    case "emergency": case "failsafe": return "bad";
    case "array-boost": return "warn";
    case "automatic": return "ok";
    default: return "";
  }
}

async function refreshStatus() {
  var data = await getJSON("/api/status");
  var status = data.status;
  byId("version").textContent = status.version ? "v" + status.version : "";
  byId("mode").textContent = labelForMode(status.mode);
  byId("mode").className = "stat-value " + classForMode(status.mode);
  byId("profile").textContent = status.active_profile || "-";
  byId("fan").textContent = status.fan_percent + " %";
  byId("fanMeter").style.width = Math.max(0, Math.min(100, status.fan_percent || 0)) + "%";
  byId("temp").textContent = status.temperature_valid ? status.maximum_disk_temperature + " °C" : "unbekannt";
  byId("temp").className = "stat-value " + (status.temperature_valid ? "" : "bad");
  byId("array").textContent = status.array_operation && status.array_operation !== "none" ? status.array_operation : "inaktiv";
  byId("reason").textContent = status.reason || "-";
  byId("controller").textContent = status.controller_online ? "online" : "offline";
  byId("controllerDot").className = "dot " + (status.controller_online ? "dot-ok" : "dot-bad");
  setBanner(status);

  activeProfileName = status.active_profile || "";
  renderProfileButtons();
}

function renderProfileButtons() {
  var container = byId("profiles");
  container.textContent = "";
  lastProfiles.forEach(function (profile) {
    var button = document.createElement("button");
    button.type = "button";
    button.className = "seg-btn" + (profile.id === activeProfileName ? " active" : "");
    button.textContent = profile.label;
    button.addEventListener("click", function () {
      post("/api/profile/" + encodeURIComponent(profile.id));
    });
    container.appendChild(button);
  });
}

async function refreshConfig(force) {
  var config = await getJSON("/api/config");
  if (force || !configDirty) {
    byId("config").value = JSON.stringify(config, null, 2);
    configDirty = false;
  }
  var profiles = config.profiles || {};
  lastProfiles = Object.keys(profiles).map(function (id) {
    return { id: id, label: (profiles[id] && profiles[id].name) || id };
  });
  renderProfileButtons();
}

// Rows are built with DOM nodes instead of innerHTML, because event messages
// contain values from var.ini and from user defined profile names.
function renderEvents() {
  var query = byId("eventFilter").value.trim().toLowerCase();
  var body = byId("events");
  body.textContent = "";
  lastEvents.slice().reverse().forEach(function (event) {
    var haystack = (String(event.type) + " " + String(event.message)).toLowerCase();
    if (query && haystack.indexOf(query) === -1) { return; }
    var tr = document.createElement("tr");
    [new Date(event.time).toLocaleString(), event.type, event.message].forEach(function (cell) {
      var td = document.createElement("td");
      td.textContent = cell === null || cell === undefined ? "" : String(cell);
      tr.appendChild(td);
    });
    body.appendChild(tr);
  });
}

async function refreshEvents() {
  lastEvents = await getJSON("/api/events?limit=80");
  renderEvents();
}

async function refreshHistory() {
  var data = await getJSON("/api/history?limit=288");
  drawArea(byId("chartTemp"), data, "temperature", colorTemp, 20, 55, " °C");
  drawArea(byId("chartFan"), data, "fan_percent", colorFan, 0, 100, " %");
}

function drawArea(canvas, points, valueKey, color, minFloor, maxFloor, suffix) {
  var ratio = window.devicePixelRatio || 1;
  var width = canvas.clientWidth || 520;
  var height = 220;
  canvas.width = Math.floor(width * ratio);
  canvas.height = Math.floor(height * ratio);

  var ctx = canvas.getContext("2d");
  ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
  ctx.clearRect(0, 0, width, height);
  ctx.font = "11px Inter, system-ui, sans-serif";

  if (!points || points.length < 2) {
    ctx.fillStyle = "#71717a";
    ctx.fillText("Noch keine Messpunkte", 12, height / 2);
    return;
  }

  var padLeft = 42, padRight = 12, padTop = 12, padBottom = 20;
  var plotW = width - padLeft - padRight;
  var plotH = height - padTop - padBottom;

  var values = points.map(function (point) { return point[valueKey]; });
  var maxValue = Math.max.apply(null, [maxFloor].concat(values));
  var minValue = Math.min.apply(null, [minFloor].concat(values));
  if (minValue === maxValue) { maxValue = minValue + 1; }
  var span = maxValue - minValue;

  ctx.strokeStyle = "rgba(255,255,255,.07)";
  ctx.lineWidth = 1;
  var gridLines = 4;
  for (var i = 0; i <= gridLines; i++) {
    var y = padTop + i * plotH / gridLines;
    ctx.beginPath();
    ctx.moveTo(padLeft, y);
    ctx.lineTo(width - padRight, y);
    ctx.stroke();
    var value = Math.round(maxValue - i * span / gridLines);
    ctx.fillStyle = "#71717a";
    ctx.fillText(value + suffix, 2, y + 4);
  }

  var px = function (index) { return padLeft + index * plotW / (points.length - 1); };
  var py = function (value) { return padTop + plotH - (value - minValue) * plotH / span; };

  var gradient = ctx.createLinearGradient(0, padTop, 0, padTop + plotH);
  gradient.addColorStop(0, color + "4d");
  gradient.addColorStop(1, color + "03");

  ctx.beginPath();
  ctx.moveTo(px(0), py(points[0][valueKey]));
  points.forEach(function (point, index) { ctx.lineTo(px(index), py(point[valueKey])); });
  ctx.lineTo(px(points.length - 1), padTop + plotH);
  ctx.lineTo(px(0), padTop + plotH);
  ctx.closePath();
  ctx.fillStyle = gradient;
  ctx.fill();

  ctx.beginPath();
  points.forEach(function (point, index) {
    var y = py(point[valueKey]);
    if (index === 0) { ctx.moveTo(px(index), y); } else { ctx.lineTo(px(index), y); }
  });
  ctx.strokeStyle = color;
  ctx.lineWidth = 2;
  ctx.stroke();

  ctx.fillStyle = "#71717a";
  ctx.fillText(new Date(points[0].time).toLocaleString(), padLeft, height - 4);
  var last = new Date(points[points.length - 1].time).toLocaleString();
  ctx.fillText(last, Math.max(padLeft, width - padRight - ctx.measureText(last).width), height - 4);
}

async function saveConfig() {
  var parsed;
  try {
    parsed = JSON.parse(byId("config").value);
  } catch (error) {
    say("Ungueltiges JSON: " + error.message);
    return;
  }
  if (await post("/api/config", parsed)) { configDirty = false; }
}

async function refreshAll() {
  try {
    await Promise.all([refreshStatus(), refreshConfig(false), refreshEvents(), refreshHistory()]);
  } catch (error) {
    say(String(error));
  }
}

function wire() {
  var stored = sessionStorage.getItem(tokenKey);
  if (stored) { byId("token").value = stored; }
  byId("token").addEventListener("change", function () {
    sessionStorage.setItem(tokenKey, token());
  });

  byId("refresh").addEventListener("click", function () { refreshAll(); });
  byId("setManual").addEventListener("click", function () {
    post("/api/fan/" + encodeURIComponent(byId("percent").value));
  });
  byId("saveConfig").addEventListener("click", saveConfig);
  byId("reloadConfig").addEventListener("click", function () { refreshConfig(true); });
  byId("config").addEventListener("input", function () { configDirty = true; });
  byId("eventFilter").addEventListener("input", renderEvents);

  document.querySelectorAll("[data-post]").forEach(function (button) {
    button.addEventListener("click", function () { post(button.dataset.post); });
  });
  document.querySelectorAll("[data-test]").forEach(function (button) {
    button.addEventListener("click", function () {
      post("/api/test/" + encodeURIComponent(button.dataset.test));
    });
  });

  window.addEventListener("resize", function () { refreshHistory(); });

  refreshAll();
  setInterval(refreshStatus, 3000);
  setInterval(refreshHistory, 30000);
  setInterval(refreshEvents, 30000);
}

wire();`
