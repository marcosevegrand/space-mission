package simulation

// // // Functions:
// // - GenerateEnvironmentalData() EnvironmentalData
// // - SimulateTemperature() float64
// // - SimulateWindSpeed() float64
// // - SimulateAtmosphericPressure() float64

// // // Implements:
// // - Random environmental data generation
// // - Realistic value ranges (temperature, humidity, wind)
// // - Time-based variations (day/night cycles)

// import (
// 	"math"
// 	"math/rand"
// 	"time"
// )

// // EnvironmentalData representa as condições ambientais num instante
// type EnvironmentalData struct {
// 	Temperature float64 // °C
// 	Humidity    float64 // 0-100%
// 	WindSpeed   float64 // m/s
// 	Pressure    float64 // hPa
// 	TimeOfDay   float64 // [0.0, 1.0] (dia=1, noite=0)
// }

// // Simula ciclo dia/noite simplificado: [0...1] ao longo de 24h
// fpackagunc dayNightCycle(now time.Time) float64 {
// 	hour := now.Hour() + float64(now.Minute())/60
// 	// picos de 1.0 às 14h, mínimos de 0.0 às 2h/4h
// 	return 0.5 + 0.5*math.Sin((hour-7)*math.Pi/12)
// }

// // Gera dados ambientais com ruído randômico e variação horária
// func GenerateEnvironmentalData(now time.Time) EnvironmentalData {
// 	tod := dayNightCycle(now)

// 	// Temperatura: variação base 15°C noite, até 30°C dia, com ruído ±2°C
// 	tBase := 15 + 15*tod
// 	temperature := tBase + rand.NormFloat64()*2

// 	// Humidade: inversamente proporcional à temperatura, ruído ±5%
// 	humidity := 90 - 40*tod + rand.NormFloat64()*5
// 	if humidity > 100 {
// 		humidity = 100
// 	}
// 	if humidity < 0 {
// 		humidity = 0
// 	}

// 	// Vento: base 2~8 m/s, com flutuações rápidas
// 	wind := 2 + 6*rand.Float64() + rand.NormFloat64()

// 	// Pressão atmosférica: 1000±10 hPa + variações lentas dia/noite
// 	pressure := 1000 + 5*math.Sin(now.Sub(time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())).Hours()/24*2*math.Pi) + rand.NormFloat64()*3

// 	return EnvironmentalData{
// 		Temperature: temperature,
// 		Humidity:    humidity,
// 		WindSpeed:   wind,
// 		Pressure:    pressure,
// 		TimeOfDay:   tod,
// 	}
// }

// func SimulateTemperature(now time.Time) float64 {
// 	data := GenerateEnvironmentalData(now)
// 	return data.Temperature
// }

// func SimulateWindSpeed(now time.Time) float64 {
// 	data := GenerateEnvironmentalData(now)
// 	return data.WindSpeed
// }

// func SimulateAtmosphericPressure(now time.Time) float64 {
// 	data := GenerateEnvironmentalData(now)
// 	return data.Pressure
// }
