package models

// ============================================================================
// Spacial Data Types
// ============================================================================

// GeoPoint represents a geographic location using latitude and longitude.
// This type is used consistently throughout the codebase for all coordinate pairs.
type GeoPoint struct {
	Latitude  float64 // Latitude in degrees (-90 to 90)
	Longitude float64 // Longitude in degrees (-180 to 180)
}

// Velocity represents the movement vector of a rover.
// Contains both the speed and direction of movement.
type Velocity struct {
	Speed     float64 // Speed in meters per second (non-negative)
	Direction float64 // Direction in degrees (0-360), where 0 is North and 90 is East
}

// ============================================================================
// Shape Types and Definitions
// ============================================================================

// Shape represents the geometric shape of a geographic area.
type Shape uint8

// Available shape types for geographic areas.
const (
	ShapeCircle    Shape = iota + 1 // Circular geographic area
	ShapeRectangle                  // Rectangular geographic area
)

// String returns the string representation of a Shape.
func (s Shape) String() string {
	switch s {
	case ShapeCircle:
		return "circle"
	case ShapeRectangle:
		return "rectangle"
	default:
		return "?"
	}
}

// ============================================================================
// Coordinate Types and Definitions
// ============================================================================

// CoordsCircle represents the coordinates of a circular area.
// Center defines the middle point and Radius defines the size of the circle.
type CoordsCircle struct {
	Center GeoPoint // Center point as latitude/longitude coordinates
	Radius float64  // Radius in meters
}

// CoordsRectangle represents the coordinates of a rectangular area.
// TopLeft and BottomRight define opposite corners of the rectangle.
type CoordsRectangle struct {
	TopLeft     GeoPoint // Top-left corner as latitude/longitude coordinates
	BottomRight GeoPoint // Bottom-right corner as latitude/longitude coordinates
}

// Coordinates is an interface that all coordinate types must implement.
// This allows different shape coordinates to be treated uniformly.
type Coordinates interface {
	isCoords()
}

// Implementations of the Coordinates interface.
// These empty methods satisfy the Coordinates interface.
func (c CoordsCircle) isCoords()    {}
func (r CoordsRectangle) isCoords() {}

// ============================================================================
// Geographic Area Types and Definitions
// ============================================================================

// GeographicArea represents a geographic region with a defined shape and coordinates.
// Used to specify areas where rovers should operate or missions should take place.
type GeographicArea struct {
	Shape  Shape       // Type of shape (circle or rectangle)
	Coords Coordinates // Specific coordinates based on the shape type
}

// ============================================================================
// Operational State Types and Definitions
// ============================================================================

// OperationalState represents the current operational mode of a rover.
// Used to track what state the rover is currently in.
type OperationalState uint8

// Available operational states that a rover can be in.
const (
	StateIdle      OperationalState = iota + 1 // Rover is stationary and idle
	StateOnMission                             // Rover is actively executing a mission
	StateError                                 // Rover encountered an error condition
	StateUnknown                               // Rover state is unknown
)

// String returns the string representation of an OperationalState.
func (os OperationalState) String() string {
	switch os {
	case StateIdle:
		return "idle"
	case StateOnMission:
		return "on_mission"
	case StateError:
		return "error"
	case StateUnknown:
		return "unknown"
	default:
		return "?"
	}
}

// ============================================================================
// Health Status Types and Definitions
// ============================================================================

// HealthStatus represents the health status of a system or subsystem.
// Indicates whether a component is functioning normally or has issues.
type HealthStatus uint8

// Health status levels for rover systems.
// Each level represents the operational status of a subsystem.
const (
	HealthOK       HealthStatus = iota + 1 // System is operating normally
	HealthWarning                          // System has minor issues but is functional
	HealthCritical                         // System has critical issues and may fail
	HealthUnknown                          // System status is unknown or uninitialized
)

// String returns the string representation of a HealthStatus.
func (hs HealthStatus) String() string {
	switch hs {
	case HealthOK:
		return "ok"
	case HealthWarning:
		return "warning"
	case HealthCritical:
		return "critical"
	case HealthUnknown:
		return "unknown"
	default:
		return "?"
	}
}

// SystemHealth represents the health status of all major rover subsystems.
// Each field indicates the operational status of a specific subsystem.
// This is used to monitor the overall health of the rover.
type SystemHealth struct {
	Motors      HealthStatus // Motor system status
	Sensors     HealthStatus // Sensor array status
	PowerSystem HealthStatus // Power and battery system status
}

// ============================================================================
// Task Types and Definitions
// ============================================================================

// Task represents the type of task a rover can perform during a mission.
type Task uint8

// Available task types that can be assigned to a rover.
const (
	TaskSampleCollection        Task = iota + 1 // Collect physical samples from the environment
	TaskImageCapture                            // Capture images and video data
	TaskEnvironmentalMonitoring                 // Monitor environmental conditions
	TaskTerrainMapping                          // Map and analyze terrain features
)

// String returns the string representation of a Task.
// This implements the Stringer interface for convenient string conversion.
func (t Task) String() string {
	switch t {
	case TaskSampleCollection:
		return "sample_collection"
	case TaskImageCapture:
		return "image_capture"
	case TaskEnvironmentalMonitoring:
		return "environmental_monitoring"
	case TaskTerrainMapping:
		return "terrain_mapping"
	default:
		return "unknown"
	}
}

// ============================================================================
// Mission Status Types and Definitions
// ============================================================================

// MissionStatus represents the current status of a mission.
// Tracks the progression of a mission from assignment through completion.
type MissionStatus uint8

// Available mission statuses throughout the mission lifecycle.
const (
	MissionUnassigned MissionStatus = iota + 1 // Mission has not been assigned to any rover
	MissionAssigned                            // Mission has been assigned but not started
	MissionInProgress                          // Mission is currently being executed
	MissionFailed                              // Mission was not completed successfully
	MissionCompleted                           // Mission was completed successfully
)

// String returns the string representation of a MissionStatus.
// This implements the Stringer interface for convenient string conversion.
func (ms MissionStatus) String() string {
	switch ms {
	case MissionUnassigned:
		return "unassigned"
	case MissionAssigned:
		return "assigned"
	case MissionInProgress:
		return "in_progress"
	case MissionFailed:
		return "failed"
	case MissionCompleted:
		return "completed"
	default:
		return "?"
	}
}
