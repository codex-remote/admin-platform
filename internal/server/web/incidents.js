import {api, escapeHTML, timeLabel} from "./ui.js";

const incidentState = {items: [], selected: null, getSelectedEvent: () => null, switchView: () => {}};
const $ = selector => document.querySelector(selector);

export function initIncidents(options) {
  incidentState.getSelectedEvent = options.getSelectedEvent;
  incidentState.switchView = options.switchView;
  $("#newIncidentButton").addEventListener("click", openIncidentDialog);
  $("#incidentCancel").addEventListener("click", () => $("#incidentDialog").close());
  $("#incidentForm").addEventListener("submit", createIncident);
}

export function incidentActionHTML() {
  return `<button id="createIncidentFromEvent" class="detail-command">建立事故</button>`;
}

export function bindIncidentAction() {
  $("#createIncidentFromEvent")?.addEventListener("click", openIncidentDialog);
}

export async function loadIncidents() {
  try {
    const payload = await api("/api/v1/incidents?limit=100");
    incidentState.items = payload.incidents || [];
    renderIncidentList();
    if (incidentState.selected) await selectIncident(incidentState.selected.incident.id);
  } catch (error) {
    $("#incidentList").innerHTML = `<div class="upload-status">${escapeHTML(error.message)}</div>`;
  }
}

function openIncidentDialog() {
  const event = incidentState.getSelectedEvent();
  if (!event) return;
  $("#incidentAnchorEventID").value = event.id;
  $("#incidentAnchorSummary").textContent = `${timeLabel(event.timestamp)} · ${event.source} · ${event.event}`;
  $("#incidentTitle").value = event.event.includes("memory") ? "iPhone 内存异常" : event.event;
  $("#incidentSummary").value = event.message || "";
  $("#incidentSeverity").value = ["error", "fault"].includes(event.level) ? "critical" : event.level === "warning" ? "warning" : "info";
  $("#incidentFormError").textContent = "";
  $("#incidentDialog").showModal();
}

async function createIncident(event) {
  event.preventDefault();
  const submit = $("#incidentSubmit");
  submit.disabled = true;
  try {
    const snapshot = await api("/api/v1/incidents", {method:"POST", headers:{"Content-Type":"application/json"}, body:JSON.stringify({
      title: $("#incidentTitle").value.trim(), summary: $("#incidentSummary").value.trim(), severity: $("#incidentSeverity").value,
      anchor_event_id: Number($("#incidentAnchorEventID").value), before_seconds: Number($("#incidentBefore").value), after_seconds: Number($("#incidentAfter").value)
    })});
    $("#incidentDialog").close();
    incidentState.selected = snapshot;
    incidentState.switchView("incidents");
  } catch (error) {
    $("#incidentFormError").textContent = error.message;
  } finally {
    submit.disabled = false;
  }
}

function renderIncidentList() {
  $("#incidentCount").textContent = incidentState.items.length;
  $("#incidentList").innerHTML = incidentState.items.length ? incidentState.items.map(item => `
    <button class="incident-row ${incidentState.selected?.incident.id === item.id ? "selected" : ""}" data-incident-id="${item.id}">
      <span class="severity-mark ${escapeHTML(item.severity)}"></span>
      <span><strong>${escapeHTML(item.title)}</strong><small>${escapeHTML(timeLabel(item.created_at))} · ${item.event_count} 条事件 · ${item.artifact_count} 个附件</small></span>
      <em>${escapeHTML(item.status)}</em>
    </button>`).join("") : `<div class="empty-state incident-empty"><strong>暂无事故</strong><span>从事件详情建立第一份固定证据快照。</span></div>`;
  document.querySelectorAll("[data-incident-id]").forEach(row => row.addEventListener("click", () => selectIncident(Number(row.dataset.incidentId))));
}

async function selectIncident(id) {
  try {
    incidentState.selected = await api(`/api/v1/incidents/${id}`);
    renderIncidentList();
    renderIncidentDetail(incidentState.selected);
  } catch (error) {
    $("#incidentDetail").innerHTML = `<div class="upload-status">${escapeHTML(error.message)}</div>`;
  }
}

function renderIncidentDetail(snapshot) {
  const item = snapshot.incident;
  const counts = Object.entries(snapshot.provenance.source_counts || {}).map(([source,count]) => `<span>${escapeHTML(source)} <b>${count}</b></span>`).join("");
  $("#incidentDetail").innerHTML = `
    <div class="incident-header"><div><span class="severity-label ${escapeHTML(item.severity)}">${escapeHTML(item.severity)}</span><h2>${escapeHTML(item.title)}</h2><p>${escapeHTML(item.summary || "无补充说明")}</p></div><a class="snapshot-download" href="/api/v1/incidents/${item.id}/snapshot">下载 JSON</a></div>
    <div class="incident-facts"><div><b>证据窗口</b><code>${escapeHTML(item.window_start)}<br>${escapeHTML(item.window_end)}</code></div><div><b>快照</b><code>${item.event_count} events / ${item.artifact_count} artifacts${item.truncated ? " / truncated" : ""}</code></div><div><b>关联</b><code>${escapeHTML(item.trace_id || item.turn_ref || item.session_id || "-")}</code></div></div>
    <div class="source-counts">${counts}</div>
    <div class="provenance-line">${escapeHTML(snapshot.schema_version)} · ${escapeHTML(snapshot.provenance.data_source)} · ${escapeHTML(snapshot.provenance.database_timezone)} · ${escapeHTML(snapshot.provenance.privacy_policy)}</div>
    <div class="incident-timeline">${(snapshot.events || []).map(event => `<div class="timeline-event"><time>${escapeHTML(timeLabel(event.timestamp))}</time><span class="level ${escapeHTML(event.level)}">${escapeHTML(event.level)}</span><span><strong>${escapeHTML(event.event)}</strong><small>${escapeHTML(event.source)}${event.profile ? ` · ${escapeHTML(event.profile)}` : ""}</small></span></div>`).join("")}</div>`;
}
