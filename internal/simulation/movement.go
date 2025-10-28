// package simulation

// // Functions:
// - UpdatePosition(current Position, velocity Velocity, dt float64) Position
// - CalculateVelocity(target, current Position, maxSpeed float64) Velocity
// - IsInGeographicArea(pos Position, area GeographicArea) bool
// - PickRandomTargetInArea(area GeographicArea) Position
// - CalculateDistanceToTarget(current, target Position) float64

// // Implements:
// - Position updates based on velocity
// - Movement within mission geographic area
// - Path planning (random waypoints or direct path)
// - Collision avoidance (optional)

package simulation

// import (
// 	"math"
// 	"math/rand"
// 	"space-mission/pkg/models"
// )

// // Atualiza a posição usando velocidade (m/s) e dt (em segundos)
// func UpdatePosition(current models.Position, velocity models.Velocity, dt float64) models.Position {
// 	rad := velocity.Direction * math.Pi / 180.0
// 	dx := velocity.Speed * math.Cos(rad) * dt
// 	dy := velocity.Speed * math.Sin(rad) * dt
// 	return models.Position{
// 		X: current.X + dx,
// 		Y: current.Y + dy,
// 		Z: current.Z, // 2D; adapta se em 3D
// 	}
// }

// // Calcula a velocity (direção+modulo) necessária para ir do current ao target (até maxSpeed)
// func CalculateVelocity(target, current models.Position, maxSpeed float64) models.Velocity {
// 	dx := target.X - current.X
// 	dy := target.Y - current.Y
// 	distance := math.Hypot(dx, dy)
// 	dir := math.Atan2(dy, dx) * 180 / math.Pi
// 	speed := maxSpeed
// 	if distance < maxSpeed {
// 		speed = distance // trava ao chegar perto
// 	}
// 	return models.Velocity{
// 		Speed:     speed,
// 		Direction: dir,
// 	}
// }

// // Verifica se uma posição está dentro da área de missão (usando bounding box retangular)
// func IsInGeographicArea(pos models.Position, area models.GeographicArea) bool {
// 	return pos.X >= area.X1 && pos.X <= area.X2 &&
// 		pos.Y >= area.Y1 && pos.Y <= area.Y2
// }

// // Escolhe waypoint aleatório dentro da área
// func PickRandomTargetInArea(area models.GeographicArea) models.Position {
// 	x := area.X1 + rand.Float64()*(area.X2-area.X1)
// 	y := area.Y1 + rand.Float64()*(area.Y2-area.Y1)
// 	return models.Position{X: x, Y: y, Z: 0}
// }

// // Distância Euclideana 2D entre dois pontos
// func CalculateDistanceToTarget(current, target models.Position) float64 {
// 	dx := current.X - target.X
// 	dy := current.Y - target.Y
// 	return math.Hypot(dx, dy)
// }
