package models

import (
	"time"
)

type Mission struct {
	ID             uint32
	GeographicArea GeographicArea
	Task           Task
	MaxDuration    time.Duration
	UpdateInterval time.Duration
	Status         MissionStatus
	Progress       float64
}

type GeographicArea struct {
	Type        string // "rectangle", "circle"
	Coordinates any
}

type Task string

const (
	TaskSampleCollection        Task = "sample_collection"
	TaskImageCapture            Task = "image_capture"
	TaskEnvironmentalMonitoring Task = "environmental_monitoring"
	TaskTerrainMapping          Task = "terrain_mapping"
	// other tasks can be added here
)

type MissionStatus string

const (
	MissionPending    MissionStatus = "pending"
	MissionInProgress MissionStatus = "in_progress"
	MissionPaused     MissionStatus = "paused"
	MissionCompleted  MissionStatus = "completed"
)

// MissionRequest represents a rover requesting a mission
type MissionRequest struct {
	RoverID   uint16
	Timestamp time.Time
}

// MissionAssignment represents a mission assigned to a rover
type MissionAssignment struct {
	RoverID        uint16
	MissionID      string
	GeographicArea GeographicArea
	Task           Task
	MaxDuration    time.Duration
	UpdateInterval time.Duration
	Timestamp      time.Time
}

// ProgressUpdate represents mission progress from a rover
type ProgressUpdate struct {
	RoverID         uint16
	MissionID       string
	Status          MissionStatus
	Progress        float64
	CurrentPosition Position
	Timestamp       time.Time
}

// MissionMessage interface to unify all message types
type MissionMessage interface {
	isMissionMessage()
}

// Implement the interface for each struct

func (m MissionAssignment) isMissionMessage() {}
func (m ProgressUpdate) isMissionMessage()    {}
func (m MissionRequest) isMissionMessage()    {}
