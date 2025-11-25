package models

import (
	"fmt"
	"time"
)

// MissionRequest represents a rover request to be assigned a mission.
// Sent by a rover to indicate it is ready for task assignment.
type MissionRequest struct {
	RoverID   uint16    // Unique identifier for the rover requesting a mission
	Position  Point     // Current geographic location of the rover
	Timestamp time.Time // Time when the request was generated
}

func (m *MissionRequest) String() string {
	return fmt.Sprintf(
		"-- Mission Request --\n"+
			"  Rover ID:    %03d\n"+
			"  Position:    (%.3f, %.3f)\n"+
			"  Timestamp:   %s",
		m.RoverID,
		m.Position.X, m.Position.Y,
		m.Timestamp.Format(time.RFC3339Nano),
	)
}

// MissionAssignment represents a mission assigned to a rover.
// Contains all information needed for the rover to execute the mission.
type MissionAssignment struct {
	MissionID       uint16         // Unique identifier for the mission
	Task            Task           // Type of task to perform
	Area            GeographicArea // Geographic area where the mission should be performed
	Status          MissionStatus  // Current status of the mission
	Progress        float64        // Mission progress percentage (0-100)
	MaxDuration     time.Duration  // Maximum allowed time to complete the mission
	UpdateFrequency time.Duration  // Frequency of mission progress updates requested
	Timestamp       time.Time      // Time when the assignment was created
}

func (m *MissionAssignment) String() string {
	return fmt.Sprintf(
		"-- Mission Assignment --\n"+
			"  Mission ID:    %03d\n"+
			"  Task:          %v\n"+
			"  Area:          %v\n"+
			"  Status:        %s\n"+
			"  Progress:      %.2f%%\n"+
			"  Max Duration:  %s\n"+
			"  Update Freq:   %s\n"+
			"  Timestamp:     %s",
		m.MissionID,
		m.Task,
		m.Area,
		m.Status,
		m.Progress,
		m.MaxDuration,
		m.UpdateFrequency,
		m.Timestamp.Format(time.RFC3339Nano),
	)
}

// MissionUpdate represents a mission progress update sent by a rover.
// Provides status information and completion metrics for an assigned mission.
type MissionUpdate struct {
	RoverID   uint16        // Unique identifier for the rover sending the update
	MissionID uint16        // Unique identifier of the mission being updated
	Status    MissionStatus // Current status of the mission
	Progress  float64       // Mission progress percentage (0-100)
	Data      string        // Additional mission-specific data or observations
	Timestamp time.Time     // Time when the update was generated
}

func (m *MissionUpdate) String() string {
	return fmt.Sprintf(
		"-- Mission Update --\n"+
			"  Rover ID:      %03d\n"+
			"  Mission ID:    %03d\n"+
			"  Status:        %s\n"+
			"  Progress:      %.2f%%\n"+
			"  Data:          %s\n"+
			"  Timestamp:     %s",
		m.RoverID,
		m.MissionID,
		m.Status,
		m.Progress,
		m.Data,
		m.Timestamp.Format(time.RFC3339Nano),
	)
}

// MissionMessage is an interface that all mission message types must implement.
// This allows different mission messages to be treated uniformly.
type MissionMessage interface {
	isMissionMessage()
}

// Implementations of the MissionMessage interface.
// These empty methods satisfy the MissionMessage interface.
func (m *MissionRequest) isMissionMessage()    {}
func (m *MissionAssignment) isMissionMessage() {}
func (m *MissionUpdate) isMissionMessage()     {}
