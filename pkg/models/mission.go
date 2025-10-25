package models

import "time"

type Mission struct {
	ID             string
	GeographicArea GeographicArea
	Task           Task
	Duration       time.Duration
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

type Circle struct {
	Center [2]float64
	Radius float64
}

type Rectangle struct {
	TopLeft     [2]float64
	BottomRight [2]float64
}

type GeographicArea struct {
	Type        string // "rectangle", "circle"
	Coordinates interface{}
}

type MissionStatus string

const (
	MissionPending    MissionStatus = "pending"
	MissionAssigned   MissionStatus = "assigned"
	MissionInProgress MissionStatus = "in_progress"
	MissionCompleted  MissionStatus = "completed"
)
