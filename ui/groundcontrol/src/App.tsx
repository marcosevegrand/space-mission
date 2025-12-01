import { useState, useEffect, useCallback, useRef } from "react";
import GridMap from "./components/GridMap";
import { RoverCard, MissionCard } from "./components/Cards";
import MissionControl from "./components/MissionControl";
import {
  Telemetry,
  MissionAssignment,
  ApiResponse,
  OperationalState,
  MissionStatus,
} from "./types";
import {
  Wifi,
  WifiOff,
  Filter,
  Satellite,
  Hammer,
  Moon,
  HelpCircle,
  X,
  Info,
  AlertTriangle,
} from "lucide-react";

const API_URL = window.ENV?.API_URL || "http://localhost:8003";

function App() {
  const [rovers, setRovers] = useState<Telemetry[]>([]);
  const [missions, setMissions] = useState<MissionAssignment[]>([]);
  const [lastUpdate, setLastUpdate] = useState(new Date());

  // Connection State
  const [isConnected, setIsConnected] = useState(false);

  const [roverFilter, setRoverFilter] = useState<string>("ALL");
  const [missionFilter, setMissionFilter] = useState<string>("ALL");

  // Modal State
  const [showHelp, setShowHelp] = useState(false);

  // Ref to prevent stacking requests
  const isFetching = useRef(false);

  const fetchData = useCallback(async () => {
    if (isFetching.current) return;
    isFetching.current = true;

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
      setIsConnected(true); // Connection successful
    } catch (err) {
      console.error("Connection lost", err);
      setIsConnected(false); // Connection failed
    } finally {
      isFetching.current = false;
    }
  }, []);

  useEffect(() => {
    const initial = setTimeout(() => fetchData(), 0);
    const interval = setInterval(fetchData, 100);

    return () => {
      clearTimeout(initial);
      clearInterval(interval);
    };
  }, [fetchData]);

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

  // Availability: Strictly "Idle" rovers (No Mission, No Error, Not Unknown)
  const availableRoversCount = rovers.filter(
    (r) =>
      !r.has_mission &&
      r.OperationalState !== OperationalState.Error &&
      r.OperationalState !== OperationalState.Unknown,
  ).length;

  const maxMissionId = missions.reduce(
    (max, m) => Math.max(max, m.MissionID),
    0,
  );

  return (
    <div className="h-screen w-screen bg-slate-950 text-slate-100 font-sans overflow-hidden flex flex-col">
      {/* HEADER */}
      <header className="h-[60px] shrink-0 bg-slate-900 border-b border-slate-800 px-6 flex justify-between items-center shadow-lg z-20 relative">
        <div>
          <h1 className="text-xl font-bold tracking-wider text-blue-400 flex items-center gap-3">
            <Satellite className="text-white" /> GROUND CONTROL{" "}
            <span className="text-slate-600">|</span>{" "}
            <span className="text-slate-400 text-sm">MISSION COMMANDER</span>
            <button
              onClick={() => setShowHelp(true)}
              className="ml-2 p-1 text-slate-500 hover:text-blue-400 hover:bg-slate-800 rounded-full transition"
              title="System Legend & Help"
            >
              <HelpCircle size={18} />
            </button>
          </h1>
        </div>
        <div className="flex items-center gap-4 text-xs font-mono">
          {isConnected ? (
            <div className="flex items-center gap-2 text-green-400 transition-colors duration-300">
              <Wifi size={14} className="animate-pulse" /> LINK ESTABLISHED
            </div>
          ) : (
            <div className="flex items-center gap-2 text-red-500 font-bold transition-colors duration-300 animate-pulse">
              <WifiOff size={14} /> CONNECTION LOST
            </div>
          )}
          <div className="text-slate-500">
            LAST SYNC: {lastUpdate.toLocaleTimeString()}
          </div>
        </div>
      </header>

      {/* MAIN CONTENT */}
      <main className="flex-1 grid grid-cols-12 gap-0 overflow-hidden">
        {/* --- LEFT PANEL: ROVERS --- */}
        <aside className="col-span-3 border-r border-slate-800 flex flex-col bg-slate-900/50 h-full overflow-hidden">
          {/* SECTION 1: AVAILABLE FLEET */}
          <div className="shrink-0 p-4 border-b border-slate-800 bg-slate-900">
            <h2 className="font-bold text-slate-300 flex items-center justify-between gap-2 mb-3">
              <span>AVAILABLE FLEET</span>
              <span className="text-xs bg-slate-700 px-2 py-0.5 rounded-full text-white">
                {availableRoversCount} / {rovers.length}
              </span>
            </h2>
            <div className="grid grid-cols-4 gap-2">
              {rovers
                .sort((a, b) => a.RoverID - b.RoverID)
                .map((r) => {
                  const isError = r.OperationalState === OperationalState.Error;
                  const isUnknown =
                    r.OperationalState === OperationalState.Unknown;
                  const hasMission = r.has_mission;

                  // --- LOGIC DETERMINATION ---
                  // Priority: Error -> Unknown -> Busy -> Idle
                  let containerClass = "";
                  let iconColor = "";
                  let textColor = "";
                  let IconComponent = Moon;
                  let title = "";

                  if (isError) {
                    // 1. ERROR (Red)
                    containerClass = "bg-red-900/10 border-red-900/40";
                    iconColor = "text-red-500";
                    textColor = "text-red-400";
                    IconComponent = AlertTriangle;
                    title = "Error State";
                  } else if (isUnknown) {
                    // 2. OFFLINE / UNKNOWN (Gray)
                    containerClass =
                      "bg-slate-800 border-slate-700/50 opacity-60";
                    iconColor = "text-slate-500";
                    textColor = "text-slate-500";
                    IconComponent = HelpCircle;
                    title = "Offline / Unknown";
                  } else if (hasMission) {
                    // 3. BUSY (Orange)
                    containerClass = "bg-orange-900/10 border-orange-900/40";
                    iconColor = "text-orange-400";
                    textColor = "text-orange-200";
                    IconComponent = Hammer;
                    title = "Busy / On Mission";
                  } else {
                    // 4. IDLE (White)
                    containerClass = "bg-slate-700 border-slate-500";
                    iconColor = "text-white";
                    textColor = "text-white";
                    IconComponent = Moon;
                    title = "Idle / Available";
                  }

                  return (
                    <div
                      key={r.RoverID}
                      className={`flex flex-col items-center justify-center p-2 rounded border transition-colors ${containerClass}`}
                      title={title}
                    >
                      <div className="mb-1">
                        <IconComponent size={16} className={iconColor} />
                      </div>
                      <span
                        className={`text-xs font-bold font-mono ${textColor}`}
                      >
                        R{r.RoverID}
                      </span>
                    </div>
                  );
                })}
              {rovers.length === 0 && (
                <span className="text-xs text-slate-500 col-span-4 text-center">
                  No Fleet Data
                </span>
              )}
            </div>
          </div>

          {/* SECTION 2: ROVER FLEET LIST */}
          <div className="flex flex-col flex-1 overflow-hidden">
            <div className="shrink-0 p-4 border-b border-slate-800 bg-slate-900">
              <h2 className="font-bold text-slate-300 flex items-center gap-2 mb-2">
                ROVER STATUS
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
                  <option value={OperationalState.On_Mission}>
                    On Mission
                  </option>
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
                <RoverCard key={r.RoverID} rover={r} />
              ))}
            </div>
          </div>
        </aside>

        {/* --- CENTER PANEL: MAP --- */}
        <section className="col-span-6 bg-slate-950 relative border-r border-slate-800 flex flex-col h-full overflow-hidden">
          <div className="absolute top-4 left-4 z-10 bg-slate-900/90 backdrop-blur px-3 py-2 rounded border border-slate-700 text-xs font-mono text-blue-300 pointer-events-none shadow-xl">
            <div>GRID SYSTEM: 1x1 METER</div>
            <div>PROJECTION: CARTESIAN 2D</div>
          </div>
          <div className="flex-1 p-0 overflow-hidden relative">
            <GridMap rovers={rovers} missions={missions} />
          </div>
        </section>

        {/* --- RIGHT PANEL: MISSIONS --- */}
        <aside className="col-span-3 flex flex-col bg-slate-900/50 h-full overflow-hidden">
          {/* SECTION 1: MISSION CONTROL FORM */}
          <MissionControl
            nextId={maxMissionId + 1}
            onMissionAdded={fetchData}
          />

          {/* SECTION 2: MISSION LOG */}
          <div className="flex flex-col flex-1 overflow-hidden">
            <div className="shrink-0 p-4 border-b border-slate-800 bg-slate-900">
              <h2 className="font-bold text-slate-300 flex items-center gap-2 mb-2">
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
                  <option value={MissionStatus.Unknown}>Unknown</option>
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
          </div>
        </aside>
      </main>

      {/* HELP MODAL */}
      {showHelp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm">
          <div className="bg-slate-900 border border-slate-700 rounded-lg shadow-2xl w-[600px] max-h-[90vh] overflow-y-auto flex flex-col animate-[fadeIn_0.2s_ease-out]">
            {/* Modal Header */}
            <div className="flex justify-between items-center p-4 border-b border-slate-800 bg-slate-800/50 sticky top-0 backdrop-blur">
              <h2 className="font-bold text-slate-100 flex items-center gap-2">
                <Info size={18} className="text-blue-400" /> SYSTEM LEGEND &
                GUIDE
              </h2>
              <button
                onClick={() => setShowHelp(false)}
                className="text-slate-400 hover:text-white transition bg-slate-800 hover:bg-slate-700 p-1 rounded"
              >
                <X size={20} />
              </button>
            </div>

            {/* Modal Content */}
            <div className="p-6 space-y-6 text-sm text-slate-300">
              {/* 1. Fleet Availability */}
              <div>
                <h3 className="text-xs font-bold text-slate-500 uppercase mb-3 border-b border-slate-800 pb-1">
                  1. Fleet Availability (Left Panel)
                </h3>
                <div className="grid grid-cols-2 gap-4">
                  {/* IDLE */}
                  <div className="flex items-center gap-3">
                    <div className="bg-slate-700 border border-slate-500 p-2 rounded">
                      <Moon size={16} className="text-white" />
                    </div>
                    <div>
                      <div className="font-bold text-white">Idle</div>
                      <div className="text-xs text-slate-500">
                        Rover is OK and has no mission.
                      </div>
                    </div>
                  </div>

                  {/* BUSY */}
                  <div className="flex items-center gap-3">
                    <div className="bg-orange-900/20 border border-orange-900/40 p-2 rounded">
                      <Hammer size={16} className="text-orange-400" />
                    </div>
                    <div>
                      <div className="font-bold text-orange-400">Busy</div>
                      <div className="text-xs text-slate-500">
                        Rover is OK and has a mission.
                      </div>
                    </div>
                  </div>

                  {/* OFFLINE */}
                  <div className="flex items-center gap-3">
                    <div className="bg-slate-800 border border-slate-700 p-2 rounded opacity-60">
                      <HelpCircle size={16} className="text-slate-500" />
                    </div>
                    <div>
                      <div className="font-bold text-slate-500">Offline</div>
                      <div className="text-xs text-slate-500">
                        Rover state is Unknown (signal lost).
                      </div>
                    </div>
                  </div>

                  {/* ERROR */}
                  <div className="flex items-center gap-3">
                    <div className="bg-red-900/20 border border-red-900/40 p-2 rounded">
                      <AlertTriangle size={16} className="text-red-500" />
                    </div>
                    <div>
                      <div className="font-bold text-red-400">Error</div>
                      <div className="text-xs text-slate-500">
                        Rover reports a critical error.
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              {/* 2. Rover Telemetry */}
              <div>
                <h3 className="text-xs font-bold text-slate-500 uppercase mb-3 border-b border-slate-800 pb-1">
                  2. Telemetry Status (Cards & Map)
                </h3>
                <ul className="space-y-2">
                  <li className="flex items-center gap-3">
                    <span className="w-3 h-3 rounded-full bg-orange-400 shadow-[0_0_8px_rgba(251,146,60,0.5)]"></span>
                    <span className="text-orange-300 font-mono">Idle</span>
                    <span className="text-xs text-slate-500 ml-auto">
                      Stationary, awaiting commands.
                    </span>
                  </li>
                  <li className="flex items-center gap-3">
                    <span className="w-3 h-3 rounded-full bg-blue-500 shadow-[0_0_8px_rgba(59,130,246,0.5)]"></span>
                    <span className="text-blue-300 font-mono">On Mission</span>
                    <span className="text-xs text-slate-500 ml-auto">
                      Actively executing a task.
                    </span>
                  </li>
                  <li className="flex items-center gap-3">
                    <span className="w-3 h-3 rounded-full bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)]"></span>
                    <span className="text-red-300 font-mono">Error</span>
                    <span className="text-xs text-slate-500 ml-auto">
                      Critical system failure detected.
                    </span>
                  </li>
                  <li className="flex items-center gap-3">
                    <span className="w-3 h-3 rounded-full bg-slate-500 shadow-[0_0_8px_rgba(100,116,139,0.5)]"></span>
                    <span className="text-slate-300 font-mono">Unknown</span>
                    <span className="text-xs text-slate-500 ml-auto">
                      No data received or invalid state.
                    </span>
                  </li>
                </ul>
              </div>

              {/* 3. Missions */}
              <div>
                <h3 className="text-xs font-bold text-slate-500 uppercase mb-3 border-b border-slate-800 pb-1">
                  3. Mission Status (Right Panel)
                </h3>
                <div className="grid grid-cols-2 gap-2 text-xs">
                  <div className="bg-slate-700/30 border border-slate-600/50 p-2 rounded text-slate-300">
                    <span className="font-bold">UNASSIGNED</span>
                    <br />
                    Created, waiting for available rover.
                  </div>
                  <div className="bg-orange-500/10 border border-orange-500/30 p-2 rounded text-orange-400">
                    <span className="font-bold">ASSIGNED</span>
                    <br />
                    Rover selected, mission not yet started.
                  </div>
                  <div className="bg-blue-500/10 border border-blue-500/30 p-2 rounded text-blue-400">
                    <span className="font-bold">IN PROGRESS</span>
                    <br />
                    Rover is actively working.
                  </div>
                  <div className="bg-green-500/10 border border-green-500/30 p-2 rounded text-green-400">
                    <span className="font-bold">COMPLETED</span>
                    <br />
                    Objective achieved successfully.
                  </div>
                  <div className="bg-red-500/10 border border-red-500/30 p-2 rounded text-red-400">
                    <span className="font-bold">FAILED</span>
                    <br />
                    Aborted or timed out.
                  </div>
                  <div className="bg-slate-800 border border-slate-700 p-2 rounded text-slate-500">
                    <span className="font-bold">UNKNOWN</span>
                    <br />
                    Status data lost or corrupted.
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
