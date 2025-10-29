package models

import "time"

type Mission struct {
	ID             string
	GeographicArea GeographicArea
	Task           Task
	MaxDuration    time.Duration
	UpdateInterval time.Duration
	Status         MissionStatus
	Progress       float64
}

type Task string

const (
	TaskSampleCollection        Task = "sample_collection"
	TaskImageCapture            Task = "image_capture"
	TaskEnvironmentalMonitoring Task = "environmental_monitoring"
	TaskTerrainMapping          Task = "terrain_mapping"
	// other tasks can be added here
)

type GeographicArea struct {
	Type        string // "rectangle", "circle"
	Coordinates any
}

type MissionStatus string

const (
	MissionPending    MissionStatus = "pending"
	MissionInProgress MissionStatus = "in_progress"
	MissionPaused     MissionStatus = "paused"
	MissionCompleted  MissionStatus = "completed"
)
