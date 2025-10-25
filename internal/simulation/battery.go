// package simulation

// // Functions:
// - ConsumeBattery(current float64, rate float64, dt time.Duration) float64
// - GetConsumptionRate(state OperationalState, velocity float64) float64
// - CheckLowBattery(level float64) bool
// - SimulateBatteryDrain(rover *Rover)

// // Implements:
// - Battery consumption based on activity (idle < moving < mission)
// - Higher consumption when moving faster
// - Low battery alerts (<20%)
// - Critical battery shutdown (<5%)
