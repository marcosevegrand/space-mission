import React, { useState, useEffect } from "react";
import {
  Upload,
  PlusCircle,
  AlertCircle,
  CheckCircle2,
  Circle as CircleIcon,
  Square,
} from "lucide-react";
import { Task, Shape } from "../types";

const API_URL = window.ENV?.API_URL || "http://localhost:8080";

// 1. Define the Payload Interface locally
interface MissionPayload {
  mission_id: number;
  task: number;
  duration_sec: number;
  frequency_sec: number;
  shape_type: number;
  // Optional fields depending on shape
  center_x?: number;
  center_y?: number;
  radius?: number;
  top_left_x?: number;
  top_left_y?: number;
  bottom_right_x?: number;
  bottom_right_y?: number;
}

export default function MissionControl({
  nextId,
  onMissionAdded,
}: {
  nextId: number;
  onMissionAdded: () => void;
}) {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  const [shape, setShape] = useState<Shape>(Shape.Circle);
  const [commonData, setCommonData] = useState({
    id: nextId,
    task: "1",
    duration: 300,
    frequency: 1,
  });

  const [circleData, setCircleData] = useState({ x: 0, y: 0, radius: 50 });
  const [rectData, setRectData] = useState({
    x1: -20,
    y1: 20,
    x2: 20,
    y2: -20,
  });

  // Sync ID when the parent suggests a new one
  useEffect(() => {
    setCommonData((prev) => ({ ...prev, id: nextId }));
  }, [nextId]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError(null);
    setSuccess(false);

    try {
      // 2. Construct the specific shape object
      let shapePayload = {};
      if (shape === Shape.Circle) {
        shapePayload = {
          center_x: Number(circleData.x),
          center_y: Number(circleData.y),
          radius: Number(circleData.radius),
        };
      } else {
        shapePayload = {
          top_left_x: Number(rectData.x1),
          top_left_y: Number(rectData.y1),
          bottom_right_x: Number(rectData.x2),
          bottom_right_y: Number(rectData.y2),
        };
      }

      // 3. Create the strictly typed payload
      const payload: MissionPayload = {
        mission_id: Number(commonData.id),
        task: Number(commonData.task),
        duration_sec: Number(commonData.duration),
        frequency_sec: Number(commonData.frequency),
        shape_type: shape,
        ...shapePayload,
      };

      const res = await fetch(`${API_URL}/api/missions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });

      const data = await res.json();

      if (!res.ok || !data.success) {
        throw new Error(data.error || "Failed to create mission");
      }

      setSuccess(true);
      onMissionAdded();

      setTimeout(() => setSuccess(false), 2000);
    } catch (err: unknown) {
      let message = "Failed to create mission";
      if (err instanceof Error) {
        message = err.message;
      }
      setError(message);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="bg-slate-900 border-b border-slate-800 p-4 shrink-0">
      <h2 className="font-bold text-slate-300 flex items-center gap-2 mb-3">
        <Upload size={16} className="text-blue-400" /> MISSION CONTROL
      </h2>

      <form onSubmit={handleSubmit} className="flex flex-col gap-3">
        {/* Row 1: ID & Task */}
        <div className="grid grid-cols-3 gap-2">
          <div className="col-span-1">
            <label className="text-[10px] text-slate-500 font-bold block mb-1">
              ID
            </label>
            <input
              type="number"
              className="w-full bg-slate-800 border border-slate-700 rounded p-1.5 text-xs text-white focus:border-blue-500 outline-none font-mono"
              value={commonData.id}
              onChange={(e) =>
                setCommonData({ ...commonData, id: Number(e.target.value) })
              }
              required
            />
          </div>
          <div className="col-span-2">
            <label className="text-[10px] text-slate-500 font-bold block mb-1">
              TASK TYPE
            </label>
            <select
              className="w-full bg-slate-800 border border-slate-700 rounded p-1.5 text-xs text-white focus:border-blue-500 outline-none"
              value={commonData.task}
              onChange={(e) =>
                setCommonData({ ...commonData, task: e.target.value })
              }
            >
              <option value={Task.Sample_Analysis}>Sample Analysis</option>
              <option value={Task.Image_Capture}>Image Capture</option>
              <option value={Task.Environmental_Monitoring}>
                Env. Monitoring
              </option>
              <option value={Task.Terrain_Mapping}>Terrain Mapping</option>
            </select>
          </div>
        </div>

        {/* Shape Selector */}
        <div>
          <label className="text-[10px] text-slate-500 font-bold block mb-1">
            AREA SHAPE
          </label>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setShape(Shape.Circle)}
              className={`flex-1 flex items-center justify-center gap-2 py-1.5 rounded text-xs border ${shape === Shape.Circle ? "bg-blue-600 border-blue-500 text-white" : "bg-slate-800 border-slate-700 text-slate-400 hover:bg-slate-700"}`}
            >
              <CircleIcon size={12} /> Circle
            </button>
            <button
              type="button"
              onClick={() => setShape(Shape.Rectangle)}
              className={`flex-1 flex items-center justify-center gap-2 py-1.5 rounded text-xs border ${shape === Shape.Rectangle ? "bg-blue-600 border-blue-500 text-white" : "bg-slate-800 border-slate-700 text-slate-400 hover:bg-slate-700"}`}
            >
              <Square size={12} /> Rectangle
            </button>
          </div>
        </div>

        {/* Dynamic Inputs based on Shape */}
        {shape === Shape.Circle ? (
          <div className="grid grid-cols-3 gap-2">
            <div>
              <label className="text-[10px] text-slate-500 font-bold block mb-1">
                CENTER X
              </label>
              <input
                type="number"
                className="w-full bg-slate-800 border border-slate-700 rounded p-1.5 text-xs text-white focus:border-blue-500 outline-none font-mono"
                value={circleData.x}
                onChange={(e) =>
                  setCircleData({ ...circleData, x: Number(e.target.value) })
                }
                required
              />
            </div>
            <div>
              <label className="text-[10px] text-slate-500 font-bold block mb-1">
                CENTER Y
              </label>
              <input
                type="number"
                className="w-full bg-slate-800 border border-slate-700 rounded p-1.5 text-xs text-white focus:border-blue-500 outline-none font-mono"
                value={circleData.y}
                onChange={(e) =>
                  setCircleData({ ...circleData, y: Number(e.target.value) })
                }
                required
              />
            </div>
            <div>
              <label className="text-[10px] text-slate-500 font-bold block mb-1">
                RADIUS
              </label>
              <input
                type="number"
                className="w-full bg-slate-800 border border-slate-700 rounded p-1.5 text-xs text-white focus:border-blue-500 outline-none font-mono"
                value={circleData.radius}
                onChange={(e) =>
                  setCircleData({
                    ...circleData,
                    radius: Number(e.target.value),
                  })
                }
                required
              />
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-2 gap-2">
            <div className="grid grid-cols-2 gap-2 border border-slate-700 p-1.5 rounded">
              <div className="col-span-2 text-[9px] text-slate-400 font-bold text-center">
                TOP LEFT
              </div>
              <input
                type="number"
                placeholder="X"
                className="w-full bg-slate-900 border border-slate-700 rounded p-1 text-xs text-white font-mono"
                value={rectData.x1}
                onChange={(e) =>
                  setRectData({ ...rectData, x1: Number(e.target.value) })
                }
                required
              />
              <input
                type="number"
                placeholder="Y"
                className="w-full bg-slate-900 border border-slate-700 rounded p-1 text-xs text-white font-mono"
                value={rectData.y1}
                onChange={(e) =>
                  setRectData({ ...rectData, y1: Number(e.target.value) })
                }
                required
              />
            </div>
            <div className="grid grid-cols-2 gap-2 border border-slate-700 p-1.5 rounded">
              <div className="col-span-2 text-[9px] text-slate-400 font-bold text-center">
                BOT RIGHT
              </div>
              <input
                type="number"
                placeholder="X"
                className="w-full bg-slate-900 border border-slate-700 rounded p-1 text-xs text-white font-mono"
                value={rectData.x2}
                onChange={(e) =>
                  setRectData({ ...rectData, x2: Number(e.target.value) })
                }
                required
              />
              <input
                type="number"
                placeholder="Y"
                className="w-full bg-slate-900 border border-slate-700 rounded p-1 text-xs text-white font-mono"
                value={rectData.y2}
                onChange={(e) =>
                  setRectData({ ...rectData, y2: Number(e.target.value) })
                }
                required
              />
            </div>
          </div>
        )}

        {/* Row 3: Duration, Frequency & Submit */}
        <div className="grid grid-cols-4 gap-2 items-end mt-1">
          <div className="col-span-1">
            <label className="text-[10px] text-slate-500 font-bold block mb-1">
              DUR (S)
            </label>
            <input
              type="number"
              className="w-full bg-slate-800 border border-slate-700 rounded p-1.5 text-xs text-white focus:border-blue-500 outline-none font-mono"
              value={commonData.duration}
              onChange={(e) =>
                setCommonData({
                  ...commonData,
                  duration: Number(e.target.value),
                })
              }
              required
            />
          </div>
          <div className="col-span-1">
            <label className="text-[10px] text-slate-500 font-bold block mb-1">
              FREQ (S)
            </label>
            <input
              type="number"
              className="w-full bg-slate-800 border border-slate-700 rounded p-1.5 text-xs text-white focus:border-blue-500 outline-none font-mono"
              value={commonData.frequency}
              onChange={(e) =>
                setCommonData({
                  ...commonData,
                  frequency: Number(e.target.value),
                })
              }
              required
              min="1"
            />
          </div>
          <button
            type="submit"
            disabled={loading}
            className="col-span-2 bg-blue-600 hover:bg-blue-500 text-white text-xs font-bold py-2 rounded flex justify-center items-center gap-2 transition disabled:opacity-50 disabled:cursor-not-allowed h-[34px]"
          >
            {loading ? (
              "SENDING..."
            ) : (
              <>
                <PlusCircle size={14} /> UPLOAD
              </>
            )}
          </button>
        </div>

        {/* Status Messages */}
        {error && (
          <div className="text-[10px] text-red-400 flex items-center gap-1 bg-red-900/20 p-1.5 rounded">
            <AlertCircle size={10} /> {error}
          </div>
        )}
        {success && (
          <div className="text-[10px] text-green-400 flex items-center gap-1 bg-green-900/20 p-1.5 rounded">
            <CheckCircle2 size={10} /> Mission Uploaded Successfully
          </div>
        )}
      </form>
    </div>
  );
}
