import { useState, useEffect } from "react"; // Removed useMemo
import GridMap from "./components/GridMap";
import { RoverCard, MissionCard } from "./components/Cards";
import {
  Telemetry,
  MissionAssignment,
  ApiResponse,
  OperationalState,
  MissionStatus,
} from "./types";
import { Wifi, Filter, Satellite } from "lucide-react";

const API_URL = "http://localhost:8080";

function App() {
  const [rovers, setRovers] = useState<Telemetry[]>([]);
  const [missions, setMissions] = useState<MissionAssignment[]>([]);
  const [lastUpdate, setLastUpdate] = useState(new Date());

  // Filters
  const [roverFilter, setRoverFilter] = useState<string>("ALL");
  const [missionFilter, setMissionFilter] = useState<string>("ALL");

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [roverRes, missionRes] = await Promise.all([
          fetch(`${API_URL}/api/rovers`),
          fetch(`${API_URL}/api/missions`),
        ]);

        const roverJson: ApiResponse<Telemetry[]> = await roverRes.json();
        const missionJson: ApiResponse<MissionAssignment[]> =
          await missionRes.json();

        if (roverJson.success) setRovers(roverJson.data || []);
        if (missionJson.success) setMissions(missionJson.data || []);

        setLastUpdate(new Date());
      } catch (err) {
        console.error("Connection lost", err);
      }
    };

    fetchData();
    const interval = setInterval(fetchData, 500);
    return () => clearInterval(interval);
  }, []);

  // Note: roverAssignments map is no longer needed since Telemetry has MissionID directly

  // --- Filter & Sort Logic ---
  const filteredRovers = rovers
    .filter((r) => {
      if (roverFilter === "ALL") return true;
      return r.OperationalState.toString() === roverFilter;
    })
    .sort((a, b) => a.RoverID - b.RoverID);

  const filteredMissions = missions
    .filter((m) => {
      if (missionFilter === "ALL") return true;
      return m.Status.toString() === missionFilter;
    })
    .sort((a, b) => a.MissionID - b.MissionID);

  return (
    <div className="h-screen w-screen bg-slate-950 text-slate-100 font-sans overflow-hidden flex flex-col">
      {/* HEADER */}
      <header className="h-[60px] shrink-0 bg-slate-900 border-b border-slate-800 px-6 flex justify-between items-center shadow-lg z-20 relative">
        <div>
          <h1 className="text-xl font-bold tracking-wider text-blue-400 flex items-center gap-3">
            <Satellite className="text-white" /> GROUND CONTROL{" "}
            <span className="text-slate-600">|</span>{" "}
            <span className="text-slate-400 text-sm">MISSION COMMANDER</span>
          </h1>
        </div>
        <div className="flex items-center gap-4 text-xs font-mono">
          <div className="flex items-center gap-2 text-green-400">
            <Wifi size={14} className="animate-pulse" /> LINK ESTABLISHED
          </div>
          <div className="text-slate-500">
            LAST SYNC: {lastUpdate.toLocaleTimeString()}
          </div>
        </div>
      </header>

      {/* MAIN CONTENT */}
      <main className="flex-1 grid grid-cols-12 gap-0 overflow-hidden">
        {/* LEFT PANEL: ROVERS */}
        <aside className="col-span-3 border-r border-slate-800 flex flex-col bg-slate-900/50 h-full overflow-hidden">
          <div className="shrink-0 p-4 border-b border-slate-800 bg-slate-900">
            <h2 className="font-bold text-slate-300 flex items-center gap-2 mb-3">
              ROVER FLEET{" "}
              <span className="text-xs bg-slate-700 px-2 py-0.5 rounded-full text-white">
                {filteredRovers.length}
              </span>
            </h2>
            <div className="flex items-center gap-2 bg-slate-800 p-2 rounded border border-slate-700">
              <Filter size={14} className="text-slate-400" />
              <select
                className="bg-transparent text-xs w-full outline-none text-slate-200 cursor-pointer"
                value={roverFilter}
                onChange={(e) => setRoverFilter(e.target.value)}
              >
                <option value="ALL">All States</option>
                <option value={OperationalState.Idle}>Idle</option>
                <option value={OperationalState.On_Mission}>On Mission</option>
                <option value={OperationalState.Error}>Error</option>
                <option value={OperationalState.Unknown}>Unknown</option>
              </select>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-4">
            {filteredRovers.length === 0 && (
              <div className="text-center text-slate-500 text-sm mt-10">
                No signals detected.
              </div>
            )}
            {filteredRovers.map((r) => (
              <RoverCard
                key={r.RoverID}
                rover={r}
                // UPDATED: No prop needed here anymore
              />
            ))}
          </div>
        </aside>

        {/* CENTER PANEL: MAP */}
        <section className="col-span-6 bg-slate-950 relative border-r border-slate-800 flex flex-col h-full overflow-hidden">
          <div className="absolute top-4 left-4 z-10 bg-slate-900/90 backdrop-blur px-3 py-2 rounded border border-slate-700 text-xs font-mono text-blue-300 pointer-events-none shadow-xl">
            <div>GRID SYSTEM: 1x1 METER</div>
            <div>PROJECTION: CARTESIAN 2D</div>
          </div>
          <div className="flex-1 p-0 overflow-hidden relative">
            <GridMap rovers={rovers} missions={missions} />
          </div>
        </section>

        {/* RIGHT PANEL: MISSIONS */}
        <aside className="col-span-3 flex flex-col bg-slate-900/50 h-full overflow-hidden">
          <div className="shrink-0 p-4 border-b border-slate-800 bg-slate-900">
            <h2 className="font-bold text-slate-300 flex items-center gap-2 mb-3">
              MISSION LOG{" "}
              <span className="text-xs bg-slate-700 px-2 py-0.5 rounded-full text-white">
                {filteredMissions.length}
              </span>
            </h2>
            <div className="flex items-center gap-2 bg-slate-800 p-2 rounded border border-slate-700">
              <Filter size={14} className="text-slate-400" />
              <select
                className="bg-transparent text-xs w-full outline-none text-slate-200 cursor-pointer"
                value={missionFilter}
                onChange={(e) => setMissionFilter(e.target.value)}
              >
                <option value="ALL">All Statuses</option>
                <option value={MissionStatus.Unassigned}>Unassigned</option>
                <option value={MissionStatus.Assigned}>Assigned</option>
                <option value={MissionStatus.In_Progress}>In Progress</option>
                <option value={MissionStatus.Completed}>Completed</option>
                <option value={MissionStatus.Failed}>Failed</option>
              </select>
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-4">
            {filteredMissions.length === 0 && (
              <div className="text-center text-slate-500 text-sm mt-10">
                No missions logged.
              </div>
            )}
            {filteredMissions.map((m) => (
              <MissionCard key={m.MissionID} mission={m} />
            ))}
          </div>
        </aside>
      </main>
    </div>
  );
}

export default App;
