package models

import (
	"fmt"
	"time"
)

// MissionRequest represents a rover requesting a mission
type MissionRequest struct {
	RoverID   uint16
	Position  Position
	Timestamp time.Time
}

// MissionAssignment represents a mission assigned to a rover
type MissionAssignment struct {
	ID             uint16 // 0 should be reserved for system use
	Task           Task
	GeographicArea GeographicArea
	Status         MissionStatus
	Progress       float32 // value between 0 and 100
	MaxDuration    time.Duration
	UpdateInterval time.Duration
	Timestamp      time.Time
}

// ProgressUpdate represents mission progress from a rover
type ProgressUpdate struct {
	RoverID       uint16
	MissionID     uint16
	MissionStatus MissionStatus
	Progress      float32
	Data          string
	Timestamp     time.Time
}

// MissionMessage interface to unify all message types
type MissionMessage interface {
	isMissionMessage()
}

func (m MissionRequest) isMissionMessage()    {}
func (m MissionAssignment) isMissionMessage() {}
func (m ProgressUpdate) isMissionMessage()    {}

type Task uint8

const (
	TaskSampleCollection Task = iota + 1
	TaskImageCapture
	TaskEnvironmentalMonitoring
	TaskTerrainMapping
)

func (t Task) String() string {
	names := [...]string{
		"sample_collection",
		"image_capture",
		"environmental_monitoring",
		"terrain_mapping",
	}
	index := int(t) - 1
	if index < 0 || index >= len(names) {
		return fmt.Sprintf("Task(%d)", t)
	}
	return names[index]
}

type GeographicArea struct {
	Shape       Shape
	Coordinates Coords
}

type MissionStatus uint8

const (
	MissionUnassigned MissionStatus = iota + 1
	MissionAssigned
	MissionInProgress
	MissionFailed
	MissionCompleted
)

func (ms MissionStatus) String() string {
	names := [...]string{
		"unassigned",
		"assigned",
		"in_progress",
		"failed",
		"completed",
	}
	index := int(ms) - 1
	if index < 0 || index >= len(names) {
		return fmt.Sprintf("MissionStatus(%d)", ms)
	}
	return names[index]
}
