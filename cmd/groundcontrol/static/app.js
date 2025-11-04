const API = window.API_HOST || 'http://localhost:8080'
const WS = API.replace(/^http/)

function el(tag, cls) {
  const e = document.createElement(tag)
  if (cls) e.className = cls
  return e
}

function renderRovers(list) {
  const container = document.getElementById('rovers')
  container.innerHTML = ''
  list.forEach(r => {
    const c = el('div')
    c.innerHTML = `<strong>Rover ${r.rover_id}</strong> <span class="small"> ${r.connected ? '✅' : '❌'}</span>`
    const pre = el('pre')
    pre.textContent = JSON.stringify(r.telemetry || {}, null, 2)
    c.appendChild(pre)
    container.appendChild(c)
  })
}

function renderMissions(list) {
  const container = document.getElementById('missions')
  container.innerHTML = ''
  list.forEach(m => {
    const c = el('div')
    c.innerHTML = `<strong>${m.ID}</strong><div class="small">Task: ${m.Task || m.task} • Status: ${m.Status || m.status} • Progress: ${Math.round((m.Progress||0)*100)}%</div>`
    container.appendChild(c)
  })
}

async function fetchAll() {
  try {
    const r1 = await fetch(API + '/rovers')
    const rovers = await r1.json()
    renderRovers(rovers)
  } catch(e){ console.warn('rovers fetch failed', e) }

  try {
    const r2 = await fetch(API + '/missions')
    const missions = await r2.json()
    renderMissions(missions)
  } catch(e){ console.warn('missions fetch failed', e) }
}

// function connectWS() {
//   const ws = new WebSocket(WS + '/ws')
//   ws.onopen = () => console.info('ws connected')
//   ws.onmessage = (ev) => {
//     try {
//       const msg = JSON.parse(ev.data)
//       if (msg.type === 'telemetry') {
//         fetch(API + '/api/rovers').then(r=>r.json()).then(renderRovers).catch(()=>{})
//       } else if (msg.type === 'mission') {
//         fetch(API + '/api/missions').then(r=>r.json()).then(renderMissions).catch(()=>{})
//       }
//     } catch(e) { console.warn('ws msg parse', e) }
//   }
//   ws.onclose = () => setTimeout(connectWS, 1500)
//   ws.onerror = (e) => console.warn('ws error', e)
// }

fetchAll()
// connectWS()
setInterval(fetchAll, 15_000)
