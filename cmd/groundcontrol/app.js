class GroundControl {
  constructor() {
    this.apiUrl = "http://localhost:8080"
    this.rovers = []
    this.missions = []
    this.selectedRover = null
    this.updateInterval = null
    this.canvas = document.getElementById("mapCanvas")
    this.ctx = this.canvas.getContext("2d")

    this.initEventListeners()
    this.resizeCanvas()
    window.addEventListener("resize", () => this.resizeCanvas())
  }

  initEventListeners() {
    document.getElementById("connectBtn").addEventListener("click", () => this.connect())
    document.getElementById("apiUrl").addEventListener("change", (e) => {
      this.apiUrl = e.target.value
    })
  }

  resizeCanvas() {
    const rect = this.canvas.parentElement.getBoundingClientRect()
    this.canvas.width = rect.width - 40
    this.canvas.height = 400
  }

  connect() {
    console.log("[v0] Conectando à API:", this.apiUrl)
    this.updateData()
    this.startAutoUpdate()
  }

  startAutoUpdate() {
    if (this.updateInterval) clearInterval(this.updateInterval)
    this.updateInterval = setInterval(() => this.updateData(), 3000)
  }

  async updateData() {
    try {
      // Buscar dados de rovers
      await this.fetchRovers()

      // Buscar dados de missões
      await this.fetchMissions()

      // Atualizar UI
      this.renderRovers()
      this.renderMissions()
      this.drawMap()
      this.updateTelemetry()

      this.setConnectionStatus(true)
      this.updateTimestamp()
    } catch (error) {
      console.error("[v0] Erro ao atualizar dados:", error)
      this.setConnectionStatus(false)
    }
  }

  async fetchRovers() {
    try {
      const response = await fetch(`${this.apiUrl}/api/rovers`)

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      const apiResponse = await response.json()
      if (apiResponse.success && Array.isArray(apiResponse.data)) {
        this.rovers = apiResponse.data.map((telemetry) => ({
          id: telemetry.id,
          position: telemetry.position,
          battery: telemetry.battery,
          temperature: telemetry.temperature,
          signal: 95, // Will improve this later
          status: telemetry.operational_state === "on_mission" ? "online" : "online",
          velocity: telemetry.velocity,
          operationalState: telemetry.operational_state,
          systemHealth: telemetry.system_health,
        }))
        console.log("[v0] Rovers received via HTTP API:", this.rovers)
      } else {
        throw new Error("Invalid API response format")
      }
    } catch (error) {
      console.error("[v0] Error fetching rovers:", error)
      this.rovers = this.getMockRovers()
    }
  }

  async fetchMissions() {
    try {
      const response = await fetch(`${this.apiUrl}/api/missions`)

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`)
      }

      const apiResponse = await response.json()
      if (apiResponse.success && Array.isArray(apiResponse.data)) {
        this.missions = apiResponse.data.map((assignment) => ({
          id: assignment.mission_id,
          task: assignment.task,
          status:
            assignment.status === "in_progress"
              ? "In Progress"
              : assignment.status === "completed"
                ? "Completed"
                : assignment.status === "failed"
                  ? "Failed"
                  : assignment.status === "assigned"
                    ? "Assigned"
                    : "Unassigned",
          progress: assignment.progress,
          area: this.parseArea(assignment.area),
        }))
        console.log("[v0] Missions received via HTTP API:", this.missions)
      } else {
        throw new Error("Invalid API response format")
      }
    } catch (error) {
      console.error("[v0] Error fetching missions:", error)
      this.missions = this.getMockMissions()
    }
  }

  parseArea(area) {
    if (!area) return null

    // Handle circle
    if (area.shape === "circle" || area.shape === 1) {
      return {
        type: "circle",
        center: area.coords?.center || area.coords?.Center || { x: 0, y: 0 },
        radius: area.coords?.radius || area.coords?.Radius || 30,
      }
    }

    // Handle rectangle
    if (area.shape === "rectangle" || area.shape === 2) {
      const topLeft = area.coords?.topLeft || area.coords?.TopLeft || { x: 0, y: 0 }
      const bottomRight = area.coords?.bottomRight || area.coords?.BottomRight || { x: 100, y: -100 }
      const centerX = (topLeft.x + bottomRight.x) / 2
      const centerY = (topLeft.y + bottomRight.y) / 2

      return {
        type: "rectangle",
        topLeft,
        bottomRight,
        center: { x: centerX, y: centerY },
      }
    }

    return null
  }

  renderRovers() {
    const roversList = document.getElementById("roversList")
    roversList.innerHTML = ""

    this.rovers.forEach((rover) => {
      const card = document.createElement("div")
      card.className = "rover-card"

      const status = rover.status === "online" ? "Online" : "Offline"
      const statusClass = rover.status === "online" ? "" : "offline"

      card.innerHTML = `
                <div class="rover-header">
                    <span class="rover-id">Rover #${rover.id}</span>
                    <span class="rover-status ${statusClass}">${status}</span>
                </div>
                <div class="rover-stats">
                    <div class="stat">
                        <span class="stat-label">Bateria:</span>
                        <span class="stat-value">${rover.battery}%</span>
                    </div>
                    <div class="stat">
                        <span class="stat-label">Temp:</span>
                        <span class="stat-value">${rover.temperature}°C</span>
                    </div>
                    <div class="stat">
                        <span class="stat-label">Posição:</span>
                        <span class="stat-value">(${rover.position.x.toFixed(2)}, ${rover.position.y.toFixed(2)})</span>
                    </div>
                    <div class="stat">
                        <span class="stat-label">Sinal:</span>
                        <span class="stat-value">${rover.signal}%</span>
                    </div>
                </div>
            `

      roversList.appendChild(card)
    })
  }

  renderMissions() {
    const missionsList = document.getElementById("missionsList")
    missionsList.innerHTML = ""

    this.missions.forEach((mission) => {
      const card = document.createElement("div")
      card.className = "mission-card"

      const statusClass = mission.status.toLowerCase().replace(/\s+/g, "-")

      card.innerHTML = `
                <div class="mission-id">Missão #${mission.id}</div>
                <div class="mission-task">${mission.task}</div>
                <span class="mission-status ${statusClass}">${mission.status}</span>
                <div class="progress-bar">
                    <div class="progress-fill" style="width: ${mission.progress}%"></div>
                </div>
            `

      missionsList.appendChild(card)
    })
  }

  drawMap() {
    const w = this.canvas.width
    const h = this.canvas.height

    // Limpar canvas
    this.ctx.fillStyle = "#0a0e27"
    this.ctx.fillRect(0, 0, w, h)

    // Grade
    this.ctx.strokeStyle = "#00aa0033"
    this.ctx.lineWidth = 1
    for (let i = 0; i < w; i += 50) {
      this.ctx.beginPath()
      this.ctx.moveTo(i, 0)
      this.ctx.lineTo(i, h)
      this.ctx.stroke()
    }
    for (let i = 0; i < h; i += 50) {
      this.ctx.beginPath()
      this.ctx.moveTo(0, i)
      this.ctx.lineTo(w, i)
      this.ctx.stroke()
    }

    // Desenhar missões (áreas)
    this.missions.forEach((mission) => {
      this.drawMissionArea(mission)
    })

    // Desenhar rovers
    this.rovers.forEach((rover, index) => {
      this.drawRover(rover, index)
    })
  }

  drawMissionArea(mission) {
    if (!mission.area) return

    const centerX = (mission.area.center?.x || 0) + this.canvas.width / 2
    const centerY = (mission.area.center?.y || 0) + this.canvas.height / 2
    const radius = mission.area.radius || 30

    this.ctx.fillStyle = "#00ff4133"
    this.ctx.strokeStyle = "#00ff41"
    this.ctx.lineWidth = 2
    this.ctx.beginPath()
    this.ctx.arc(centerX, centerY, radius, 0, Math.PI * 2)
    this.ctx.fill()
    this.ctx.stroke()
  }

  drawRover(rover, index) {
    const x = (rover.position?.x || 0) + this.canvas.width / 2
    const y = (rover.position?.y || 0) + this.canvas.height / 2

    const isOnline = rover.status === "online"
    this.ctx.fillStyle = isOnline ? "#00ccff" : "#ff0000"
    this.ctx.strokeStyle = isOnline ? "#00ffff" : "#ff6666"
    this.ctx.lineWidth = 2

    // Desenhar rover como quadrado
    this.ctx.fillRect(x - 8, y - 8, 16, 16)
    this.ctx.strokeRect(x - 8, y - 8, 16, 16)

    // Label
    this.ctx.fillStyle = "#00ff41"
    this.ctx.font = "12px Courier New"
    this.ctx.fillText(`R${rover.id}`, x + 10, y)
  }

  updateTelemetry() {
    const telemetryDiv = document.getElementById("telemetryData")

    if (this.rovers.length === 0) {
      telemetryDiv.innerHTML = "<p>Sem dados de telemetria</p>"
      return
    }

    const rover = this.rovers[0]
    telemetryDiv.innerHTML = `
            <div class="telemetry-item">
                <div class="telemetry-label">Rover Ativo</div>
                <div class="telemetry-value">Rover #${rover.id}</div>
            </div>
            <div class="telemetry-item">
                <div class="telemetry-label">Bateria</div>
                <div class="telemetry-value">${rover.battery}%</div>
            </div>
            <div class="telemetry-item">
                <div class="telemetry-label">Temperatura</div>
                <div class="telemetry-value">${rover.temperature}°C</div>
            </div>
            <div class="telemetry-item">
                <div class="telemetry-label">Sinal</div>
                <div class="telemetry-value">${rover.signal}%</div>
            </div>
        `
  }

  updateTimestamp() {
    const now = new Date()
    document.getElementById("lastUpdate").textContent = now.toLocaleTimeString("pt-PT")
  }

  setConnectionStatus(connected) {
    const indicator = document.getElementById("connectionStatus")
    const text = document.getElementById("connectionText")

    if (connected) {
      indicator.classList.add("connected")
      text.textContent = "Conectado"
    } else {
      indicator.classList.remove("connected")
      text.textContent = "Desconectado"
    }
  }

  // Dados simulados para teste
  getMockRovers() {
    return [
      {
        id: 1,
        position: { x: 10, y: 10 },
        battery: 85,
        temperature: 45,
        signal: 95,
        status: "online",
      },
      {
        id: 2,
        position: { x: -50, y: 30 },
        battery: 60,
        temperature: 52,
        signal: 80,
        status: "online",
      },
    ]
  }

  getMockMissions() {
    return [
      {
        id: 1,
        task: "Sample Analysis",
        status: "In Progress",
        progress: 45,
        area: { center: { x: 10, y: 10 }, radius: 20 },
      },
      {
        id: 2,
        task: "Image Capture",
        status: "Unassigned",
        progress: 0,
        area: { center: { x: 0, y: -100 }, radius: 30 },
      },
    ]
  }
}

// Inicializar quando página carrega
document.addEventListener("DOMContentLoaded", () => {
  const groundControl = new GroundControl()
})
