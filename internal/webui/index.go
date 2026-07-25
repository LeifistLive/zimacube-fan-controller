package webui

const IndexHTML = `<!doctype html>
<html lang="de">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>ZimaCube Fan Controller</title>
<style>
:root{color-scheme:dark;font-family:Inter,system-ui,sans-serif;background:#111;color:#eee}
body{margin:0;padding:24px;max-width:920px;margin:auto}
h1{font-size:1.7rem;margin-bottom:20px}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(190px,1fr));gap:12px}
.card{background:#1c1c1c;border:1px solid #333;border-radius:14px;padding:16px}
.label{color:#aaa;font-size:.85rem}.value{font-size:1.55rem;font-weight:700;margin-top:6px}
.controls{display:flex;gap:10px;flex-wrap:wrap;margin-top:18px}
button,input{border-radius:10px;border:1px solid #444;background:#222;color:#fff;padding:10px 12px}
button{cursor:pointer}button:hover{background:#303030}
input[type=number]{width:90px}.wide{width:240px}
.ok{color:#7ee787}.bad{color:#ff7b72}pre{white-space:pre-wrap}
</style>
</head>
<body>
<h1>ZimaCube Fan Controller</h1>
<div class="grid">
  <div class="card"><div class="label">Modus</div><div class="value" id="mode">–</div></div>
  <div class="card"><div class="label">Lüfter</div><div class="value" id="fan">–</div></div>
  <div class="card"><div class="label">Max. HDD</div><div class="value" id="temp">–</div></div>
  <div class="card"><div class="label">Array</div><div class="value" id="array">–</div></div>
  <div class="card"><div class="label">Controller</div><div class="value" id="controller">–</div></div>
</div>

<div class="card" style="margin-top:12px">
  <div class="label">Grund</div>
  <div id="reason" style="margin-top:6px">–</div>
</div>

<div class="controls">
  <input id="percent" type="number" min="1" max="100" value="75">
  <button onclick="setManual()">Manuell setzen</button>
  <button onclick="post('/api/mode/auto')">Automatik</button>
  <button onclick="post('/api/mode/emergency')">Notfall</button>
  <input id="token" class="wide" type="password" placeholder="API-Token (optional)">
</div>

<pre id="message"></pre>

<script>
async function refresh(){
  try{
    const r=await fetch('/api/status',{cache:'no-store'});
    const d=await r.json(), s=d.status;
    document.getElementById('mode').textContent=s.mode;
    document.getElementById('fan').textContent=s.fan_percent+' %';
    document.getElementById('temp').textContent=s.maximum_disk_temperature+' °C';
    document.getElementById('array').textContent=s.array_operation;
    document.getElementById('reason').textContent=s.reason;
    const c=document.getElementById('controller');
    c.textContent=s.controller_online?'online':'offline';
    c.className='value '+(s.controller_online?'ok':'bad');
  }catch(e){document.getElementById('message').textContent=e}
}
async function post(path){
  const token=document.getElementById('token').value;
  const r=await fetch(path,{method:'POST',headers:token?{'X-API-Token':token}:{}});
  document.getElementById('message').textContent=await r.text();
  setTimeout(refresh,500);
}
function setManual(){
  const p=document.getElementById('percent').value;
  post('/api/fan/'+encodeURIComponent(p));
}
refresh();setInterval(refresh,5000);
</script>
</body>
</html>`
