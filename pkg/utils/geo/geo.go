package geo

import (
	"math"
	"space-mission/pkg/models"
	"space-mission/pkg/utils/safe"
)

func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func SquaredDistance(p1, p2 models.Point) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return dx*dx + dy*dy
}

func GetIntegerPointsInArea(area models.GeographicArea) *safe.List[models.Point] {
	pointsQueue := safe.NewList[models.Point]()

	switch coords := area.Coords.(type) {
	case models.CoordsCircle:
		minX := int(math.Floor(coords.Center.X - coords.Radius))
		maxX := int(math.Ceil(coords.Center.X + coords.Radius))
		minY := int(math.Floor(coords.Center.Y - coords.Radius))
		maxY := int(math.Ceil(coords.Center.Y + coords.Radius))
		squaredRadius := coords.Radius * coords.Radius

		for x := minX; x <= maxX; x++ {
			for y := minY; y <= maxY; y++ {
				p := models.Point{X: float64(x), Y: float64(y)}
				if SquaredDistance(p, coords.Center) <= squaredRadius {
					pointsQueue.PushBack(p)
				}
			}
		}

	case models.CoordsRectangle:
		minRectX := math.Min(coords.TopLeft.X, coords.BottomRight.X)
		maxRectX := math.Max(coords.TopLeft.X, coords.BottomRight.X)
		minRectY := math.Min(coords.TopLeft.Y, coords.BottomRight.Y)
		maxRectY := math.Max(coords.TopLeft.Y, coords.BottomRight.Y)

		startX := int(math.Ceil(minRectX))
		endX := int(math.Floor(maxRectX))
		startY := int(math.Ceil(minRectY))
		endY := int(math.Floor(maxRectY))

		for x := startX; x <= endX; x++ {
			for y := startY; y <= endY; y++ {
				pointsQueue.PushBack(models.Point{X: float64(x), Y: float64(y)})
			}
		}
	}

	return pointsQueue
}

func GetAreaCenter(area models.GeographicArea) models.Point {
	switch coords := area.Coords.(type) {
	case models.CoordsCircle:
		return coords.Center
	case models.CoordsRectangle:
		centerX := (coords.TopLeft.X + coords.BottomRight.X) / 2.0
		centerY := (coords.TopLeft.Y + coords.BottomRight.Y) / 2.0
		return models.Point{X: centerX, Y: centerY}
	}
	return models.Point{}
}

// GenerateShortestPath now populates a queue instead of a slice.
// The resulting path is enqueued into the provided `pathQueue`.
func GenerateShortestPath(startPoint models.Point, area models.GeographicArea, pathQueue *safe.List[models.Point]) {
	// 1. Get all integer points within the area.
	pointsInAreaQueue := GetIntegerPointsInArea(area)

	// 2. If no integer points are found, enqueue the center of the area and return.
	if pointsInAreaQueue.IsEmpty() {
		centerPoint := GetAreaCenter(area)
		pathQueue.PushBack(centerPoint) // MODIFIED: Enqueue instead of append
		return
	}

	// 3. Prepare for the Nearest Neighbor algorithm using a map for efficiency.
	unvisitedPoints := make(map[models.Point]bool)
	allPoints := make([]models.Point, 0, pointsInAreaQueue.Size())
	for !pointsInAreaQueue.IsEmpty() {
		p, _ := pointsInAreaQueue.PopFront()
		unvisitedPoints[p] = true
		allPoints = append(allPoints, p)
	}

	// This slice will temporarily hold the ordered path.
	generatedPath := make([]models.Point, 0, len(allPoints))
	currentPoint := startPoint

	// 4. Build the path using the Nearest Neighbor heuristic.
	for i := 0; i < len(allPoints); i++ {
		closestPoint := models.Point{}
		minDist := math.MaxFloat64
		foundPoint := false

		for _, p := range allPoints {
			if _, ok := unvisitedPoints[p]; ok {
				dist := SquaredDistance(currentPoint, p)
				if dist < minDist {
					minDist = dist
					closestPoint = p
					foundPoint = true
				}
			}
		}

		if !foundPoint {
			break
		}

		generatedPath = append(generatedPath, closestPoint)
		currentPoint = closestPoint
		delete(unvisitedPoints, closestPoint)
	}

	// 5. Enqueue the final sorted path into the output queue.
	for _, p := range generatedPath {
		pathQueue.PushBack(p)
	}
}

func DistanceToArea(p models.Point, area models.GeographicArea) float64 {
	switch area.Shape {
	case models.ShapeCircle:
		coords := area.Coords.(models.CoordsCircle)
		dist := distance(p, coords.Center)
		if dist <= coords.Radius {
			return 0
		}
		return dist - coords.Radius
	case models.ShapeRectangle:
		coords := area.Coords.(models.CoordsRectangle)

		// Check if the point is inside the rectangle
		if p.X >= coords.TopLeft.X && p.X <= coords.BottomRight.X &&
			p.Y <= coords.TopLeft.Y && p.Y >= coords.BottomRight.Y {
			return 0
		}

		// Find the closest point on the rectangle's boundary
		closestX := p.X
		if p.X < coords.TopLeft.X {
			closestX = coords.TopLeft.X
		} else if p.X > coords.BottomRight.X {
			closestX = coords.BottomRight.X
		}

		closestY := p.Y
		if p.Y > coords.TopLeft.Y {
			closestY = coords.TopLeft.Y
		} else if p.Y < coords.BottomRight.Y {
			closestY = coords.BottomRight.Y
		}

		closestPoint := models.Point{X: closestX, Y: closestY}
		return distance(p, closestPoint)
	}
	return 0
}

// distance calculates the Euclidean distance between two points.
func distance(p1, p2 models.Point) float64 {
	dx := p1.X - p2.X
	dy := p1.Y - p2.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// epsilon defines the margin of error for floating point comparison.
const epsilon = 1e-9

func EqualPoints(p1 models.Point, p2 models.Point) bool {
	return math.Abs(p1.X-p2.X) < epsilon && math.Abs(p1.Y-p2.Y) < epsilon
}
