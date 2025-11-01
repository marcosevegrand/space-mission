package models

import (
	"time"
)

type Mission struct {
	ID             uint16
	Task           Task
	GeographicArea GeographicArea
	Status         MissionStatus
	Progress       float32 // value between 0 and 100
}

type Task uint8

const (
	TaskSampleCollection Task = iota + 1
	TaskImageCapture
	TaskEnvironmentalMonitoring
	TaskTerrainMapping
)

func (t Task) String() string {
	return [...]string{
		"sample_collection",
		"image_capture",
		"environmental_monitoring",
		"terrain_mapping",
	}[t-1]
}

type GeographicArea struct {
	Shape       Shape
	Coordinates Coords
}

type MissionStatus uint8

const (
	MissionPending MissionStatus = iota + 1
	MissionInProgress
	MissionPaused
	MissionCompleted
)

func (ms MissionStatus) String() string {
	return [...]string{
		"pending",
		"in_progress",
		"paused",
		"completed",
	}[ms-1]
}

// MissionRequest represents a rover requesting a mission
type MissionRequest struct {
	RoverID   uint16
	Timestamp time.Time
}

// MissionAssignment represents a mission assigned to a rover
type MissionAssignment struct {
	RoverID        uint16
	MaxDuration    time.Duration
	UpdateInterval time.Duration
	Mission        Mission
	Timestamp      time.Time
}

// ProgressUpdate represents mission progress from a rover
type ProgressUpdate struct {
	RoverID   uint16
	MissionID uint16
	Status    MissionStatus
	Progress  float32
	Content   string
	Timestamp time.Time
}

// MissionMessage interface to unify all message types
type MissionMessage interface {
	isMissionMessage()
}

func (m MissionRequest) isMissionMessage()    {}
func (m MissionAssignment) isMissionMessage() {}
func (m ProgressUpdate) isMissionMessage()    {}
