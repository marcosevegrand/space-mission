import { useRef, useEffect, useState, MouseEvent } from "react";
import {
  Telemetry,
  MissionAssignment,
  Shape,
  MissionStatus,
  CoordsCircle,
  CoordsRectangle,
  OperationalState,
} from "../types";
import { Plus, Minus, Move } from "lucide-react";

interface GridMapProps {
  rovers: Telemetry[];
  missions: MissionAssignment[];
}

export default function GridMap({ rovers, missions }: GridMapProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const canvasRef = useRef<HTMLCanvasElement>(null);

  const [dimensions, setDimensions] = useState({ width: 0, height: 0 });
  const [zoom, setZoom] = useState<number>(1.0);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [isDragging, setIsDragging] = useState(false);
  const [dragStart, setDragStart] = useState({ x: 0, y: 0 });

  const BASE_PIXELS_PER_UNIT = 20;

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        setDimensions({ width, height });
      }
    });
    resizeObserver.observe(container);
    return () => resizeObserver.disconnect();
  }, []);

  const handleMouseDown = (e: MouseEvent) => {
    setIsDragging(true);
    setDragStart({ x: e.clientX - pan.x, y: e.clientY - pan.y });
  };

  const handleMouseMove = (e: MouseEvent) => {
    if (!isDragging) return;
    setPan({ x: e.clientX - dragStart.x, y: e.clientY - dragStart.y });
  };

  const handleMouseUp = () => setIsDragging(false);

  const handleZoom = (direction: "in" | "out") => {
    setZoom((prev) => {
      const newZoom = direction === "in" ? prev * 1.2 : prev / 1.2;
      return Math.max(0.1, Math.min(newZoom, 10));
    });
  };

  // --- DRAWING LOGIC ---
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas || dimensions.width === 0 || dimensions.height === 0) return;

    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    const width = dimensions.width;
    const height = dimensions.height;
    const currentScale = BASE_PIXELS_PER_UNIT * zoom;

    const toScreen = (x: number, y: number) => ({
      x: width / 2 + pan.x + x * currentScale,
      y: height / 2 + pan.y - y * currentScale,
    });

    const toWorld = (sx: number, sy: number) => ({
      x: (sx - width / 2 - pan.x) / currentScale,
      y: (height / 2 + pan.y - sy) / currentScale,
    });

    // --- FILTER LOGIC (Updated) ---
    const now = new Date().getTime();

    const visibleMissions = missions.filter((m) => {
      // 1. Always show active or pending missions
      if (
        m.Status === MissionStatus.Assigned ||
        m.Status === MissionStatus.In_Progress ||
        m.Status === MissionStatus.Unassigned
      ) {
        return true;
      }

      // 2. For Completed, Failed, or Unknown: Hide if older than 15s
      const lastUpdate = new Date(m.last_updated).getTime();
      const diffSeconds = (now - lastUpdate) / 1000;

      // Keep if updated within the last 15 seconds
      return diffSeconds < 15;
    });

    // Frame & Grid
    ctx.fillStyle = "#0f172a"; // Slate 900
    ctx.fillRect(0, 0, width, height);

    const topLeft = toWorld(0, 0);
    const bottomRight = toWorld(width, height);
    let step = 1;
    if (currentScale < 10) step = 5;
    if (currentScale < 4) step = 10;
    if (currentScale < 1) step = 50;

    ctx.lineWidth = 1;
    ctx.strokeStyle = "#1e293b"; // Slate 800

    const startX = Math.floor(topLeft.x / step) * step;
    const endX = Math.ceil(bottomRight.x / step) * step;
    const startY = Math.floor(bottomRight.y / step) * step;
    const endY = Math.ceil(topLeft.y / step) * step;

    ctx.beginPath();
    for (let x = startX; x <= endX; x += step) {
      const p1 = toScreen(x, 0);
      const px = Math.round(p1.x) + 0.5;
      ctx.moveTo(px, 0);
      ctx.lineTo(px, height);
    }
    for (let y = startY; y <= endY; y += step) {
      const p1 = toScreen(0, y);
      const py = Math.round(p1.y) + 0.5;
      ctx.moveTo(0, py);
      ctx.lineTo(width, py);
    }
    ctx.stroke();

    // Axis
    ctx.lineWidth = 2;
    ctx.strokeStyle = "#475569";
    ctx.beginPath();
    const origin = toScreen(0, 0);
    ctx.moveTo(origin.x, 0);
    ctx.lineTo(origin.x, height);
    ctx.moveTo(0, origin.y);
    ctx.lineTo(width, origin.y);
    ctx.stroke();
    ctx.fillStyle = "#64748b";
    ctx.font = "10px monospace";
    ctx.textAlign = "left";
    ctx.textBaseline = "alphabetic";
    ctx.fillText("(0,0)", origin.x + 4, origin.y - 4);

    // --- DRAW MISSIONS ---
    visibleMissions.forEach((m) => {
      // Mission Colors
      switch (m.Status) {
        case MissionStatus.Unassigned:
          ctx.fillStyle = "rgba(255, 255, 255, 0.05)"; // White (Transparent)
          ctx.strokeStyle = "rgba(255, 255, 255, 0.2)";
          break;
        case MissionStatus.Assigned:
          ctx.fillStyle = "rgba(249, 115, 22, 0.1)"; // Orange
          ctx.strokeStyle = "rgba(249, 115, 22, 0.5)";
          break;
        case MissionStatus.In_Progress:
          ctx.fillStyle = "rgba(59, 130, 246, 0.2)"; // Blue
          ctx.strokeStyle = "rgba(59, 130, 246, 0.8)";
          break;
        case MissionStatus.Failed:
          ctx.fillStyle = "rgba(239, 68, 68, 0.2)"; // Red
          ctx.strokeStyle = "rgba(239, 68, 68, 0.8)";
          break;
        case MissionStatus.Completed:
          ctx.fillStyle = "rgba(34, 197, 94, 0.2)"; // Green
          ctx.strokeStyle = "rgba(34, 197, 94, 0.8)";
          break;
        case MissionStatus.Unknown:
          ctx.fillStyle = "rgba(156, 163, 175, 0.2)"; // Light Gray
          ctx.strokeStyle = "rgba(156, 163, 175, 0.6)";
          break;
        default:
          ctx.fillStyle = "rgba(100, 116, 139, 0.1)"; // Fallback Gray
          ctx.strokeStyle = "rgba(100, 116, 139, 0.5)";
          break;
      }

      ctx.lineWidth = 2;
      ctx.beginPath();

      let labelCenter = { x: 0, y: 0 };
      let shouldDrawLabel = false;

      if (m.Area.Shape === Shape.Circle) {
        const c = m.Area.Coords as CoordsCircle;
        if (c.Center) {
          const center = toScreen(c.Center.X, c.Center.Y);
          const radiusPx = c.Radius * currentScale;
          ctx.arc(center.x, center.y, Math.max(radiusPx, 2), 0, 2 * Math.PI);
          labelCenter = center;
          shouldDrawLabel = true;
        }
      } else if (m.Area.Shape === Shape.Rectangle) {
        const r = m.Area.Coords as CoordsRectangle;
        if (r.TopLeft && r.BottomRight) {
          const p1 = toScreen(r.TopLeft.X, r.TopLeft.Y);
          const p2 = toScreen(r.BottomRight.X, r.BottomRight.Y);
          ctx.rect(p1.x, p1.y, p2.x - p1.x, p2.y - p1.y);
          labelCenter = {
            x: p1.x + (p2.x - p1.x) / 2,
            y: p1.y + (p2.y - p1.y) / 2,
          };
          shouldDrawLabel = true;
        }
      }

      ctx.fill();
      ctx.stroke();

      if (shouldDrawLabel) {
        ctx.fillStyle = "rgba(255, 255, 255, 0.6)";
        ctx.font = "10px monospace";
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillText(`M${m.MissionID}`, labelCenter.x, labelCenter.y);
      }
    });

    // --- DRAW ROVERS ---
    rovers.forEach((r) => {
      const pos = toScreen(r.Position.X, r.Position.Y);

      // Velocity Vector
      const vx = r.Velocity.X;
      const vy = r.Velocity.Y;
      if (Math.abs(vx) > 0.01 || Math.abs(vy) > 0.01) {
        ctx.beginPath();
        ctx.moveTo(pos.x, pos.y);
        const mag = Math.sqrt(vx * vx + vy * vy);
        const dirX = vx / mag;
        const dirY = vy / mag;
        const vecEnd = { x: pos.x + dirX * 20, y: pos.y - dirY * 20 };
        ctx.lineTo(vecEnd.x, vecEnd.y);
        ctx.strokeStyle = "#fff";
        ctx.lineWidth = 1;
        ctx.stroke();
      }

      ctx.beginPath();
      ctx.arc(pos.x, pos.y, 6, 0, 2 * Math.PI);

      // Rover Colors
      switch (r.OperationalState) {
        case OperationalState.Idle:
          ctx.fillStyle = "#f97316"; // Orange
          break;
        case OperationalState.On_Mission:
          ctx.fillStyle = "#3b82f6"; // Blue
          break;
        case OperationalState.Error:
          ctx.fillStyle = "#ef4444"; // Red
          break;
        default:
          ctx.fillStyle = "#64748b"; // Gray (Unknown)
          break;
      }

      ctx.fill();
      ctx.strokeStyle = "#fff";
      ctx.lineWidth = 1;
      ctx.stroke();

      ctx.fillStyle = "#fff";
      ctx.font = "bold 12px monospace";
      ctx.textAlign = "left";
      ctx.textBaseline = "alphabetic";
      ctx.fillText(`R${r.RoverID}`, pos.x + 8, pos.y - 8);
    });
  }, [rovers, missions, zoom, pan, dimensions]);

  return (
    <div
      ref={containerRef}
      className="relative w-full h-full overflow-hidden rounded-lg bg-slate-900 group"
    >
      <canvas
        ref={canvasRef}
        width={dimensions.width}
        height={dimensions.height}
        className={`block ${isDragging ? "cursor-grabbing" : "cursor-grab"}`}
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseUp={handleMouseUp}
        onMouseLeave={handleMouseUp}
      />

      {/* CONTROLS */}
      <div className="absolute bottom-4 right-4 flex flex-col gap-2">
        <button
          onClick={() => setPan({ x: 0, y: 0 })}
          className="bg-slate-800 text-slate-200 p-2 rounded shadow-lg border border-slate-700 hover:bg-slate-700 hover:text-blue-400 transition"
          title="Reset View"
        >
          <Move size={20} />
        </button>
        <div className="flex flex-col bg-slate-800 rounded shadow-lg border border-slate-700 overflow-hidden">
          <button
            onClick={() => handleZoom("in")}
            className="p-2 hover:bg-slate-700 hover:text-green-400 transition border-b border-slate-700"
          >
            <Plus size={20} />
          </button>
          <button
            onClick={() => handleZoom("out")}
            className="p-2 hover:bg-slate-700 hover:text-red-400 transition"
          >
            <Minus size={20} />
          </button>
        </div>
        <div className="bg-slate-900/80 text-xs text-slate-400 px-2 py-1 rounded text-center font-mono">
          {zoom.toFixed(1)}x
        </div>
      </div>
    </div>
  );
}
