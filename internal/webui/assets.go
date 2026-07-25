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
<header>
<div>
<h1>ZimaCube Fan Controller</h1>
<div class="sub">Live-Status, Profile, Verlauf und Konfiguration <span id="version" class="muted"></span></div>
</div>
<div class="controls">
<input id="token" class="wide" type="password" placeholder="API-Token (optional)" autocomplete="off">
<button id="refresh" type="button">Aktualisieren</button>
</div>
</header>

<div id="banner" class="banner" hidden></div>

<div class="grid">
<div class="card"><div class="label">Modus</div><div class="value" id="mode">-</div></div>
<div class="card"><div class="label">Profil</div><div class="value" id="profile">-</div></div>
<div class="card"><div class="label">Lüfter</div><div class="value" id="fan">-</div></div>
<div class="card"><div class="label">Max. HDD</div><div class="value" id="temp">-</div></div>
<div class="card"><div class="label">Array</div><div class="value" id="array">-</div></div>
<div class="card"><div class="label">Controller</div><div class="value" id="controller">-</div></div>
</div>

<section class="card">
<div class="label">Grund</div><div id="reason" class="mt-6">-</div>
</section>

<section class="card">
<div class="section-title">Steuerung</div>
<div class="controls">
<input id="percent" type="number" min="1" max="100" value="75">
<button id="setManual" type="button">Manuell setzen</button>
<button type="button" data-post="/api/mode/auto">Automatik</button>
<button type="button" data-post="/api/mode/emergency">Notfall</button>
<button type="button" data-test="25">Test 25 %</button>
<button type="button" data-test="50">Test 50 %</button>
<button type="button" data-test="75">Test 75 %</button>
<button type="button" data-test="100">Test 100 %</button>
</div>
<div class="profile mt-12" id="profiles"></div>
<pre id="message"></pre>
</section>

<section class="card">
<div class="section-title">Verlauf</div>
<canvas id="chart" height="260"></canvas>
<div class="muted mt-8">Temperatur und Lüfter-Prozent der letzten Messpunkte.</div>
</section>

<section class="card">
<div class="section-title">Ereignisse</div>
<table><thead><tr><th>Zeit</th><th>Typ</th><th>Meldung</th></tr></thead><tbody id="events"></tbody></table>
</section>

<section class="card">
<div class="section-title">Konfiguration</div>
<textarea id="config" spellcheck="false"></textarea>
<div class="controls mt-10">
<button id="saveConfig" type="button">Konfiguration speichern</button>
<button id="reloadConfig" type="button">Verwerfen und neu laden</button>
</div>
</section>

<script src="/app.js"></script>
</body>
</html>`

const StyleCSS = `:root{color-scheme:dark;font-family:Inter,system-ui,sans-serif;background:#0e1116;color:#eef2f7}
*{box-sizing:border-box}body{margin:0;max-width:1180px;padding:24px;margin:auto}
header{display:flex;justify-content:space-between;align-items:center;gap:16px;flex-wrap:wrap}
h1{font-size:1.7rem;margin:0}.sub{color:#8e9aab;margin-top:5px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(180px,1fr));gap:12px;margin-top:20px}
.card{background:#171c24;border:1px solid #29313d;border-radius:15px;padding:16px}
.label{color:#8e9aab;font-size:.82rem}.value{font-size:1.55rem;font-weight:750;margin-top:6px}
.ok{color:#67d391}.bad{color:#ff7b72}.warn{color:#ffc76b}
.banner{margin-top:16px;padding:12px 16px;border-radius:12px;border:1px solid #7a4a1c;background:#2a1d10;color:#ffc76b}
.controls{display:flex;gap:10px;flex-wrap:wrap;align-items:center}
button,input,select,textarea{border-radius:10px;border:1px solid #394352;background:#11161d;color:#fff;padding:10px 12px}
button{cursor:pointer}button:hover{background:#232b36}
input[type=number]{width:90px}.wide{min-width:220px}
section{margin-top:16px}.section-title{font-size:1.05rem;font-weight:700;margin-bottom:10px}
canvas{width:100%;height:260px;background:#11161d;border-radius:10px}
table{width:100%;border-collapse:collapse;font-size:.9rem}
td,th{padding:9px;border-bottom:1px solid #2c3440;text-align:left}
pre{white-space:pre-wrap;overflow:auto}
.profile{display:flex;gap:8px;flex-wrap:wrap}.muted{color:#8e9aab}
.mt-6{margin-top:6px}.mt-8{margin-top:8px}.mt-10{margin-top:10px}.mt-12{margin-top:12px}
textarea{width:100%;min-height:220px;font-family:ui-monospace,monospace}`

const ScriptJS = `"use strict";
var byId = function (id) { return document.getElementById(id); };
var tokenKey = "zimafan-token";
var configDirty = false;

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

async function refreshStatus() {
  var data = await getJSON("/api/status");
  var status = data.status;
  byId("version").textContent = status.version ? "v" + status.version : "";
  byId("mode").textContent = status.mode || "-";
  byId("profile").textContent = status.active_profile || "-";
  byId("fan").textContent = status.fan_percent + " %";
  byId("temp").textContent = status.temperature_valid ? status.maximum_disk_temperature + " \u00b0C" : "unbekannt";
  byId("temp").className = "value " + (status.temperature_valid ? "" : "bad");
  byId("array").textContent = status.array_operation || "-";
  byId("reason").textContent = status.reason || "-";
  byId("controller").textContent = status.controller_online ? "online" : "offline";
  byId("controller").className = "value " + (status.controller_online ? "ok" : "bad");
  setBanner(status);
}

async function refreshConfig(force) {
  var config = await getJSON("/api/config");
  if (force || !configDirty) {
    byId("config").value = JSON.stringify(config, null, 2);
    configDirty = false;
  }
  var container = byId("profiles");
  container.textContent = "";
  Object.keys(config.profiles || {}).forEach(function (name) {
    var button = document.createElement("button");
    button.type = "button";
    button.textContent = "Profil: " + name;
    button.addEventListener("click", function () {
      post("/api/profile/" + encodeURIComponent(name));
    });
    container.appendChild(button);
  });
}

// Rows are built with DOM nodes instead of innerHTML, because event messages
// contain values from var.ini and from user defined profile names.
async function refreshEvents() {
  var rows = await getJSON("/api/events?limit=80");
  var body = byId("events");
  body.textContent = "";
  rows.slice().reverse().forEach(function (event) {
    var tr = document.createElement("tr");
    [new Date(event.time).toLocaleString(), event.type, event.message].forEach(function (cell) {
      var td = document.createElement("td");
      td.textContent = cell === null || cell === undefined ? "" : String(cell);
      tr.appendChild(td);
    });
    body.appendChild(tr);
  });
}

async function refreshHistory() {
  drawChart(await getJSON("/api/history?limit=288"));
}

function drawChart(data) {
  var canvas = byId("chart");
  var ratio = window.devicePixelRatio || 1;
  var width = canvas.clientWidth || 1100;
  var height = 260;
  canvas.width = Math.floor(width * ratio);
  canvas.height = Math.floor(height * ratio);

  var ctx = canvas.getContext("2d");
  ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
  ctx.clearRect(0, 0, width, height);
  ctx.strokeStyle = "#29313d";
  ctx.lineWidth = 1;
  for (var i = 0; i <= 5; i++) {
    var y = 20 + i * (height - 40) / 5;
    ctx.beginPath();
    ctx.moveTo(40, y);
    ctx.lineTo(width - 20, y);
    ctx.stroke();
  }

  ctx.fillStyle = "#ffb86c";
  ctx.fillText("Temperatur", 45, 14);
  ctx.fillStyle = "#6aa9ff";
  ctx.fillText("Luefter %", 125, 14);
  if (!data || data.length < 2) {
    ctx.fillStyle = "#8e9aab";
    ctx.fillText("Noch keine Messpunkte", 45, height / 2);
    return;
  }

  var temps = data.map(function (point) { return point.temperature; });
  var maxTemp = Math.max.apply(null, [55].concat(temps));
  var minTemp = Math.min.apply(null, [20].concat(temps));
  var span = maxTemp - minTemp || 1;
  var px = function (index) { return 40 + index * (width - 60) / (data.length - 1); };
  var pyTemp = function (value) { return height - 20 - (value - minTemp) * (height - 40) / span; };
  var pyFan = function (value) { return height - 20 - value * (height - 40) / 100; };

  ctx.lineWidth = 2;
  ctx.strokeStyle = "#ffb86c";
  ctx.beginPath();
  data.forEach(function (point, index) {
    var y = pyTemp(point.temperature);
    if (index === 0) { ctx.moveTo(px(index), y); } else { ctx.lineTo(px(index), y); }
  });
  ctx.stroke();

  ctx.strokeStyle = "#6aa9ff";
  ctx.beginPath();
  data.forEach(function (point, index) {
    var y = pyFan(point.fan_percent);
    if (index === 0) { ctx.moveTo(px(index), y); } else { ctx.lineTo(px(index), y); }
  });
  ctx.stroke();

  ctx.fillStyle = "#8e9aab";
  ctx.fillText(new Date(data[0].time).toLocaleString(), 40, height - 4);
  var last = new Date(data[data.length - 1].time).toLocaleString();
  ctx.fillText(last, Math.max(40, width - 20 - ctx.measureText(last).width), height - 4);
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
