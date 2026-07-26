package webui

// IndexHTML, StyleCSS and ScriptJS are served from separate routes so that the
// Content-Security-Policy no longer needs 'unsafe-inline'. All icons are
// hand-drawn inline SVG (no icon font, no CDN) to stay inside default-src 'self'.
const IndexHTML = `<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>ZimaCube Fan Controller</title>
<link rel="stylesheet" href="/app.css">
</head>
<body>
<div class="shell">

<aside class="sidebar">
<div class="brand">
<span class="brand-icon-box">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="10" cy="10" r="6.5"/><path d="M10 10L10 4.2M10 10L14.7 12.8M10 10L5.3 12.8"/><circle cx="10" cy="10" r="1.3" fill="currentColor" stroke="none"/></svg>
</span>
<div>
<div class="brand-name">ZimaCube</div>
<div class="brand-sub">Fan Controller</div>
</div>
</div>

<nav class="side-nav">
<div class="side-nav-label">Ansicht</div>
<a href="#overview" class="side-link active" data-section="overview">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M2 10h3.2l1.8-5 3 10 2-8 1.5 3H18"/></svg>
Status
</a>
<a href="#control" class="side-link" data-section="control">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="6" x2="17" y2="6"/><circle cx="12" cy="6" r="1.6" fill="currentColor" stroke="none"/><line x1="3" y1="10" x2="17" y2="10"/><circle cx="7" cy="10" r="1.6" fill="currentColor" stroke="none"/><line x1="3" y1="14" x2="17" y2="14"/><circle cx="14" cy="14" r="1.6" fill="currentColor" stroke="none"/></svg>
Steuerung
</a>
<a href="#history" class="side-link" data-section="history">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="16" x2="4" y2="11"/><line x1="10" y1="16" x2="10" y2="6"/><line x1="16" y1="16" x2="16" y2="13"/></svg>
Verlauf
</a>
<a href="#events-section" class="side-link" data-section="events-section">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="3.5" cy="6" r="0.9" fill="currentColor" stroke="none"/><line x1="7" y1="6" x2="17" y2="6"/><circle cx="3.5" cy="10" r="0.9" fill="currentColor" stroke="none"/><line x1="7" y1="10" x2="17" y2="10"/><circle cx="3.5" cy="14" r="0.9" fill="currentColor" stroke="none"/><line x1="7" y1="14" x2="17" y2="14"/></svg>
Ereignisse
</a>
<a href="#config-section" class="side-link" data-section="config-section">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M7 3C5.5 3 5 3.8 5 5v2.5c0 1-.4 1.5-1.5 1.5 1.1 0 1.5.5 1.5 1.5V13c0 1.2.5 2 2 2"/><path d="M13 3c1.5 0 2 .8 2 2v2.5c0 1 .4 1.5 1.5 1.5-1.1 0-1.5.5-1.5 1.5V13c0 1.2-.5 2-2 2"/></svg>
Konfiguration
</a>
</nav>

<div class="sidebar-footer">
<span class="dot" id="sidebarDot"></span>
<div>
<div class="sidebar-footer-title">Controller</div>
<div class="sidebar-footer-sub" id="sidebarStatus">–</div>
</div>
<button type="button" id="logout" class="sidebar-logout" title="Abmelden" aria-label="Abmelden">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M8 4H4.8A1.8 1.8 0 0 0 3 5.8v8.4A1.8 1.8 0 0 0 4.8 16H8"/><path d="M13 6.5 17 10l-4 3.5"/><line x1="17" y1="10" x2="7.5" y2="10"/></svg>
</button>
</div>
</aside>

<div class="content">

<header class="page-head">
<div>
<h1>Dashboard</h1>
<div class="sub">Live-Überwachung deiner HDD-Lüftersteuerung <span id="version" class="version-badge"></span></div>
</div>
<div class="head-actions">
<button type="button" id="themeToggle" class="theme-toggle" title="Hell/Dunkel umschalten" aria-label="Hell/Dunkel umschalten">
<svg class="icon-sun" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="10" cy="10" r="3.2"/><path d="M10 2.5v2M10 15.5v2M2.5 10h2M15.5 10h2M4.9 4.9l1.4 1.4M13.7 13.7l1.4 1.4M4.9 15.1l1.4-1.4M13.7 6.3l1.4-1.4"/></svg>
<svg class="icon-moon" viewBox="0 0 20 20" fill="currentColor" stroke="none"><path d="M15.8 12.9A6.2 6.2 0 0 1 7.1 4.2a6.7 6.7 0 1 0 8.7 8.7Z"/></svg>
</button>
<button id="refresh" type="button" class="pill-btn pill-primary">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M3.9 10A6.1 6.1 0 0 1 16 6.9"/><path d="M16.1 10A6.1 6.1 0 0 1 4 13.1"/><path d="M16 4v3.3h-3.3"/><path d="M4 16v-3.3h3.3"/></svg>
Aktualisieren
</button>
</div>
</header>

<div id="banner" class="banner" hidden></div>

<section id="overview" class="section">
<h2 class="section-title">System Overview</h2>

<div class="hero-grid">
<div class="hero-card">
<div class="hero-head">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="10" cy="10" r="6.5"/><path d="M10 10L10 4.2M10 10L14.7 12.8M10 10L5.3 12.8"/><circle cx="10" cy="10" r="1.3" fill="currentColor" stroke="none"/></svg></span>
<div><div class="hero-title">Lüfter</div><div class="hero-sub">Sollwert (angeforderter Prozentwert)</div></div>
</div>
<div class="hero-value" id="fan">-</div>
<div class="meter"><div class="meter-fill" id="fanMeter"></div></div>
<div class="hero-meta"><span id="fanMetaLeft">-</span><span id="fanMetaRight" class="muted"></span></div>
<div class="muted small" id="fanFeedbackNote">Keine RPM-Rückmeldung vom Controller, nur der zuletzt geschriebene Wert wird angezeigt.</div>
</div>

<div class="hero-card">
<div class="hero-head">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M11.5 11.2V4.5a1.5 1.5 0 0 0-3 0v6.7a3 3 0 1 0 3 0Z"/><circle cx="10" cy="14" r="1" fill="currentColor" stroke="none"/></svg></span>
<div><div class="hero-title">Temperatur</div><div class="hero-sub">Maximale HDD-Temperatur</div></div>
</div>
<div class="hero-value" id="temp">-</div>
<div class="meter"><div class="meter-fill" id="tempMeter"></div></div>
<div class="hero-meta"><span id="tempMetaLeft">-</span><span id="tempMetaRight" class="muted"></span></div>
</div>

<div class="hero-card">
<div class="hero-head">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M10 2.5 16 4.8V9.5C16 13.8 13.4 17 10 18 6.6 17 4 13.8 4 9.5V4.8Z"/></svg></span>
<div><div class="hero-title">Sicherheitsmarge</div><div class="hero-sub">Abstand zur Notfalltemperatur</div></div>
</div>
<div class="hero-value" id="margin">-</div>
<div class="meter"><div class="meter-fill" id="marginMeter"></div></div>
<div class="hero-meta"><span id="marginMetaLeft">-</span><span id="marginMetaRight" class="muted"></span></div>
</div>
</div>

<div class="info-bar">
<span class="icon-box icon-box-lg">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><rect x="6" y="6" width="8" height="8" rx="1.5"/><path d="M8 6V3M12 6V3M8 17v-3M12 17v-3M6 8H3M6 12H3M17 8h-3M17 12h-3"/></svg>
</span>
<div class="info-main">
<div class="info-title">I²C Controller <span class="badge" id="modeBadge">-</span><span class="badge badge-neutral" id="profileBadge">-</span><span class="version-badge" id="infoVersion"></span></div>
<div class="info-meta" id="infoMeta"></div>
<div class="info-reason" id="reason">-</div>
</div>
<div class="info-status-wrap"><span class="dot" id="controllerDot"></span><span id="controller" class="info-status">-</span></div>
</div>

<div class="panel">
<div class="panel-head"><div class="panel-head-title">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M11.5 11.2V4.5a1.5 1.5 0 0 0-3 0v6.7a3 3 0 1 0 3 0Z"/><circle cx="10" cy="14" r="1" fill="currentColor" stroke="none"/></svg></span>
<div><h2>Festplatten</h2><div class="muted small">Temperatur je HDD (Cache/SSD ausgeschlossen)</div></div>
</div></div>
<div class="disk-grid" id="diskGrid"><div class="muted small">Keine Festplatten erkannt.</div></div>
</div>
</section>

<section id="control" class="section">
<h2 class="section-title">Steuerung</h2>
<div class="resource-grid">

<div class="panel">
<div class="panel-head panel-head-row">
<div class="panel-head-title">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M10 3 17 7 10 11 3 7Z"/><path d="M3 11l7 4 7-4"/><path d="M3 15l7 4 7-4"/></svg></span>
<div><h2>Profile</h2><div class="muted small">Aktives Lüfterprofil wechseln</div></div>
</div>
</div>
<div class="table-wrap">
<table>
<thead><tr><th>Profil</th><th>Boost</th><th>Notfall</th><th>Status</th></tr></thead>
<tbody id="profileTable"></tbody>
</table>
</div>
</div>

<div class="panel">
<div class="panel-head">
<div class="panel-head-title">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><line x1="3" y1="6" x2="17" y2="6"/><circle cx="12" cy="6" r="1.6" fill="currentColor" stroke="none"/><line x1="3" y1="10" x2="17" y2="10"/><circle cx="7" cy="10" r="1.6" fill="currentColor" stroke="none"/><line x1="3" y1="14" x2="17" y2="14"/><circle cx="14" cy="14" r="1.6" fill="currentColor" stroke="none"/></svg></span>
<div><h2>Modus &amp; Test</h2><div class="muted small">Automatik, manuell oder Notfall; darunter ein Testlauf</div></div>
</div>
</div>
<div class="mode-switch" id="modeSwitch">
<button type="button" class="mode-btn" data-set-mode="automatic">Automatik</button>
<button type="button" class="mode-btn" data-set-mode="manual">Manuell</button>
<button type="button" class="mode-btn mode-btn-danger" data-set-mode="emergency">Notfall</button>
</div>
<div class="controls-row" id="manualRow" hidden>
<input id="percent" class="input num-input" type="number" min="1" max="100" value="75">
<button id="setManual" class="pill-btn pill-primary" type="button">Setzen</button>
</div>
<div class="controls-row" id="testRow" hidden>
<span class="muted small">Test</span>
<button type="button" class="pill-btn pill-ghost" data-test="25">25 %</button>
<button type="button" class="pill-btn pill-ghost" data-test="50">50 %</button>
<button type="button" class="pill-btn pill-ghost" data-test="75">75 %</button>
<button type="button" class="pill-btn pill-ghost" data-test="100">100 %</button>
</div>
<pre id="message" class="message" hidden></pre>
</div>

</div>
</section>

<section id="history" class="section">
<h2 class="section-title">Verlauf</h2>
<div class="chart-grid">
<div class="panel">
<div class="panel-head"><div class="panel-head-title">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="16" x2="4" y2="11"/><line x1="10" y1="16" x2="10" y2="6"/><line x1="16" y1="16" x2="16" y2="13"/></svg></span>
<div><h2>Temperatur</h2><div class="muted small">Maximale HDD-Temperatur</div></div>
</div></div>
<div class="chart-wrap">
<canvas id="chartTemp" height="220"></canvas>
<div class="chart-tooltip" id="chartTempTooltip" hidden></div>
</div>
</div>
<div class="panel">
<div class="panel-head"><div class="panel-head-title">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><line x1="4" y1="16" x2="4" y2="11"/><line x1="10" y1="16" x2="10" y2="6"/><line x1="16" y1="16" x2="16" y2="13"/></svg></span>
<div><h2>Lüfterdrehzahl</h2><div class="muted small">Geschriebener Prozentwert</div></div>
</div></div>
<div class="chart-wrap">
<canvas id="chartFan" height="220"></canvas>
<div class="chart-tooltip" id="chartFanTooltip" hidden></div>
</div>
</div>
</div>
</section>

<section id="events-section" class="section">
<div class="panel">
<div class="panel-head panel-head-row">
<div class="panel-head-title">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="3.5" cy="6" r="0.9" fill="currentColor" stroke="none"/><line x1="7" y1="6" x2="17" y2="6"/><circle cx="3.5" cy="10" r="0.9" fill="currentColor" stroke="none"/><line x1="7" y1="10" x2="17" y2="10"/><circle cx="3.5" cy="14" r="0.9" fill="currentColor" stroke="none"/><line x1="7" y1="14" x2="17" y2="14"/></svg></span>
<div><h2>Ereignisse</h2><div class="muted small">Verlauf von Modus- und Lüfteränderungen</div></div>
</div>
<input id="eventFilter" class="input filter-input" type="text" placeholder="Filter...">
</div>
<div class="table-wrap">
<table>
<thead><tr><th>Zeit</th><th>Typ</th><th>Meldung</th></tr></thead>
<tbody id="events"></tbody>
</table>
</div>
<div class="table-footer pagination">
<span class="muted small" id="eventsFooter"></span>
<div class="pagination-controls">
<button type="button" class="pill-btn pill-ghost table-action" id="eventsPrev" aria-label="Vorherige Seite">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 4 6 10l6 6"/></svg>
</button>
<span class="muted small" id="eventsPageLabel">Seite 1</span>
<button type="button" class="pill-btn pill-ghost table-action" id="eventsNext" aria-label="Nächste Seite">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M8 4l6 6-6 6"/></svg>
</button>
</div>
</div>
</div>
</section>

<section id="config-section" class="section">
<div class="panel">
<div class="panel-head"><div class="panel-head-title">
<span class="icon-box"><svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M7 3C5.5 3 5 3.8 5 5v2.5c0 1-.4 1.5-1.5 1.5 1.1 0 1.5.5 1.5 1.5V13c0 1.2.5 2 2 2"/><path d="M13 3c1.5 0 2 .8 2 2v2.5c0 1 .4 1.5 1.5 1.5-1.1 0-1.5.5-1.5 1.5V13c0 1.2-.5 2-2 2"/></svg></span>
<div><h2>Konfiguration</h2><div class="muted small">Profile als JSON bearbeiten</div></div>
</div></div>
<textarea id="config" class="input config-editor" spellcheck="false"></textarea>
<div class="controls-row mt">
<button id="saveConfig" class="pill-btn pill-primary" type="button">Speichern</button>
<button id="reloadConfig" class="pill-btn" type="button">Verwerfen und neu laden</button>
</div>
</div>
</section>

</div>
</div>

<script src="/app.js"></script>
</body>
</html>`

// LoginHTML is served unauthenticated at GET /login. It intentionally does
// not load app.js (which assumes the dashboard DOM exists), loading its own
// small login.js instead; the strict script-src (inherited from default-src
// 'self', see securityHeaders) rules out an inline <script> here.
const LoginHTML = `<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Anmelden – ZimaCube Fan Controller</title>
<link rel="stylesheet" href="/app.css">
</head>
<body class="login-body">
<div class="login-shell">
<form class="login-card" id="loginForm">
<span class="brand-icon-box login-icon">
<svg viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><circle cx="10" cy="10" r="6.5"/><path d="M10 10L10 4.2M10 10L14.7 12.8M10 10L5.3 12.8"/><circle cx="10" cy="10" r="1.3" fill="currentColor" stroke="none"/></svg>
</span>
<h1 class="login-title">ZimaCube Fan Controller</h1>
<div class="muted small login-sub">Bitte anmelden</div>
<label class="login-label" for="loginUser">Benutzername</label>
<input id="loginUser" class="input login-input" type="text" autocomplete="username" required>
<label class="login-label" for="loginPassword">Passwort</label>
<input id="loginPassword" class="input login-input" type="password" autocomplete="current-password" required>
<button type="submit" class="pill-btn pill-primary login-submit">Anmelden</button>
<div class="login-error" id="loginError" hidden></div>
</form>
</div>
<script src="/login.js"></script>
</body>
</html>`

const LoginJS = `"use strict";
document.getElementById("loginForm").addEventListener("submit", async function (event) {
  event.preventDefault();
  var user = document.getElementById("loginUser").value;
  var password = document.getElementById("loginPassword").value;
  var errorBox = document.getElementById("loginError");
  errorBox.hidden = true;
  try {
    var response = await fetch("/login", {
      method: "POST",
      headers: { Authorization: "Basic " + btoa(user + ":" + password) }
    });
    if (response.ok) {
      window.location.href = "/";
      return;
    }
    var body = await response.json().catch(function () { return {}; });
    errorBox.textContent = body.error || "Anmeldung fehlgeschlagen.";
    errorBox.hidden = false;
  } catch (error) {
    errorBox.textContent = String(error);
    errorBox.hidden = false;
  }
});`

const StyleCSS = `:root{
color-scheme:dark;
--bg:#0a0a10;
--sidebar-bg:#0c0c13;
--card:#121218;
--card-2:#17171f;
--border:#1e1e27;
--border-soft:#2a2a35;
--text:#f4f4f6;
--text-muted:#9d9dae;
--text-dim:#6d6d80;
--purple:#8b5cf6;
--purple-dark:#6d28d9;
--purple-soft:rgba(139,92,246,.14);
--blue:#3b82f6;
--green:#22c55e;
--yellow:#eab308;
--red:#ef4444;
--radius-card:16px;
--radius-control:10px;
}
:root[data-theme="light"]{
color-scheme:light;
--bg:#f4f4f8;
--sidebar-bg:#ffffff;
--card:#ffffff;
--card-2:#f0f0f5;
--border:#e3e3ec;
--border-soft:#d4d4e2;
--text:#16161f;
--text-muted:#5c5c70;
--text-dim:#8b8ba0;
--purple-soft:rgba(139,92,246,.12);
}
*{box-sizing:border-box}
html{scroll-behavior:smooth;background:var(--bg)}
body{
margin:0;color:var(--text);background:var(--bg);
font-family:Inter,-apple-system,"Segoe UI",system-ui,sans-serif;
font-size:14px;line-height:1.45;
-webkit-font-smoothing:antialiased;
}
h1,h2{margin:0;font-weight:650;letter-spacing:-.01em}
h1{font-size:1.7rem}
h2{font-size:.95rem}
.muted{color:var(--text-muted)}
.small{font-size:.78rem}
svg{display:block}

.shell{display:flex;align-items:flex-start;min-height:100vh}

.sidebar{
width:250px;flex:0 0 250px;background:var(--sidebar-bg);border-right:1px solid var(--border);
display:flex;flex-direction:column;padding:22px 16px;position:sticky;top:0;height:100vh;overflow-y:auto;
}
.brand{display:flex;align-items:center;gap:12px;padding:4px 8px 24px}
.brand-icon-box{
width:42px;height:42px;border-radius:11px;flex:0 0 auto;
background:linear-gradient(135deg,var(--purple),var(--purple-dark));
display:flex;align-items:center;justify-content:center;color:#fff;
}
.brand-icon-box svg{width:22px;height:22px}
.brand-name{font-weight:700;font-size:1rem;letter-spacing:-.01em}
.brand-sub{font-size:.72rem;color:var(--text-muted)}

.side-nav{display:flex;flex-direction:column;gap:2px;margin-top:4px}
.side-nav-label{font-size:.68rem;text-transform:uppercase;letter-spacing:.06em;color:var(--text-dim);padding:10px 10px 6px;font-weight:650}
.side-link{
display:flex;align-items:center;gap:10px;padding:9px 10px;border-radius:9px;
color:var(--text-muted);text-decoration:none;font-size:.86rem;font-weight:550;
border-left:2px solid transparent;
}
.side-link svg{width:16px;height:16px;flex:0 0 auto}
.side-link:hover{background:var(--card-2);color:var(--text)}
/* Deliberately squarer than the hover state: a thicker left rail and tight
   corners read as an active tab, not another rounded pill. */
.side-link.active{
background:var(--purple-soft);color:var(--text);border-left:3px solid var(--purple);
border-radius:4px;font-weight:650;
}

.sidebar-footer{margin-top:auto;display:flex;align-items:center;gap:10px;padding:14px 10px 4px;border-top:1px solid var(--border)}
.sidebar-footer-title{font-size:.8rem;font-weight:650}
.sidebar-footer-sub{font-size:.72rem;color:var(--text-muted)}
.sidebar-logout{
margin-left:auto;width:30px;height:30px;border-radius:8px;flex:0 0 auto;
border:1px solid transparent;background:transparent;color:var(--text-dim);
display:flex;align-items:center;justify-content:center;cursor:pointer;
}
.sidebar-logout svg{width:16px;height:16px}
.sidebar-logout:hover{background:var(--card-2);color:var(--red);border-color:var(--border-soft)}

.content{flex:1;min-width:0;padding:26px 32px 60px;display:flex;flex-direction:column;gap:26px}

.page-head{display:flex;justify-content:space-between;align-items:flex-start;gap:16px;flex-wrap:wrap}
.sub{color:var(--text-muted);font-size:.82rem;margin-top:6px}
.head-actions{display:flex;gap:10px;align-items:center;flex-wrap:wrap}

.version-badge{
display:inline-block;padding:2px 8px;border-radius:6px;background:var(--card-2);
border:1px solid var(--border-soft);color:var(--text-muted);font-size:.72rem;font-weight:650;
margin-left:4px;vertical-align:middle;
}

.pill-btn{
display:inline-flex;align-items:center;gap:7px;padding:9px 15px;border-radius:var(--radius-control);
border:1px solid var(--border-soft);background:var(--card-2);color:var(--text);
font:inherit;font-weight:650;font-size:.85rem;cursor:pointer;
}
.pill-btn svg{width:14px;height:14px}
.pill-btn:hover{background:#1c1c26;border-color:var(--text-dim)}
.pill-primary{background:var(--purple);border-color:var(--purple);color:#fff}
.pill-primary:hover{background:#7c4de0;border-color:#7c4de0}
.pill-danger{color:#fca5a5;border-color:rgba(239,68,68,.35)}
.pill-danger:hover{background:rgba(239,68,68,.12);border-color:var(--red)}
.pill-ghost{background:transparent}
.table-action{padding:6px 12px;font-size:.78rem}

.input{
border-radius:var(--radius-control);border:1px solid var(--border-soft);background:var(--card-2);
color:var(--text);padding:9px 12px;font:inherit;
}
.input:focus{outline:2px solid var(--purple);outline-offset:-1px}
.num-input{width:90px}
.filter-input{width:200px}

.theme-toggle{
width:38px;height:38px;border-radius:10px;flex:0 0 auto;
border:1px solid var(--border-soft);background:var(--card-2);color:var(--text);
display:flex;align-items:center;justify-content:center;cursor:pointer;
}
.theme-toggle svg{width:18px;height:18px}
.theme-toggle:hover{border-color:var(--text-dim)}
.theme-toggle .icon-moon{display:none}
:root[data-theme="light"] .theme-toggle .icon-sun{display:none}
:root[data-theme="light"] .theme-toggle .icon-moon{display:block}

.mode-switch{display:flex;gap:2px;background:var(--card-2);border:1px solid var(--border-soft);border-radius:var(--radius-control);padding:3px}
.mode-btn{
flex:1;padding:9px 10px;border:none;background:transparent;color:var(--text-muted);
font:inherit;font-weight:650;font-size:.85rem;border-radius:7px;cursor:pointer;
}
.mode-btn:hover{color:var(--text)}
.mode-btn.active{background:var(--purple);color:#fff}
.mode-btn-danger.active{background:var(--red)}

.disk-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(140px,1fr));gap:10px}
.disk-tile{
border:1px solid var(--border);border-radius:12px;padding:10px 12px;background:var(--card-2);
}
.disk-tile-name{font-size:.78rem;font-weight:650;color:var(--text-muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.disk-tile-value{font-size:1.3rem;font-weight:700;margin-top:2px}
.disk-tile-value.standby{color:var(--text-dim);font-size:.95rem;font-weight:650}

.banner{
margin:0;padding:12px 16px;border-radius:12px;
border:1px solid rgba(239,68,68,.35);background:rgba(239,68,68,.1);color:#fca5a5;
}

/* Each section is its own "page": the sidebar switches which one is
   .active instead of scrolling a single long page (see showSection in
   app.js). */
.section{display:none;flex-direction:column;gap:14px}
.section.active{display:flex}
.section-title{font-size:.78rem;text-transform:uppercase;letter-spacing:.06em;color:var(--text-dim);font-weight:650}

.hero-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(230px,1fr));gap:14px}
.hero-card{background:var(--card);border:1px solid var(--border);border-radius:var(--radius-card);padding:18px 20px}
.hero-head{display:flex;align-items:center;gap:12px;margin-bottom:14px}
.hero-title{font-weight:650;font-size:.92rem}
.hero-sub{font-size:.74rem;color:var(--text-muted);margin-top:1px}
.hero-value{font-size:2rem;font-weight:700;letter-spacing:-.02em}
.hero-meta{display:flex;justify-content:space-between;margin-top:9px;font-size:.78rem;color:var(--text-muted)}

.icon-box{
width:38px;height:38px;border-radius:10px;flex:0 0 auto;
background:linear-gradient(135deg,var(--purple),var(--purple-dark));
display:flex;align-items:center;justify-content:center;color:#fff;
}
.icon-box svg{width:19px;height:19px}
.icon-box-lg{width:46px;height:46px;border-radius:12px}
.icon-box-lg svg{width:24px;height:24px}

.meter{margin-top:10px;height:7px;border-radius:999px;background:var(--border);overflow:hidden}
.meter-fill{height:100%;width:0;border-radius:999px;background:var(--blue);transition:width .3s ease}
.meter-fill.ok{background:var(--green)}
.meter-fill.warn{background:var(--yellow)}
.meter-fill.bad{background:var(--red)}

.info-bar{
display:flex;align-items:center;gap:14px;flex-wrap:wrap;
background:var(--card);border:1px solid var(--border);border-radius:var(--radius-card);padding:14px 18px;
}
.info-main{flex:1;min-width:220px}
.info-title{font-weight:650;font-size:.92rem;display:flex;align-items:center;flex-wrap:wrap;gap:2px}
.info-meta{display:flex;flex-wrap:wrap;margin-top:6px;font-size:.78rem;color:var(--text-muted)}
.info-meta span{padding-right:12px;margin-right:12px;border-right:1px solid var(--border)}
.info-meta span:last-child{border-right:none;padding-right:0;margin-right:0}
.info-reason{margin-top:6px;color:var(--text-muted);font-size:.82rem}
.info-status-wrap{display:flex;align-items:center}
.info-status{font-weight:650;font-size:.85rem}

.badge{
display:inline-flex;align-items:center;padding:3px 10px;border-radius:999px;
font-size:.72rem;font-weight:650;margin-left:8px;
}
.badge-green{background:rgba(34,197,94,.14);color:#4ade80}
.badge-yellow{background:rgba(234,179,8,.14);color:#facc15}
.badge-red{background:rgba(239,68,68,.14);color:#f87171}
.badge-neutral{background:var(--card-2);color:var(--text-muted)}

.dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:8px;vertical-align:middle;background:var(--text-dim)}
.dot-ok{background:var(--green);box-shadow:0 0 0 3px rgba(34,197,94,.16)}
.dot-bad{background:var(--red);box-shadow:0 0 0 3px rgba(239,68,68,.16)}

.panel{background:var(--card);border:1px solid var(--border);border-radius:var(--radius-card);padding:18px 20px}
.panel-head{margin-bottom:12px}
.panel-head-row{display:flex;justify-content:space-between;align-items:center;gap:12px;flex-wrap:wrap}
.panel-head-title{display:flex;align-items:center;gap:12px}

.controls-row{display:flex;gap:10px;flex-wrap:wrap;align-items:center;margin-top:10px}
.controls-row:first-child{margin-top:0}
.controls-row.mt{margin-top:14px}
/* Without this, the [hidden] attribute has no visual effect here: an author
   rule (this class sets display:flex) always wins over the browser's
   default [hidden]{display:none} UA rule at equal specificity, regardless
   of the hidden attribute being present. manualRow/testRow only actually
   toggled their DOM property, never their rendered visibility. */
.controls-row[hidden]{display:none}

.resource-grid{display:grid;grid-template-columns:1.15fr .85fr;gap:16px}
@media (max-width:1000px){.resource-grid{grid-template-columns:1fr}}

.message{white-space:pre-wrap;overflow:auto;color:var(--text-muted);margin:12px 0 0;font-size:.85rem}

.chart-grid{display:grid;grid-template-columns:1fr 1fr;gap:16px}
@media (max-width:860px){.chart-grid{grid-template-columns:1fr}}
.chart-wrap{position:relative}
canvas{width:100%;display:block}
.chart-tooltip{
position:absolute;pointer-events:none;z-index:2;
background:var(--card-2);border:1px solid var(--border-soft);border-radius:8px;
padding:6px 10px;font-size:.76rem;color:var(--text);white-space:nowrap;
transform:translate(-50%,-115%);box-shadow:0 4px 14px rgba(0,0,0,.25);
}
.chart-tooltip[hidden]{display:none}

.table-wrap{overflow-x:auto}
table{width:100%;border-collapse:collapse;font-size:.85rem}
th{
text-align:left;color:var(--text-muted);font-weight:600;font-size:.76rem;
text-transform:uppercase;letter-spacing:.03em;padding:0 10px 10px;border-bottom:1px solid var(--border);
}
td{padding:10px;border-bottom:1px solid var(--border);color:var(--text)}
tbody tr:hover{background:var(--card-2)}
tbody tr:last-child td{border-bottom:none}
.table-footer{padding:10px 2px 0}
.pagination{display:flex;align-items:center;justify-content:space-between;gap:12px;flex-wrap:wrap}
.pagination-controls{display:flex;align-items:center;gap:8px}
.pagination-controls .table-action{padding:6px 9px}
.pagination-controls .table-action svg{width:14px;height:14px}
.pagination-controls .table-action:disabled{opacity:.4;cursor:default;pointer-events:none}
#eventsPageLabel{min-width:70px;text-align:center}

.config-editor{width:100%;min-height:220px;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.82rem;resize:vertical}

.login-body{display:flex;min-height:100vh;align-items:center;justify-content:center;padding:20px}
.login-shell{width:100%;max-width:360px}
.login-card{
display:flex;flex-direction:column;gap:4px;
background:var(--card);border:1px solid var(--border);border-radius:var(--radius-card);
padding:32px 28px;text-align:center;
}
.login-icon{margin:0 auto 14px}
.login-title{font-size:1.15rem}
.login-sub{margin-bottom:18px}
.login-label{text-align:left;font-size:.78rem;color:var(--text-muted);font-weight:650;margin:10px 0 4px}
.login-input{width:100%}
.login-submit{width:100%;justify-content:center;margin-top:18px}
.login-error{color:#fca5a5;font-size:.82rem;margin-top:12px}
.login-error[hidden]{display:none}

@media (max-width:900px){
.shell{flex-direction:column}
.sidebar{
width:100%;flex:0 0 auto;height:auto;position:relative;
flex-direction:row;align-items:center;gap:18px;padding:14px 16px;overflow-x:auto;
}
.brand{padding:0}
.side-nav{flex-direction:row;margin-top:0}
.side-nav-label{display:none}
.side-link{white-space:nowrap}
.sidebar-footer{display:none}
.content{padding:18px 16px 40px}
}`

const ScriptJS = `"use strict";
var byId = function (id) { return document.getElementById(id); };
var themeKey = "zimafan-theme";
var configDirty = false;
var lastEvents = [];
var lastProfiles = [];
var activeProfileName = "";
var activeEmergencyTemp = 52;
// Set to "manual" while the operator has opened the Manuell view locally but
// not yet clicked "Setzen" (so the server override is not "manual" yet). See
// effectiveMode(): this is what keeps the highlighted button and the
// manual/test rows from disagreeing with each other.
var localModeOverride = null;
var messageTimer = null;

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme === "light" ? "light" : "dark";
}
applyTheme(localStorage.getItem(themeKey));

var themeColors = getComputedStyle(document.documentElement);
var colorFan = themeColors.getPropertyValue("--blue").trim() || "#3b82f6";
var colorTemp = themeColors.getPropertyValue("--purple").trim() || "#8b5cf6";

function toggleTheme() {
  var next = document.documentElement.dataset.theme === "light" ? "dark" : "light";
  localStorage.setItem(themeKey, next);
  applyTheme(next);
  refreshHistory();
}

// Shown for 4s and then cleared automatically, so a stale confirmation (e.g.
// "{"status":"auto requested"}") does not sit on screen forever.
function say(text) {
  var box = byId("message");
  box.textContent = text;
  box.hidden = !text;
  if (messageTimer) { clearTimeout(messageTimer); }
  if (text) {
    messageTimer = setTimeout(function () { box.hidden = true; box.textContent = ""; }, 4000);
  }
}

// fetchWithTimeout aborts a request that is still pending after timeoutMs.
// Without this, a connection stalled by e.g. the machine sleeping or a
// network change never resolves nor rejects, and the tab looks permanently
// stuck loading until it is refreshed by hand.
function fetchWithTimeout(url, options, timeoutMs) {
  var controller = new AbortController();
  var timer = setTimeout(function () { controller.abort(); }, timeoutMs || 10000);
  var merged = Object.assign({}, options, { signal: controller.signal });
  return fetch(url, merged).finally(function () { clearTimeout(timer); });
}

async function getJSON(url) {
  var response = await fetchWithTimeout(url, { cache: "no-store" });
  if (response.status === 401) { window.location.href = "/login"; throw new Error("nicht angemeldet"); }
  var text = await response.text();
  if (!response.ok) { throw new Error(response.status + ": " + text); }
  return JSON.parse(text);
}

// Turns a JSON API response into a short, human message instead of dumping
// the raw payload: the error field on failure, otherwise a plain confirmation.
async function describeResponse(response) {
  var text = await response.text();
  var parsed = null;
  try { parsed = JSON.parse(text); } catch (error) { /* not JSON, fall through */ }
  if (parsed && parsed.error) { return parsed.error; }
  if (response.ok) { return "Erledigt."; }
  return response.status + ": " + text;
}

async function post(path, body) {
  try {
    var response = await fetchWithTimeout(path, {
      method: "POST",
      headers: body ? { "Content-Type": "application/json" } : {},
      body: body ? JSON.stringify(body) : undefined
    });
    if (response.status === 401) { window.location.href = "/login"; return false; }
    say(await describeResponse(response));
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

function badgeClassForMode(mode) {
  switch (mode) {
    case "emergency": case "failsafe": return "badge-red";
    case "array-boost": return "badge-yellow";
    case "automatic": return "badge-green";
    default: return "badge-neutral";
  }
}

function levelClass(percentBad) {
  if (percentBad >= 90) { return "bad"; }
  if (percentBad >= 70) { return "warn"; }
  return "ok";
}

function clampPct(value) { return Math.max(0, Math.min(100, value || 0)); }

function formatUptime(seconds) {
  seconds = Math.max(0, seconds || 0);
  var days = Math.floor(seconds / 86400);
  var hours = Math.floor((seconds % 86400) / 3600);
  var minutes = Math.floor((seconds % 3600) / 60);
  if (days > 0) { return days + "d " + hours + "h"; }
  if (hours > 0) { return hours + "h " + minutes + "m"; }
  return minutes + "m";
}

function metaSpan(container, text) {
  var span = document.createElement("span");
  span.textContent = text;
  container.appendChild(span);
}

async function refreshStatus() {
  var data = await getJSON("/api/status");
  var status = data.status;

  byId("version").textContent = status.version ? "v" + status.version : "";
  byId("infoVersion").textContent = status.version ? "v" + status.version : "";

  byId("fan").textContent = status.target_percent + " %";
  byId("fanMeter").style.width = clampPct(status.target_percent) + "%";
  var appliedText = "Ist: " + status.last_applied_percent + " %";
  if (!status.last_write_successful) { appliedText += " (letzter Schreibversuch fehlgeschlagen)"; }
  byId("fanMetaLeft").textContent = appliedText;
  byId("fanMetaRight").textContent = labelForMode(status.mode);

  var tempRatio = status.temperature_valid ? clampPct(Math.round((status.maximum_disk_temperature / activeEmergencyTemp) * 100)) : null;
  byId("temp").textContent = status.temperature_valid ? status.maximum_disk_temperature + " °C" : "unbekannt";
  var tempMeter = byId("tempMeter");
  var marginMeter = byId("marginMeter");
  if (tempRatio === null) {
    tempMeter.style.width = "0%";
    tempMeter.className = "meter-fill bad";
    marginMeter.style.width = "0%";
    marginMeter.className = "meter-fill bad";
    byId("tempMetaLeft").textContent = "-";
    byId("margin").textContent = "-";
    byId("marginMetaLeft").textContent = "-";
  } else {
    tempMeter.style.width = tempRatio + "%";
    tempMeter.className = "meter-fill " + levelClass(tempRatio);
    var margin = clampPct(100 - tempRatio);
    marginMeter.style.width = margin + "%";
    marginMeter.className = "meter-fill " + levelClass(tempRatio);
    byId("tempMetaLeft").textContent = tempRatio + " % von " + activeEmergencyTemp + " °C";
    byId("margin").textContent = margin + " %";
    byId("marginMetaLeft").textContent = margin + " %";
  }
  byId("tempMetaRight").textContent = status.disks_reporting + " HDD" + (status.disks_reporting === 1 ? "" : "s");
  byId("marginMetaRight").textContent = "Grenze " + activeEmergencyTemp + " °C";

  byId("modeBadge").textContent = labelForMode(status.mode);
  byId("modeBadge").className = "badge " + badgeClassForMode(status.mode);
  byId("profileBadge").textContent = status.active_profile || "-";

  var metaContainer = byId("infoMeta");
  metaContainer.textContent = "";
  metaSpan(metaContainer, "Bus " + (status.i2c_bus !== undefined ? status.i2c_bus : "-"));
  metaSpan(metaContainer, status.i2c_address || "-");
  metaSpan(metaContainer, "Laufzeit " + formatUptime(status.uptime_seconds));
  metaSpan(metaContainer, "Array: " + (status.array_operation && status.array_operation !== "none" ? status.array_operation : "inaktiv"));

  byId("controller").textContent = status.controller_online ? "online" : "offline";
  byId("controllerDot").className = "dot " + (status.controller_online ? "dot-ok" : "dot-bad");
  byId("sidebarStatus").textContent = status.controller_online ? "online" : "offline";
  byId("sidebarDot").className = "dot " + (status.controller_online ? "dot-ok" : "dot-bad");

  byId("reason").textContent = status.reason || "-";
  setBanner(status);

  activeProfileName = status.active_profile || "";
  renderProfileTable();
  renderDiskGrid(status.disks);
  applyModeState((data.override && data.override.mode) || "");
}

function renderDiskGrid(disks) {
  var grid = byId("diskGrid");
  grid.textContent = "";
  if (!disks || disks.length === 0) {
    var empty = document.createElement("div");
    empty.className = "muted small";
    empty.textContent = "Keine Festplatten erkannt.";
    grid.appendChild(empty);
    return;
  }
  disks.forEach(function (disk) {
    var tile = document.createElement("div");
    tile.className = "disk-tile";

    var name = document.createElement("div");
    name.className = "disk-tile-name";
    name.textContent = disk.name || "-";
    tile.appendChild(name);

    var value = document.createElement("div");
    if (disk.valid) {
      value.className = "disk-tile-value";
      value.textContent = disk.temperature + " °C";
    } else {
      value.className = "disk-tile-value standby";
      value.textContent = "Standby";
    }
    tile.appendChild(value);

    grid.appendChild(tile);
  });
}

// Reconciles the server's override state with a locally opened-but-not-yet-
// committed Manuell view: a bare status poll reporting "automatic" must not
// yank the view away while the operator is still typing a percent, but a
// genuinely different committed override (e.g. Notfall set from another
// tab) must win over a stale local click.
function effectiveMode(serverMode) {
  if (localModeOverride === "manual") {
    if (serverMode === "" || serverMode === "manual") { return "manual"; }
    localModeOverride = null;
  }
  return serverMode || "automatic";
}

// Both the highlighted button and the manual/test rows are driven by this
// one mode value, so they can never disagree with each other the way a
// separately tracked "is the row open" flag could.
function setModeUI(mode) {
  document.querySelectorAll(".mode-btn").forEach(function (btn) {
    btn.classList.toggle("active", btn.dataset.setMode === mode);
  });
  var showExtra = mode === "manual";
  byId("manualRow").hidden = !showExtra;
  byId("testRow").hidden = !showExtra;
}

// The segmented mode control reflects the requested override (empty/"" means
// automatic), not the resulting live status.mode, which can show
// "array-boost"/"failsafe" from safety logic while the operator's own choice
// is still "automatic".
function applyModeState(overrideMode) {
  setModeUI(effectiveMode(overrideMode));
}

function renderProfileTable() {
  var body = byId("profileTable");
  body.textContent = "";
  lastProfiles.forEach(function (profile) {
    var tr = document.createElement("tr");

    var nameTd = document.createElement("td");
    var strong = document.createElement("strong");
    strong.textContent = profile.label;
    nameTd.appendChild(strong);

    var boostTd = document.createElement("td");
    boostTd.textContent = (profile.boost === undefined ? "-" : profile.boost + " %");

    var emergencyTd = document.createElement("td");
    emergencyTd.textContent = (profile.emergencyTemp === undefined ? "-" : profile.emergencyTemp + " °C");

    var statusTd = document.createElement("td");
    if (profile.id === activeProfileName) {
      var badge = document.createElement("span");
      badge.className = "badge badge-green";
      badge.style.marginLeft = "0";
      badge.textContent = "Aktiv";
      statusTd.appendChild(badge);
    } else {
      var button = document.createElement("button");
      button.type = "button";
      button.className = "pill-btn pill-ghost table-action";
      button.textContent = "Aktivieren";
      button.addEventListener("click", function () {
        post("/api/profile/" + encodeURIComponent(profile.id));
      });
      statusTd.appendChild(button);
    }

    tr.appendChild(nameTd);
    tr.appendChild(boostTd);
    tr.appendChild(emergencyTd);
    tr.appendChild(statusTd);
    body.appendChild(tr);
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
    var profile = profiles[id] || {};
    return { id: id, label: profile.name || id, boost: profile.array_boost_percent, emergencyTemp: profile.emergency_temperature };
  });
  var current = profiles[config.active_profile];
  if (current && current.emergency_temperature) { activeEmergencyTemp = current.emergency_temperature; }
  renderProfileTable();
}

var eventsPageSize = 10;
var eventsPage = 0;

// Rows are built with DOM nodes instead of innerHTML, because event messages
// contain values from var.ini and from user defined profile names.
function renderEvents() {
  var query = byId("eventFilter").value.trim().toLowerCase();
  var filtered = lastEvents.slice().reverse().filter(function (event) {
    var haystack = (String(event.type) + " " + String(event.message)).toLowerCase();
    return !query || haystack.indexOf(query) !== -1;
  });

  var pageCount = Math.max(1, Math.ceil(filtered.length / eventsPageSize));
  if (eventsPage >= pageCount) { eventsPage = pageCount - 1; }
  if (eventsPage < 0) { eventsPage = 0; }

  var start = eventsPage * eventsPageSize;
  var pageItems = filtered.slice(start, start + eventsPageSize);

  var body = byId("events");
  body.textContent = "";
  pageItems.forEach(function (event) {
    var tr = document.createElement("tr");
    [new Date(event.time).toLocaleString(), event.type, event.message].forEach(function (cell) {
      var td = document.createElement("td");
      td.textContent = cell === null || cell === undefined ? "" : String(cell);
      tr.appendChild(td);
    });
    body.appendChild(tr);
  });

  var shownFrom = filtered.length === 0 ? 0 : start + 1;
  var shownTo = Math.min(start + eventsPageSize, filtered.length);
  byId("eventsFooter").textContent = "Zeige " + shownFrom + "–" + shownTo + " von " + filtered.length + " Ereignissen";
  byId("eventsPageLabel").textContent = "Seite " + (eventsPage + 1) + " von " + pageCount;
  byId("eventsPrev").disabled = eventsPage <= 0;
  byId("eventsNext").disabled = eventsPage >= pageCount - 1;
}

async function refreshEvents() {
  lastEvents = await getJSON("/api/events?limit=200");
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
    ctx.fillStyle = "#6d6d80";
    ctx.fillText("Noch keine Messpunkte", 12, height / 2);
    canvas._tooltip = null;
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
    ctx.fillStyle = "#6d6d80";
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

  ctx.fillStyle = "#6d6d80";
  ctx.fillText(new Date(points[0].time).toLocaleString(), padLeft, height - 4);
  var last = new Date(points[points.length - 1].time).toLocaleString();
  ctx.fillText(last, Math.max(padLeft, width - padRight - ctx.measureText(last).width), height - 4);

  // Remembered so a mousemove handler (see wire()) can find the nearest
  // point by x position and show its exact value, without redrawing. py is
  // kept alongside px so the tooltip can require the cursor to actually be
  // near the line, not just anywhere in the gradient-filled area below it.
  canvas._tooltip = {
    points: points, valueKey: valueKey, suffix: suffix,
    px: points.map(function (_, index) { return px(index); }),
    py: points.map(function (point) { return py(point[valueKey]); })
  };
}

// hoverTolerancePx bounds how far (in CSS pixels) the cursor may be from the
// line itself, vertically, before the tooltip hides. Without it the tooltip
// showed anywhere under the line, including deep in the gradient fill.
var hoverTolerancePx = 14;

function showChartTooltip(canvas, tooltip, clientX, clientY) {
  var data = canvas._tooltip;
  if (!data) { tooltip.hidden = true; return; }

  var rect = canvas.getBoundingClientRect();
  var x = clientX - rect.left;
  var y = clientY - rect.top;
  var nearest = 0;
  var nearestDist = Infinity;
  data.px.forEach(function (px, index) {
    var dist = Math.abs(px - x);
    if (dist < nearestDist) { nearestDist = dist; nearest = index; }
  });

  if (Math.abs(y - data.py[nearest]) > hoverTolerancePx) {
    tooltip.hidden = true;
    return;
  }

  var point = data.points[nearest];
  var value = point[data.valueKey];
  tooltip.textContent = new Date(point.time).toLocaleString() + " – " + value + data.suffix;
  tooltip.style.left = x + "px";
  tooltip.style.top = y + "px";
  tooltip.hidden = false;
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

var sectionIds = ["overview", "control", "history", "events-section", "config-section"];

// Each sidebar entry is its own page: exactly one section is visible at a
// time (see the .section/.section.active rule in app.css) instead of one
// long page the old IntersectionObserver scroll-spy highlighted as you
// scrolled past it.
function showSection(id) {
  if (sectionIds.indexOf(id) === -1) { id = sectionIds[0]; }
  sectionIds.forEach(function (sectionId) {
    var section = byId(sectionId);
    if (section) { section.classList.toggle("active", sectionId === id); }
  });
  document.querySelectorAll(".side-link").forEach(function (link) {
    link.classList.toggle("active", link.dataset.section === id);
  });
  // canvas.clientWidth is always 0 while its section is display:none, so a
  // redraw that happened to run in the background (the 30s interval keeps
  // ticking regardless of which page is showing) can leave the chart sized
  // for a hidden 0-width element. Redraw once the History page is actually
  // visible again, so it always reflects the real container width.
  if (id === "history") { refreshHistory(); }
}

function setupNavigation() {
  var initial = window.location.hash.replace("#", "");
  showSection(initial || sectionIds[0]);
  window.addEventListener("hashchange", function () {
    showSection(window.location.hash.replace("#", ""));
  });
}

function wireChartTooltip(canvasId, tooltipId) {
  var canvas = byId(canvasId);
  var tooltip = byId(tooltipId);
  canvas.addEventListener("mousemove", function (event) {
    showChartTooltip(canvas, tooltip, event.clientX, event.clientY);
  });
  canvas.addEventListener("mouseleave", function () { tooltip.hidden = true; });
}

function wire() {
  byId("themeToggle").addEventListener("click", toggleTheme);

  byId("logout").addEventListener("click", async function () {
    await fetch("/logout", { method: "POST" });
    window.location.href = "/login";
  });

  document.querySelectorAll(".mode-btn").forEach(function (btn) {
    btn.addEventListener("click", function () {
      var mode = btn.dataset.setMode;
      if (mode === "manual") {
        localModeOverride = "manual";
        setModeUI("manual");
        byId("percent").focus();
        return;
      }
      localModeOverride = null;
      setModeUI(mode);
      post(mode === "emergency" ? "/api/mode/emergency" : "/api/mode/auto");
    });
  });

  byId("refresh").addEventListener("click", function () { refreshAll(); });
  byId("setManual").addEventListener("click", function () {
    post("/api/fan/" + encodeURIComponent(byId("percent").value));
  });
  byId("saveConfig").addEventListener("click", saveConfig);
  byId("reloadConfig").addEventListener("click", function () { refreshConfig(true); });
  byId("config").addEventListener("input", function () { configDirty = true; });
  byId("eventFilter").addEventListener("input", function () {
    eventsPage = 0;
    renderEvents();
  });
  byId("eventsPrev").addEventListener("click", function () {
    eventsPage--;
    renderEvents();
  });
  byId("eventsNext").addEventListener("click", function () {
    eventsPage++;
    renderEvents();
  });

  document.querySelectorAll("[data-test]").forEach(function (button) {
    button.addEventListener("click", function () {
      post("/api/test/" + encodeURIComponent(button.dataset.test));
    });
  });

  wireChartTooltip("chartTemp", "chartTempTooltip");
  wireChartTooltip("chartFan", "chartFanTooltip");

  window.addEventListener("resize", function () { refreshHistory(); });
  setupNavigation();

  // Browsers heavily throttle setInterval in a background tab (sometimes to
  // once a minute or less), so a dashboard left in a background tab can sit
  // on stale data for a long time. Force an immediate refresh the moment the
  // tab/window is actually looked at again instead of waiting for the next
  // (possibly long-delayed) tick.
  document.addEventListener("visibilitychange", function () {
    if (document.visibilityState === "visible") { refreshAll(); }
  });
  window.addEventListener("focus", function () { refreshAll(); });

  refreshAll();
  setInterval(refreshStatus, 3000);
  setInterval(refreshHistory, 30000);
  setInterval(refreshEvents, 30000);
}

wire();`
