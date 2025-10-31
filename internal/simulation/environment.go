package simulation

import (
	"math"
	"math/rand"
	"time"
)

// EnvironmentalData holds environmental sensor readings in float32
type EnvironmentalData struct {
	Temperature float32 // degrees Celsius
	Humidity    float32 // percentage (0-100)
	WindSpeed   float32 // meters per second
	Pressure    float32 // hectopascals
	TimeOfDay   float32 // normalized day/night cycle: 0 = night, 1 = day
}

// DayNightCycle returns a normalized value describing day (1) and night (0) cycle based on current time
func DayNightCycle(now time.Time) float32 {
	hour := float32(now.Hour()) + float32(now.Minute())/60.0
	return 0.5 + 0.5*float32(math.Sin(float64((hour-7)*math.Pi/12)))
}

// GenerateEnvironmentalData creates a randomized environmental snapshot at a given time
func GenerateEnvironmentalData(now time.Time) EnvironmentalData {
	tod := DayNightCycle(now)

	// Temperature varies from 15°C at night to 30°C at day with ±2°C noise
	tBase := 15 + 15*tod
	temperature := tBase + float32(rand.NormFloat64()*2)

	// Humidity inversely relates to temperature with ±5% noise, clamped 0-100%
	humidity := 90 - 40*tod + float32(rand.NormFloat64()*5)
	if humidity > 100 {
		humidity = 100
	} else if humidity < 0 {
		humidity = 0
	}

	// Wind speed fluctuates between ~2 and 8 m/s with noise
	wind := float32(2 + 6*rand.Float64() + rand.NormFloat64())

	// Pressure has daily sinusoidal variation around 1000 hPa with noise
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	elapsedHours := float32(now.Sub(midnight).Hours())
	pressure := 1000 + 5*float32(math.Sin(float64(elapsedHours)/24*2*math.Pi)) + float32(rand.NormFloat64()*3)

	return EnvironmentalData{
		Temperature: temperature,
		Humidity:    humidity,
		WindSpeed:   wind,
		Pressure:    pressure,
		TimeOfDay:   tod,
	}
}
