import {
  Telemetry,
  MissionAssignment,
  OperationalState,
  MissionStatus,
  Task,
  HealthStatus,
  Shape,
  CoordsCircle,
  CoordsRectangle,
} from "../types";
import {
  Battery,
  Thermometer,
  AlertTriangle,
  CheckCircle,
  HelpCircle,
  XCircle,
  MapPin,
  Circle as CircleIcon,
  Square,
  Clock,
  Target,
} from "lucide-react";

// --- HELPER: Duration Formatter ---
const formatDuration = (totalSeconds: number) => {
  const seconds = Math.floor(totalSeconds) / 1_000_000_000;

  if (seconds < 60) {
    return `${seconds}s`;
  }

  const days = Math.floor(seconds / (3600 * 24));
  const hours = Math.floor((seconds % (3600 * 24)) / 3600);
  const minutes = Math.floor((seconds % 3600) / 60);
  const secs = Math.floor(seconds % 60);

  if (days > 0) {
    return `${days}d ${hours}h ${minutes}m ${secs}s`;
  }
  if (hours > 0) {
    return `${hours}h ${minutes}m ${secs}s`;
  }
  return `${minutes}m ${secs}s`;
};

// --- ROVER CARD ---
// Updated Interface: Now accepts assignedMissionId as an optional prop
export function RoverCard({
  rover,
  assignedMissionId,
}: {
  rover: Telemetry;
  assignedMissionId?: number;
}) {
  const getStatusColor = (s: OperationalState) => {
    switch (s) {
      case OperationalState.On_Mission:
        return "text-blue-400 border-blue-500";
      case OperationalState.Error:
        return "text-red-400 border-red-500";
      case OperationalState.Unknown:
        return "text-slate-400 border-slate-500";
      default:
        return "text-green-400 border-green-500";
    }
  };

  return (
    <div
      className={`bg-slate-800 p-4 rounded-lg border-l-4 mb-3 hover:bg-slate-750 transition ${getStatusColor(rover.OperationalState)}`}
    >
      <div className="flex justify-between items-start mb-2">
        <h3 className="font-bold text-slate-100">ROVER #{rover.RoverID}</h3>
        <span className="text-xs font-mono uppercase bg-slate-900 px-2 py-1 rounded text-slate-200">
          {OperationalState[rover.OperationalState]}
        </span>
      </div>

      {/* NEW SECTION: Mission Assignment Display */}
      <div className="mb-2 pb-2 border-b border-slate-700/50 flex items-center justify-between text-xs font-mono">
        <div className="flex items-center gap-1.5 text-slate-400">
          <Target size={12} />
          <span>MISSION:</span>
        </div>
        {assignedMissionId ? (
          <span className="text-blue-300 bg-blue-900/20 px-1.5 py-0.5 rounded border border-blue-900/50">
            #{assignedMissionId}
          </span>
        ) : (
          <span className="text-slate-600">NONE</span>
        )}
      </div>

      <div className="grid grid-cols-2 gap-2 text-sm text-slate-300">
        <div className="flex items-center gap-2">
          <Battery
            size={14}
            className={
              rover.BatteryPercentage < 20 ? "text-red-500" : "text-green-500"
            }
          />
          <span>{rover.BatteryPercentage.toFixed(1)}%</span>
        </div>
        <div className="flex items-center gap-2">
          <Thermometer size={14} />
          <span>{rover.Temperature.toFixed(1)}°C</span>
        </div>
        <div className="col-span-2 flex flex-col gap-1 mt-1 font-mono text-[11px] text-slate-400 bg-slate-900/50 p-2 rounded">
          <div className="flex justify-between">
            <span>POS</span>
            <span className="text-slate-200">
              X: {rover.Position.X.toFixed(2)} | Y:{" "}
              {rover.Position.Y.toFixed(2)}
            </span>
          </div>
          <div className="flex justify-between">
            <span>VEL</span>
            <span className="text-slate-200">
              X: {rover.Velocity.X.toFixed(2)} | Y:{" "}
              {rover.Velocity.Y.toFixed(2)}
            </span>
          </div>
        </div>
      </div>

      <div className="mt-3 pt-2 border-t border-slate-700 flex gap-2 justify-between">
        <HealthIcon label="MOTORS" status={rover.SystemHealth.Motors} />
        <HealthIcon label="SENSORS" status={rover.SystemHealth.Sensors} />
        <HealthIcon label="POWER" status={rover.SystemHealth.PowerSystem} />
      </div>
    </div>
  );
}

function HealthIcon({
  label,
  status,
}: {
  label: string;
  status: HealthStatus;
}) {
  let color = "text-slate-500";
  let Icon = HelpCircle;

  switch (status) {
    case HealthStatus.OK:
      color = "text-green-500";
      Icon = CheckCircle;
      break;
    case HealthStatus.Warning:
      color = "text-yellow-500";
      Icon = AlertTriangle;
      break;
    case HealthStatus.Critical:
      color = "text-red-500";
      Icon = XCircle;
      break;
  }

  return (
    <div className={`flex flex-col items-center ${color}`}>
      <Icon size={14} />
      <span className="text-[9px] font-bold mt-1">{label}</span>
    </div>
  );
}

// --- MISSION CARD ---
export function MissionCard({ mission }: { mission: MissionAssignment }) {
  const getProgressColor = (status: MissionStatus) => {
    if (status === MissionStatus.Failed) return "bg-red-500";
    if (status === MissionStatus.Completed) return "bg-green-500";
    if (status === MissionStatus.Unassigned) return "bg-slate-600";
    return "bg-blue-500";
  };

  const renderAreaInfo = () => {
    if (mission.Area.Shape === Shape.Circle) {
      const c = mission.Area.Coords as CoordsCircle;
      if (!c || !c.Center)
        return <span className="text-red-400">Invalid Coords</span>;

      return (
        <div className="grid grid-cols-2 gap-x-2 gap-y-1">
          <span className="text-slate-500">TYPE:</span>
          <span className="text-slate-300 flex items-center gap-1">
            <CircleIcon size={10} /> CIRCLE
          </span>

          <span className="text-slate-500">CENTER:</span>
          <span className="text-slate-300">
            [{c.Center.X.toFixed(0)}, {c.Center.Y.toFixed(0)}]
          </span>

          <span className="text-slate-500">RADIUS:</span>
          <span className="text-slate-300">{c.Radius.toFixed(0)}m</span>
        </div>
      );
    } else if (mission.Area.Shape === Shape.Rectangle) {
      const r = mission.Area.Coords as CoordsRectangle;
      if (!r || !r.TopLeft || !r.BottomRight)
        return <span className="text-red-400">Invalid Coords</span>;

      return (
        <div className="grid grid-cols-2 gap-x-2 gap-y-1">
          <span className="text-slate-500">TYPE:</span>
          <span className="text-slate-300 flex items-center gap-1">
            <Square size={10} /> RECT
          </span>

          <span className="text-slate-500">TOP-L:</span>
          <span className="text-slate-300">
            [{r.TopLeft.X.toFixed(0)}, {r.TopLeft.Y.toFixed(0)}]
          </span>

          <span className="text-slate-500">BOT-R:</span>
          <span className="text-slate-300">
            [{r.BottomRight.X.toFixed(0)}, {r.BottomRight.Y.toFixed(0)}]
          </span>
        </div>
      );
    }
    return <span>Unknown Shape</span>;
  };

  return (
    <div className="bg-slate-800 p-4 rounded-lg border border-slate-700 mb-3 hover:border-slate-500 transition group">
      {/* Header */}
      <div className="flex justify-between items-center mb-2">
        <h3 className="font-bold text-slate-200">
          MISSION #{mission.MissionID}
        </h3>
        <span className="text-[10px] uppercase text-blue-300 border border-blue-900 bg-blue-900/30 px-2 py-0.5 rounded">
          {Task[mission.Task]}
        </span>
      </div>

      {/* Status & Duration */}
      <div className="flex justify-between items-center text-xs text-slate-500 mb-3 font-mono bg-slate-900/30 p-1.5 rounded">
        <span className="font-bold text-slate-400">
          {MissionStatus[mission.Status]}
        </span>
        <span className="flex items-center gap-1" title="Max Duration">
          <Clock size={10} />
          {formatDuration(mission.MaxDuration)} Limit
        </span>
      </div>

      {/* Geographic Data Block */}
      <div className="mb-3 p-2 bg-slate-900 rounded border border-slate-700/50 text-[10px] font-mono">
        <div className="flex items-center gap-1 text-slate-400 mb-2 border-b border-slate-700 pb-1">
          <MapPin size={10} /> GEOGRAPHIC TARGET
        </div>
        {renderAreaInfo()}
      </div>

      {/* Progress Bar */}
      <div className="flex items-center gap-2">
        <div className="flex-1 bg-slate-900 rounded-full h-1.5">
          <div
            className={`h-1.5 rounded-full transition-all duration-500 ${getProgressColor(mission.Status)}`}
            style={{ width: `${mission.Progress}%` }}
          ></div>
        </div>
        <div className="text-[10px] text-slate-400 font-mono w-8 text-right">
          {mission.Progress.toFixed(0)}%
        </div>
      </div>
    </div>
  );
}
