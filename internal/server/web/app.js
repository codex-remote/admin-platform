import {api, escapeHTML, sizeLabel, short, timeLabel} from "./ui.js";
import {bindIncidentAction, incidentActionHTML, initIncidents, loadIncidents} from "./incidents.js";

const state = { source: "", profile: "", level: "", search: "", events: [], selected: null, view: "events" };
const $ = selector => document.querySelector(selector);
const $$ = selector => [...document.querySelectorAll(selector)];

async function loadOverview() {
  const data = await api("/api/overview"); const counts = data.by_source || {};
  $("#countAll").textContent = data.total_events || 0; $("#countPhone").textContent = counts["iphone-app"] || 0;
  $("#countSystem").textContent = counts["iphone-system"] || 0; $("#countRelay").textContent = counts["relay-server"] || 0; $("#countAgent").textContent = counts["mac-agent"] || 0;
  $("#countCollector").textContent = counts["diagnostics-collector"] || 0;
}

async function loadServices() {
  try {
    const payload = await api("/api/services"); const services = payload.services || [];
    $("#serviceStrip").innerHTML = services.map(service => `<div class="service-pill ${escapeHTML(service.status)}"><i class="dot"></i><div><strong>${escapeHTML(service.name)}</strong> <span>${escapeHTML(service.profile)}</span></div><em>${service.latency_ms}ms</em></div>`).join("");
  } catch (error) { toast(error.message); }
}

async function loadEvents() {
  const params = new URLSearchParams({limit:"300"});
  if (state.source) params.set("source", state.source); if (state.profile) params.set("profile", state.profile);
  if (state.level) params.set("level", state.level); if (state.search) params.set("q", state.search);
  try {
    const data = await api(`/api/events?${params}`);
    const selectedID = state.selected?.id;
    state.events = data.events || [];
    if (selectedID) state.selected = state.events.find(event => event.id === selectedID) || state.selected;
    renderEvents();
  } catch (error) { toast(error.message); }
}

function renderEvents() {
  $("#resultCount").textContent = `${state.events.length} 条`;
  $("#eventEmpty").classList.toggle("hidden", state.events.length > 0);
  const body = $("#eventRows");
  const retained = new Set();
  let cursor = body.firstElementChild;
  state.events.forEach(event => {
    retained.add(String(event.id));
    let row = body.querySelector(`tr[data-id="${event.id}"]`);
    const correlation = event.trace_id || event.turn_ref || event.session_id;
    if (!row) {
      row = document.createElement("tr");
      row.dataset.id = event.id;
      row.innerHTML = `<td class="time-cell">${escapeHTML(timeLabel(event.timestamp))}</td><td><span class="level ${escapeHTML(event.level)}">${escapeHTML(event.level)}</span></td><td><span class="source-label ${escapeHTML(event.source)}"><i></i>${escapeHTML(event.source)}${event.profile ? ` · ${escapeHTML(event.profile)}` : ""}</span></td><td><span class="event-name">${escapeHTML(event.event)}</span>${event.message ? `<br><span>${escapeHTML(event.message)}</span>` : ""}</td><td class="correlation">${escapeHTML(short(correlation))}</td>`;
    }
    row.classList.toggle("selected", state.selected?.id === event.id);
    if (row !== cursor) body.insertBefore(row, cursor);
    cursor = row.nextElementSibling;
  });
  [...body.children].forEach(row => { if (!retained.has(row.dataset.id)) row.remove(); });
}

function selectEvent(id) {
  state.selected = state.events.find(event => event.id === id); renderEvents(); if (!state.selected) return;
  const e = state.selected; const fields = typeof e.fields === "object" ? e.fields : {};
  $("#eventDetail").innerHTML = `<div class="detail-header"><div class="detail-title-row"><span class="level ${escapeHTML(e.level)}">${escapeHTML(e.level)}</span>${incidentActionHTML()}</div><h2>${escapeHTML(e.event)}</h2><p>${escapeHTML(e.message || `${e.source}${e.profile ? ` / ${e.profile}` : ""}`)}</p></div><div class="detail-section"><h3>时间与来源</h3>${kv("timestamp",e.timestamp)}${kv("received",e.received_at)}${kv("source",e.source)}${kv("profile",e.profile||"-")}${kv("category",e.category)}</div><div class="detail-section"><h3>关联</h3>${kv("session_id",e.session_id||"-")}${kv("sequence",e.sequence||"-")}${kv("trace_id",e.trace_id||"-")}${kv("turn_ref",e.turn_ref||"-")}${kv("artifact_id",e.artifact_id||"-")}</div><div class="detail-section"><h3>结构化字段</h3><pre class="field-json">${escapeHTML(JSON.stringify(fields,null,2))}</pre></div>`;
  bindIncidentAction();
}
function kv(key,value){return `<div class="kv"><b>${escapeHTML(key)}</b><code>${escapeHTML(value)}</code></div>`}

async function loadDevices() {
  $("#deviceList").innerHTML = `<div class="upload-status">正在读取 CoreDevice…</div>`;
  try { const payload=await api("/api/devices"); const devices=payload.devices||[]; $("#deviceCount").textContent=devices.length; $("#deviceList").innerHTML=devices.length?devices.map(device=>`<div class="device-row"><div><strong>${escapeHTML(device.name)} · ${escapeHTML(device.marketing_name)}</strong><span>iOS ${escapeHTML(device.os_version)} (${escapeHTML(device.os_build)}) · ${escapeHTML(device.product_type)}<br>${escapeHTML(device.id)}</span></div><div class="device-actions"><button data-device='${escapeHTML(JSON.stringify(device))}' data-full="false">标准采集</button><button class="primary" data-device='${escapeHTML(JSON.stringify(device))}' data-full="true">完整日志</button></div></div>`).join(""):`<div class="upload-status">没有发现已配对 iPhone。</div>`; $$("[data-device]").forEach(button=>button.addEventListener("click",()=>startCapture(JSON.parse(button.dataset.device),button.dataset.full==="true"))); } catch(error){$("#deviceList").innerHTML=`<div class="upload-status">${escapeHTML(error.message)}</div>`}
}

async function startCapture(device, fullLogs) {
  if (!confirm(`${fullLogs?"完整":"标准"}采集将请求 ${device.name} 生成 sysdiagnose，是否继续？`)) return;
  try { await api("/api/captures",{method:"POST",headers:{"Content-Type":"application/json"},body:JSON.stringify({device_id:device.id,device_name:device.name,full_logs:fullLogs})}); toast("采集任务已创建"); loadCaptures(); } catch(error){toast(error.message)}
}

async function loadCaptures(){try{const payload=await api("/api/captures");const captures=payload.captures||[];$("#captureCount").textContent=captures.length;$("#captureList").innerHTML=captures.length?captures.map(c=>`<div class="capture-row"><span class="capture-status ${escapeHTML(c.status)}">${escapeHTML(c.status)}</span><div><strong>${escapeHTML(c.device_name)}</strong><span>${escapeHTML(c.created_at)}${c.error?` · ${escapeHTML(c.error)}`:""}</span></div><span>${c.full_logs?"完整日志":"标准采集"}${c.artifact_id?` · #${c.artifact_id}`:""}</span></div>`).join(""):`<div class="upload-status">暂无采集任务。</div>`}catch(error){toast(error.message)}}

async function loadArtifacts(){try{const payload=await api("/api/artifacts");const artifacts=payload.artifacts||[];$("#artifactRows").innerHTML=artifacts.map(a=>`<tr><td>${escapeHTML(timeLabel(a.created_at))}</td><td>${escapeHTML(a.source)}</td><td>${escapeHTML(a.kind)}</td><td>${escapeHTML(a.name)}</td><td>${escapeHTML(sizeLabel(a.size_bytes))}</td><td><code>${escapeHTML(short(a.sha256,12))}</code></td><td><a href="/api/artifacts/${a.id}">下载</a></td></tr>`).join("")}catch(error){toast(error.message)}}

async function uploadArtifact(file){const body=new FormData();body.append("file",file);$("#uploadStatus").textContent="正在保存并解析…";try{const result=await api("/api/artifacts",{method:"POST",body});$("#uploadStatus").textContent=`已导入 ${result.inserted} 条新事件`;toast("诊断包已入库");await Promise.all([loadArtifacts(),loadEvents(),loadOverview()])}catch(error){$("#uploadStatus").textContent=error.message;toast(error.message)}}

function switchView(view){state.view=view;$$('.tab').forEach(tab=>tab.classList.toggle('active',tab.dataset.view===view));$$('.view').forEach(panel=>panel.classList.remove('active'));$(`#${view}View`).classList.add('active');if(view==='incidents')loadIncidents();if(view==='captures'){loadDevices();loadCaptures()}if(view==='artifacts')loadArtifacts()}
let toastTimer;function toast(message){const node=$("#toast");node.textContent=message;node.classList.add("show");clearTimeout(toastTimer);toastTimer=setTimeout(()=>node.classList.remove("show"),3500)}
function debounce(fn,delay){let timer;return(...args)=>{clearTimeout(timer);timer=setTimeout(()=>fn(...args),delay)}}
const refreshEventsSoon=debounce(loadEvents,500);

$$('.tab').forEach(tab=>tab.addEventListener('click',()=>switchView(tab.dataset.view)));
$("#eventRows").addEventListener("click", event => { const row=event.target.closest("tr[data-id]"); if(row) selectEvent(Number(row.dataset.id)); });
$$('.source').forEach(button=>button.addEventListener('click',()=>{$$('.source').forEach(item=>item.classList.remove('active'));button.classList.add('active');state.source=button.dataset.source;state.selected=null;loadEvents()}));
$$('input[name="profile"]').forEach(input=>input.addEventListener('change',()=>{state.profile=input.value;loadEvents()}));
$("#levelFilter").addEventListener("change",event=>{state.level=event.target.value;loadEvents()});
$("#searchInput").addEventListener("input",debounce(event=>{state.search=event.target.value.trim();loadEvents()},250));
$("#clearFilters").addEventListener("click",()=>{state.source="";state.profile="";state.level="";state.search="";$("#searchInput").value="";$("#levelFilter").value="";$$('.source').forEach((item,index)=>item.classList.toggle('active',index===0));$('input[name="profile"][value=""]').checked=true;loadEvents()});
$("#refreshButton").addEventListener("click",()=>Promise.all([loadOverview(),loadServices(),loadEvents()]));
$("#loadDevices").addEventListener("click",loadDevices);$("#artifactUpload").addEventListener("change",event=>{if(event.target.files[0])uploadArtifact(event.target.files[0])});

initIncidents({getSelectedEvent:()=>state.selected,switchView});
const stream=new EventSource('/api/stream');stream.addEventListener('changed',()=>{loadOverview();if(state.view==='events')refreshEventsSoon();if(state.view==='incidents')loadIncidents();if(state.view==='captures')loadCaptures();if(state.view==='artifacts')loadArtifacts()});stream.onopen=()=>$("#liveState").classList.remove("off");stream.onerror=()=>$("#liveState").classList.add("off");
Promise.all([loadOverview(),loadServices(),loadEvents()]);setInterval(loadServices,10000);
