package mothership

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"space-mission/pkg/models"
)

// RunCLI starts the interactive command line interface in a goroutine
func (m *Mothership) RunCLI() {
	scanner := bufio.NewScanner(os.Stdin)

	fmt.Println("\n=======================================================")
	fmt.Println("       MOTHERSHIP CLI STARTED")
	fmt.Println("=======================================================")
	fmt.Println("Type 'help' to see available commands.")
	fmt.Println("Type 'exit' to close this interface (server keeps running).")
	fmt.Println("-------------------------------------------------------")

	go func() {
		for {
			fmt.Print("\n[Mothership] > ")
			if !scanner.Scan() {
				break
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			parts := strings.Fields(line)
			command := strings.ToLower(parts[0])

			switch command {
			case "help":
				m.printHelp()
			case "exit":
				fmt.Println("Exited CLI")
				return
			case "add":
				// We expect at least: id, task, shape, duration, freq (5 args)
				if len(parts) < 6 {
					fmt.Println("Error: Invalid arguments. Type 'help' for usage.")
					continue
				}
				m.parseAndAddMission(parts[1:])
			case "load":
				if len(parts) < 2 {
					fmt.Println("Error: Missing filename. Usage: load <filename>")
					continue
				}
				m.loadMissionsFromFile(parts[1])
			default:
				fmt.Println("Unknown command. Type 'help'.")
			}
		}
	}()
}

func (m *Mothership) printHelp() {
	fmt.Println("\nAvailable Commands:")
	fmt.Println("  add <id> <task> <shape> <duration> <freq> [params...]")
	fmt.Println("    Adds a new mission to the pool.")
	fmt.Println("  load <filename>")
	fmt.Println("    Loads missions from a file (one set of arguments per line).")
	fmt.Println("\n    Arguments:")
	fmt.Println("      <id>       : Unique integer ID")
	fmt.Println("      <task>     : sample | image | monitor | terrain")
	fmt.Println("      <shape>    : circle | rect")
	fmt.Println("      <duration> : Max duration (e.g. 10m, 300s, 1h)")
	fmt.Println("      <freq>     : Update frequency (e.g. 3s, 500ms)")
	fmt.Println("\n    Shape Parameters:")
	fmt.Println("      circle     : <x> <y> <radius>")
	fmt.Println("      rect       : <x1> <y1> <x2> <y2>")
	fmt.Println("\n    Examples:")
	fmt.Println("      add 101 sample circle 10m 3s 10 10 5")
	fmt.Println("      load missions.txt")
}

func (m *Mothership) loadMissionsFromFile(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file '%s': %v\n", filename, err)
		return
	}
	defer file.Close()

	fmt.Printf("Loading missions from %s...\n", filename)
	scanner := bufio.NewScanner(file)
	lineNum := 0
	count := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}

		args := strings.Fields(line)

		// Be lenient if the user put "add" at the start of the line in the file
		if strings.ToLower(args[0]) == "add" {
			args = args[1:]
		}

		if len(args) < 5 {
			fmt.Printf("[Line %d] Skipped: Insufficient arguments\n", lineNum)
			continue
		}

		fmt.Printf("[Line %d] Processing: %s\n", lineNum, line)
		m.parseAndAddMission(args)
		count++
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file: %v\n", err)
	}
	fmt.Printf("Finished loading. Processed %d lines.\n", count)
}

func (m *Mothership) parseAndAddMission(args []string) {
	// 1. Parse ID
	id, err := strconv.ParseUint(args[0], 10, 16)
	if err != nil {
		fmt.Printf("Error: Invalid Mission ID '%s'\n", args[0])
		return
	}

	// 2. Parse Task
	var task models.Task
	switch strings.ToLower(args[1]) {
	case "sample", "sample_analysis":
		task = models.TaskSampleAnalysis
	case "image", "image_capture":
		task = models.TaskImageCapture
	case "monitor", "environmental_monitoring":
		task = models.TaskEnvironmentalMonitoring
	case "terrain", "terrain_mapping":
		task = models.TaskTerrainMapping
	default:
		fmt.Printf("Error: Unknown task '%s'. Valid: sample, image, monitor, terrain\n", args[1])
		return
	}

	// 3. Parse Shape Type
	shapeStr := strings.ToLower(args[2])

	// 4. Parse Duration
	maxDuration, err := time.ParseDuration(args[3])
	if err != nil {
		fmt.Printf("Error: Invalid Duration '%s' (use format 10m, 30s, etc)\n", args[3])
		return
	}

	// 5. Parse Frequency
	updateFreq, err := time.ParseDuration(args[4])
	if err != nil {
		fmt.Printf("Error: Invalid Frequency '%s' (use format 3s, 500ms, etc)\n", args[4])
		return
	}

	// 6. Parse Shape Coordinates
	params := args[5:]
	var area models.GeographicArea

	switch shapeStr {
	case "circle":
		if len(params) != 3 {
			fmt.Println("Error: Circle requires 3 params: <x> <y> <radius>")
			return
		}
		x, _ := strconv.ParseFloat(params[0], 64)
		y, _ := strconv.ParseFloat(params[1], 64)
		r, _ := strconv.ParseFloat(params[2], 64)
		area = models.GeographicArea{
			Shape: models.ShapeCircle,
			Coords: models.CoordsCircle{
				Center: models.Point{X: x, Y: y},
				Radius: r,
			},
		}

	case "rect", "rectangle":
		if len(params) != 4 {
			fmt.Println("Error: Rectangle requires 4 params: <x1> <y1> <x2> <y2>")
			return
		}
		x1, _ := strconv.ParseFloat(params[0], 64)
		y1, _ := strconv.ParseFloat(params[1], 64)
		x2, _ := strconv.ParseFloat(params[2], 64)
		y2, _ := strconv.ParseFloat(params[3], 64)
		area = models.GeographicArea{
			Shape: models.ShapeRectangle,
			Coords: models.CoordsRectangle{
				TopLeft:     models.Point{X: x1, Y: y1},
				BottomRight: models.Point{X: x2, Y: y2},
			},
		}

	default:
		fmt.Printf("Error: Unknown shape '%s'. Valid: circle, rect\n", shapeStr)
		return
	}

	// 7. Create and Add Mission
	assignment := &models.MissionAssignment{
		MissionID:       uint16(id),
		RoverID:         0, // Unassigned
		Task:            task,
		Area:            area,
		Status:          models.MissionUnassigned,
		Progress:        0,
		MaxDuration:     maxDuration,
		UpdateFrequency: updateFreq,
		Timestamp:       time.Now(),
	}

	err = m.AddMissionAssignment(assignment)
	if err != nil {
		fmt.Printf("Error adding mission: %v\n", err)
	} else {
		fmt.Printf("SUCCESS: Mission %d added [Duration: %s, Freq: %s]\n", id, maxDuration, updateFreq)
	}
}
