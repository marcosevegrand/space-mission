// src/types.ts

export interface Point {
  X: number;
  Y: number;
}

export interface Velocity {
  X: number;
  Y: number;
}

// --- ENUMS (Matching Go iota + 1) ---

export enum Shape {
  Circle = 1,
  Rectangle = 2,
}

export enum OperationalState {
  Idle = 1,
  On_Mission = 2,
  Error = 3,
  Unknown = 4,
}

export enum HealthStatus {
  OK = 1,
  Warning = 2,
  Critical = 3,
  Unknown = 4,
}

export enum Task {
  Sample_Analysis = 1,
  Image_Capture = 2,
  Environmental_Monitoring = 3,
  Terrain_Mapping = 4,
}

export enum MissionStatus {
  Unassigned = 1,
  Assigned = 2,
  In_Progress = 3,
  Failed = 4,
  Completed = 5,
  Unknown = 6,
}

// --- COMPLEX TYPES ---

export interface SystemHealth {
  Motors: HealthStatus;
  Sensors: HealthStatus;
  PowerSystem: HealthStatus;
}

export interface CoordsCircle {
  Center: Point;
  Radius: number;
}

export interface CoordsRectangle {
  TopLeft: Point;
  BottomRight: Point;
}

export interface GeographicArea {
  Shape: Shape;
  // In JSON, this will be one of the concrete structs depending on Shape
  Coords: CoordsCircle | CoordsRectangle;
}

// --- MAIN ENTITIES ---

export interface Telemetry {
  RoverID: number;
  Position: Point;
  OperationalState: OperationalState;
  BatteryPercentage: number;
  Velocity: Velocity;
  Temperature: number;
  SystemHealth: SystemHealth;
  Timestamp: string;
}

export interface MissionAssignment {
  MissionID: number;
  RoverID: number;
  Task: Task;
  Area: GeographicArea;
  Status: MissionStatus;
  Progress: number;
  MaxDuration: number;
  UpdateFrequency: number;
  Timestamp: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data: T;
}
