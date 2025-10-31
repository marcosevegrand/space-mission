// connect to API websocket and update page automatically
(function () {
  const WS_PATH = "/ws";
  const API_TELEMETRY = "/api/telemetry";
  const API_MISSIONS = "/api/missions";

  // DOM containers (create these IDs in your HTML)
  const telemetryContainerId = "telemetry-list";
  const missionsContainerId = "missions-list";

  function log(...args) { console.log("[WS]", ...args); }

  // render helpers
  function renderTelemetry(list) {
    const el = document.getElementById(telemetryContainerId);
    if (!el) return;
    el.innerHTML = "";
    list.forEach(t => {
      const row = document.createElement("div");
      row.className = "telemetry-row";
      row.textContent = `Rover ${t.rover_id || t.RoverID}: pos=(${t.position?.x ?? t.Position?.X},${t.position?.y ?? t.Position?.Y}) bat=${t.battery_level ?? t.BatteryLevel}%`;
      el.appendChild(row);
    });
  }

  function renderMissions(list) {
    const el = document.getElementById(missionsContainerId);
    if (!el) return;
    el.innerHTML = "";
    list.forEach(m => {
      const row = document.createElement("div");
      row.className = "mission-row";
      row.textContent = `${m.id || m.ID} — ${m.task || m.Task} — ${Math.round((m.progress||m.Progress||0)*100)}% (${m.status || m.Status})`;
      el.appendChild(row);
    });
  }

  // initial fetch to populate page
  async function initialLoad() {
    try {
      const [tRes, mRes] = await Promise.all([fetch(API_TELEMETRY), fetch(API_MISSIONS)]);
      if (tRes.ok) renderTelemetry(await tRes.json());
      if (mRes.ok) renderMissions(await mRes.json());
    } catch (e) {
      log("initial load failed:", e);
    }
  }

  // websocket with reconnect
  function startWebSocket() {
    const proto = (location.protocol === "https:") ? "wss:" : "ws:";
    const wsUrl = `${proto}//${location.host}${WS_PATH}`;
    let backoff = 1000;

    function connect() {
      const ws = new WebSocket(wsUrl);
      ws.onopen = () => {
        log("connected");
        backoff = 1000;
      };
      ws.onmessage = (ev) => {
        try {
          const msg = JSON.parse(ev.data);
          switch (msg.type) {
            case "telemetry_snapshot":
              renderTelemetry(msg.telemetry || msg);
              break;
            case "telemetry":
              // for single telemetry messages, merge/update list: easiest is to re-fetch
              initialLoad();
              break;
            case "missions_snapshot":
              renderMissions(msg.missions || msg);
              break;
            case "mission":
              initialLoad();
              break;
            default:
              // ignore unknown
              break;
          }
        } catch (err) {
          log("invalid message", err);
        }
      };
      ws.onclose = () => {
        log("disconnected; reconnecting in", backoff, "ms");
        setTimeout(() => {
          backoff = Math.min(backoff * 1.5, 30000);
          connect();
        }, backoff);
      };
      ws.onerror = (e) => {
        log("ws error", e);
        ws.close();
      };
    }

    connect();
  }

  // bootstrap
  document.addEventListener("DOMContentLoaded", () => {
    initialLoad();
    startWebSocket();
  });
})();