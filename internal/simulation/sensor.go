package simulation

import (
	"math"
	"math/rand/v2"
	"space-mission/pkg/models"
	"time"
)

const (
	// Internal temperature constants
	initialInternalTemperature     = 30.0 // Rover's internal temperature in Celsius
	maxInternalTemperatureVariance = 5.0  // Maximum internal temperature variance in Celsius
)

type SensorModule struct {
	health              models.HealthStatus
	roverPosition       models.Point
	internalTemperature float64
	humidity            float64
	pressure            float64
	co2                 float64
	carbon              float64
	hydrogen            float64
	oxygen              float64
	temperature         float64
	lastCallTemp        time.Time
	lastCallELev        time.Time
	elevation           float64
}

type ImageMetadata struct {
	Timestamp   time.Time
	Resolution  string
	Position    models.Point
	Description string
}

func NewSensorModule() *SensorModule {
	return &SensorModule{
		health:              models.HealthOK,
		roverPosition:       models.Point{X: 0, Y: 0},
		internalTemperature: initialInternalTemperature,
		humidity:            50.0,
		pressure:            1013.25,
		co2:                 400.0,
		lastCallTemp:        time.Now(),
		lastCallELev:        time.Now(),
		elevation:           0.0,
	}
}

func (s *SensorModule) GetHealth(deltaT time.Duration) models.HealthStatus {
	if deltaT == 0 {
		return s.health
	}

	randomValue := rand.Float64() * 100.0

	if randomValue < (probWarning*deltaT.Seconds()) && s.health == models.HealthOK {
		s.health = models.HealthWarning
		return s.health
	}

	if randomValue < (probCritical * deltaT.Seconds()) {
		s.health = models.HealthCritical
		return s.health
	}

	return s.health
}

func (s *SensorModule) GetInternalTemperature(deltaT time.Duration) float64 {
	// Convert deltaT to seconds
	seconds := deltaT.Seconds()

	// Calculate maximum allowed temperature change for this time slice
	maxDelta := 0.5 * seconds

	// Randomly choose a temperature delta from -maxDelta to +maxDelta
	deltaTemp := (rand.Float64()*2 - 1) * maxDelta

	// Apply the change
	newTemp := s.internalTemperature + deltaTemp

	// Clamp the temperature within ±5 degrees of the initial temperature
	minTemp := initialInternalTemperature - maxInternalTemperatureVariance
	maxTemp := initialInternalTemperature + maxInternalTemperatureVariance
	if newTemp < minTemp {
		newTemp = minTemp
	} else if newTemp > maxTemp {
		newTemp = maxTemp
	}

	s.internalTemperature = newTemp
	return s.internalTemperature
}

// GetPosition calculates the new position of an object after a time interval,
// given its initial position and constant velocity in a Cartesian coordinate system.
func (s *SensorModule) GetPosition(deltaT time.Duration, initial models.Point, v models.Velocity) models.Point {
	if deltaT == 0 {
		return s.roverPosition
	}

	// Calculate the time elapsed in seconds.
	timeSeconds := deltaT.Seconds()

	// Calculate displacement on each axis.
	// Displacement = Velocity * Time
	deltaX := v.X * timeSeconds
	deltaY := v.Y * timeSeconds

	// Calculate the new position by adding the displacement to the initial position.
	newPosition := models.Point{
		X: initial.X + deltaX,
		Y: initial.Y + deltaY,
	}

	// --- Snapping Logic ---
	// If the new position is extremely close to an integer, round it.
	// This prevents floating-point errors from stopping the rover just short of its target.

	// Find the closest integer for the X coordinate.
	roundedX := math.Round(newPosition.X)
	// Check if the difference is within the toleranceDistance threshold.
	if math.Abs(newPosition.X-roundedX) <= toleranceDistance {
		newPosition.X = roundedX
	}

	// Find the closest integer for the Y coordinate.
	roundedY := math.Round(newPosition.Y)
	// Check if the difference is within the toleranceDistance threshold.
	if math.Abs(newPosition.Y-roundedY) <= toleranceDistance {
		newPosition.Y = roundedY
	}

	s.roverPosition = newPosition

	return newPosition
}

// Simulate environmental sensor readings with slight random variations
func (s *SensorModule) GetHumidity() float64 {
	// Simulate humidity changes within a range of ±2%
	deltaHumidity := (rand.Float64()*2 - 1) * 2.0
	newHumidity := s.humidity + deltaHumidity

	// Clamp humidity between 0% and 100%
	if newHumidity < 0 {
		newHumidity = 0
	} else if newHumidity > 100 {
		newHumidity = 100
	}

	s.humidity = newHumidity
	return s.humidity
}

func (s *SensorModule) GetTemperature(lastCallTemp time.Time) float64 {
	// Calculate time difference since last call
	timeDiff := time.Since(s.lastCallTemp).Seconds()

	// Simulate temperature changes within a range of ±1.5 degrees Celsius
	deltaTemperature := (rand.Float64()*2 - 1) * 1.5
	newTemperature := s.temperature + deltaTemperature*timeDiff

	// Clamp temperature to a reasonable range (e.g., -10 to 45 degrees Celsius)
	if newTemperature < -10 {
		newTemperature = -10
	} else if newTemperature > 45 {
		newTemperature = 45
	}

	s.temperature = newTemperature
	s.lastCallTemp = time.Now()
	return s.temperature
}

func (s *SensorModule) GetPressure() float64 {
	// Simulate pressure changes within a range of ±1 hPa
	deltaPressure := (rand.Float64()*2 - 1) * 1.0
	newPressure := s.pressure + deltaPressure

	// Clamp pressure to a reasonable range (e.g., 900 to 1100 hPa)
	if newPressure < 900 {
		newPressure = 900
	} else if newPressure > 1100 {
		newPressure = 1100
	}

	s.pressure = newPressure
	return s.pressure
}

func (s *SensorModule) GetCO2Level() float64 {
	// Simulate CO2 level changes within a range of ±10 ppm
	deltaCO2 := (rand.Float64()*2 - 1) * 10.0
	newCO2 := s.co2 + deltaCO2

	// Clamp CO2 levels to a reasonable range (e.g., 350 to 5000 ppm)
	if newCO2 < 350 {
		newCO2 = 350
	} else if newCO2 > 5000 {
		newCO2 = 5000
	}

	s.co2 = newCO2
	return s.co2
}

// SimulateSampleAnalysis simulates the analysis of soil samples
func (s *SensorModule) SimulateSampleAnalysis() {
	s.carbon = rand.Float64() * 50
	s.hydrogen = rand.Float64() * 50
	s.oxygen = rand.Float64() * 50
}

func (s *SensorModule) GetCarbon() float64 {
	return s.carbon
}

func (s *SensorModule) GetHydrogen() float64 {
	return s.hydrogen
}

func (s *SensorModule) GetOxygen() float64 {
	return s.oxygen
}

// SimulateTerrainMapping simulates terrain mapping data
func (s *SensorModule) GetElevation(lastCallELev time.Time) float64 {
	// Calculate time difference since last call
	timeDiff := time.Since(s.lastCallELev).Seconds()

	// Simulate elevation changes within a range of ±1 meters per call
	// Multiplied by timeDiff to make it proportional to time elapsed
	deltaElevation := (rand.Float64()*2 - 1) * 1.0 * timeDiff

	newElevation := s.elevation + deltaElevation

	// Clamp elevation to a reasonable range (e.g., 0 to 1500 meters)
	if newElevation < 0 {
		newElevation = 0
	} else if newElevation > 1500 {
		newElevation = 1500
	}

	s.elevation = newElevation
	s.lastCallELev = time.Now()
	return s.elevation
}

func (s *SensorModule) SimulateImageCapture(position models.Point) ImageMetadata {
	resolutions := []string{"1920x1080", "1280x720", "640x480"}
	descriptions := []string{
		"Imagem do terreno rochoso",
		"Imagem de área com areia",
		"Imagem de solo com vegetação seca",
	}
	return ImageMetadata{
		Timestamp:   time.Now(),
		Resolution:  resolutions[rand.IntN(len(resolutions))],
		Position:    position,
		Description: descriptions[rand.IntN(len(descriptions))],
	}
}
