document.addEventListener("DOMContentLoaded", () => {
    const apiUrlInput = document.getElementById("apiUrl") || { value: "http://localhost:8080" }
    const roversList = document.getElementById("roversList")
    const missionsList = document.getElementById("missionsList") || { innerHTML: "" }
    const lastUpdate = document.getElementById("lastUpdate")
    const activeRovers = document.getElementById("activeRovers")
    const activeMissions = document.getElementById("activeMissions")
    const systemHealth = document.getElementById("systemHealth")

    function operationalStateToString(state) {
        switch (state) {
            case 1: return "Idle"
            case 2: return "On Mission"
            case 3: return "Error"
            default: return "Unknown"
        }
    }

    // --- Map setup (safe integration) ---
    let map = null
    let roverMarkers = []
    let mapReady = false

    function initMap() {
        if (typeof L !== "undefined" && document.getElementById("map")) {
            map = L.map('map').setView([0, 0], 2)
            L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
                attribution: '© OpenStreetMap contributors'
            }).addTo(map)
            mapReady = true
        }
    }

    initMap()

    // --- Canvas grid setup ---
    const canvas = document.getElementById("mapCanvas")
    const GRID_SIZE = 10 // 10x10 grid
    const CELL_SIZE = 40 // px
    const CANVAS_SIZE = GRID_SIZE * CELL_SIZE

    if (canvas) {
        canvas.width = CANVAS_SIZE
        canvas.height = CANVAS_SIZE
    }

    function renderCanvasGrid(rovers, missions) {
        if (!canvas) return
        const ctx = canvas.getContext("2d")
        ctx.clearRect(0, 0, CANVAS_SIZE, CANVAS_SIZE)

        // --- Compute bounds ---
        let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity
        rovers.forEach(r => {
            if (r.Position) {
                minX = Math.min(minX, r.Position.X)
                maxX = Math.max(maxX, r.Position.X)
                minY = Math.min(minY, r.Position.Y)
                maxY = Math.max(maxY, r.Position.Y)
            }
        })
        missions.forEach(m => {
            if (m.Area && m.Area.Coords && m.Area.Coords.Center) {
                minX = Math.min(minX, m.Area.Coords.Center.X)
                maxX = Math.max(maxX, m.Area.Coords.Center.X)
                minY = Math.min(minY, m.Area.Coords.Center.Y)
                maxY = Math.max(maxY, m.Area.Coords.Center.Y)
            }
            if (m.Area && m.Area.Coords && m.Area.Coords.TopLeft) {
                minX = Math.min(minX, m.Area.Coords.TopLeft.X)
                maxX = Math.max(maxX, m.Area.Coords.BottomRight.X)
                minY = Math.min(minY, m.Area.Coords.TopLeft.Y)
                maxY = Math.max(maxY, m.Area.Coords.BottomRight.Y)
            }
        })
        // Add padding
        if (!isFinite(minX) || !isFinite(maxX) || !isFinite(minY) || !isFinite(maxY)) {
            minX = -200; maxX = 200; minY = -200; maxY = 200;
        }
        const padding = 20
        minX -= padding; maxX += padding; minY -= padding; maxY += padding;

        // --- Coordinate mapping ---
        function worldToCanvas(x, y) {
            const cx = ((x - minX) / (maxX - minX)) * CANVAS_SIZE
            const cy = CANVAS_SIZE - ((y - minY) / (maxY - minY)) * CANVAS_SIZE
            return [cx, cy]
        }

        // Draw grid
        ctx.strokeStyle = "#eee"
        for (let i = 0; i <= GRID_SIZE; i++) {
            ctx.beginPath()
            ctx.moveTo(0, i * CELL_SIZE)
            ctx.lineTo(CANVAS_SIZE, i * CELL_SIZE)
            ctx.stroke()
            ctx.beginPath()
            ctx.moveTo(i * CELL_SIZE, 0)
            ctx.lineTo(i * CELL_SIZE, CANVAS_SIZE)
            ctx.stroke()
        }

        // Plot mission zones
        missions.forEach(mission => {
            if (mission.Area && mission.Area.Shape === 1 && mission.Area.Coords && mission.Area.Coords.Center && mission.Area.Coords.Radius) {
                // Circle
                const center = mission.Area.Coords.Center
                const radius = mission.Area.Coords.Radius
                const [x, y] = worldToCanvas(center.X, center.Y)
                // Scale radius to canvas units
                const scale = CANVAS_SIZE / (maxX - minX)
                ctx.beginPath()
                ctx.arc(x, y, radius * scale, 0, 2 * Math.PI)
                ctx.strokeStyle = "#ff9800"
                ctx.lineWidth = 2
                ctx.stroke()
                ctx.fillStyle = "rgba(255,152,0,0.15)"
                ctx.fill()
                ctx.fillStyle = "#fff"
                ctx.font = "10px Arial"
                ctx.textAlign = "center"
                ctx.textBaseline = "top"
                ctx.fillText(`M${mission.MissionID}`, x, y + radius * scale + 2)
            }
            // Rectangle support (optional)
            if (mission.Area && mission.Area.Shape === 2 && mission.Area.Coords && mission.Area.Coords.TopLeft && mission.Area.Coords.BottomRight) {
                const tl = mission.Area.Coords.TopLeft
                const br = mission.Area.Coords.BottomRight
                const [x1, y1] = worldToCanvas(tl.X, tl.Y)
                const [x2, y2] = worldToCanvas(br.X, br.Y)
                ctx.beginPath()
                ctx.rect(x1, y1, x2 - x1, y2 - y1)
                ctx.strokeStyle = "#ff9800"
                ctx.lineWidth = 2
                ctx.stroke()
                ctx.fillStyle = "rgba(255,152,0,0.15)"
                ctx.fill()
                ctx.fillStyle = "#fff"
                ctx.font = "10px Arial"
                ctx.textAlign = "left"
                ctx.textBaseline = "top"
                ctx.fillText(`M${mission.MissionID}`, x1 + 5, y1 + 5)
            }
        })

        // Plot rovers
        rovers.forEach(rover => {
            if (rover.Position) {
                const [x, y] = worldToCanvas(rover.Position.X, rover.Position.Y)
                ctx.fillStyle = "#2196f3"
                ctx.beginPath()
                ctx.arc(x, y, CELL_SIZE / 6, 0, 2 * Math.PI)
                ctx.fill()
                ctx.strokeStyle = "#fff"
                ctx.lineWidth = 2
                ctx.stroke()
                ctx.fillStyle = "#fff"
                ctx.font = "10px Arial"
                ctx.textAlign = "center"
                ctx.textBaseline = "top"
                ctx.fillText(`R${rover.RoverID}`, x, y + CELL_SIZE / 6 + 2)
            }
        })
    }

    async function fetchRovers() {
        const apiUrl = apiUrlInput.value
        try {
            const response = await fetch(`${apiUrl}/api/rovers`)
            if (!response.ok) {
                console.error("Failed to fetch rovers:", response.status, response.statusText)
                throw new Error("Network response was not ok")
            }
            const apiResponse = await response.json()
            roversList.innerHTML = ""
            // Remove old markers only if map is ready
            if (mapReady) {
                roverMarkers.forEach(marker => map.removeLayer(marker))
                roverMarkers = []
            }
            if (apiResponse.success && Array.isArray(apiResponse.data)) {
                activeRovers.textContent = apiResponse.data.length
                let healthPercent = 100
                if (apiResponse.data.length > 0) {
                    const avgBattery = apiResponse.data.reduce((sum, rover) => sum + rover.BatteryPercentage, 0) / apiResponse.data.length
                    systemHealth.textContent = `${avgBattery.toFixed(0)}%`
                } else {
                    systemHealth.textContent = "0%"
                }
                apiResponse.data.forEach(rover => {
                    const card = document.createElement("div")
                    card.className = "rover-card"
                    card.innerHTML = `
                        <div class="rover-header">
                            <span class="rover-id">Rover #${rover.RoverID}</span>
                            <span class="rover-state ${String(rover.OperationalState).toLowerCase()}">${operationalStateToString(rover.OperationalState)}</span>
                        </div>
                        <div class="rover-stats">
                            <div class="stat"><span class="stat-label">Battery</span><span class="stat-value">${Number(rover.BatteryPercentage).toFixed(3)}%</span></div>
                            <div class="stat"><span class="stat-label">Temp</span><span class="stat-value">${Number(rover.Temperature).toFixed(3)}°C</span></div>
                        </div>
                        <div class="rover-position">
                            <span class="stat-label">Position</span>
                            <span class="stat-value">(${Number(rover.Position.X).toFixed(2)}, ${Number(rover.Position.Y).toFixed(2)})</span>
                        </div>
                    `
                    roversList.appendChild(card)

                    // --- Add marker to map if available and map is ready ---
                    if (mapReady && rover.Latitude !== undefined && rover.Longitude !== undefined) {
                        const marker = L.marker([rover.Latitude, rover.Longitude])
                            .addTo(map)
                            .bindPopup(`Rover #${rover.RoverID}<br>Status: ${operationalStateToString(rover.OperationalState)}`)
                        roverMarkers.push(marker)
                    }
                })
                // --- Fit map to markers if any and map is ready ---
                if (mapReady && roverMarkers.length > 0) {
                    const group = new L.featureGroup(roverMarkers)
                    map.fitBounds(group.getBounds().pad(0.2))
                }
                // Save rovers for grid
                window._latestRovers = apiResponse.data
                renderCanvasGrid(window._latestRovers, window._latestMissions || [])
            } else {
                activeRovers.textContent = "0"
                systemHealth.textContent = "0%"
                roversList.innerHTML = "<p>No rovers available.</p>"
                window._latestRovers = []
                renderCanvasGrid(window._latestRovers, window._latestMissions || [])
            }
        } catch (error) {
            console.error("Error fetching rovers:", error)
            activeRovers.textContent = "0"
            systemHealth.textContent = "0%"
            roversList.innerHTML = "<p>Error loading rovers.</p>"
            window._latestRovers = []
            renderCanvasGrid(window._latestRovers, window._latestMissions || [])
        }
    }

    function missionStatusToString(status) {
        switch (status) {
            case 1: return "Unassigned"
            case 2: return "Assigned"
            case 3: return "In Progress"
            case 4: return "Failed"
            case 5: return "Completed"
            default: return "Unknown"
        }
    }

    // Converts status string to a valid CSS class name
    function statusClassName(status) {
        return missionStatusToString(status).toLowerCase().replace(/\s+/g, '-')
    }


    function missionTaskToString(task) {
        switch (task) {
            case 1: return "Sample Analysis"
            case 2: return "Image Capture"
            case 3: return "Environmental Monitoring"
            case 4: return "Terrain Mapping"
            default: return "Unknown"
        }
    }

    async function fetchMissions() {
        const apiUrl = apiUrlInput.value
        try {
            const response = await fetch(`${apiUrl}/api/missions`)
            const apiResponse = await response.json()
            missionsList.innerHTML = ""
            if (apiResponse.success && Array.isArray(apiResponse.data)) {
                // Update Active Missions count
                activeMissions.textContent = apiResponse.data.length

                apiResponse.data.forEach(mission => {
                    const card = document.createElement("div")
                    card.className = "mission-card"
                    card.innerHTML = `
                        <div class="mission-id">Mission #${mission.MissionID}</div>
                        <div class="mission-task">${missionTaskToString(mission.Task)}</div>
                        <span class="mission-status ${statusClassName(mission.Status)}">${missionStatusToString(mission.Status)}</span>
                        <div class="progress-bar">
                            <div class="progress-fill" style="width: ${mission.Progress}%"></div>
                        </div>
                    `
                    missionsList.appendChild(card)
                })
                if (lastUpdate) {
                    lastUpdate.textContent = "Last Update: " + (new Date()).toLocaleTimeString()
                }
                // Save missions for grid
                window._latestMissions = apiResponse.data
                renderCanvasGrid(window._latestRovers || [], window._latestMissions)
            } else {
                activeMissions.textContent = "0"
                missionsList.innerHTML = "<p>No missions available.</p>"
                window._latestMissions = []
                renderCanvasGrid(window._latestRovers || [], window._latestMissions)
            }
        } catch (error) {
            activeMissions.textContent = "0"
            missionsList.innerHTML = "<p>Error loading missions.</p>"
            window._latestMissions = []
            renderCanvasGrid(window._latestRovers || [], window._latestMissions)
        }
    }

    // Initial fetch
    fetchRovers()
    fetchMissions()

    // Optionally, refresh every N seconds
    setInterval(() => {
        fetchRovers()
        fetchMissions()
        renderCanvasGrid(window._latestRovers || [], window._latestMissions || [])
    }, 250)
})