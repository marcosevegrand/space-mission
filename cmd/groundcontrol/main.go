package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

var (
	defaultPort        = ":3000"
	defaultAPIURLInput = "10.0.0.21:8003"
	defaultDistPath    = "/dist"
)

func main() {
	port := flag.String("port", defaultPort, "Port to serve the website on")
	apiURLInput := flag.String("api-addr", defaultAPIURLInput, "Mothership Observation API URL")
	distPath := flag.String("dist", defaultDistPath, "Path to the static files")
	flag.Parse()

	// If the user forgot "http://", add it automatically
	finalAPIURL := *apiURLInput
	if !strings.HasPrefix(finalAPIURL, "http://") && !strings.HasPrefix(finalAPIURL, "https://") {
		finalAPIURL = "http://" + finalAPIURL
	}

	// Handle the dynamic config file
	http.HandleFunc("/config.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		fmt.Fprintf(w, "window.ENV = { API_URL: '%s' };", finalAPIURL)
	})

	// Serve static files
	fs := http.FileServer(http.Dir(*distPath))
	http.Handle("/", fs)

	fmt.Println("=======================================================")
	fmt.Printf("       GROUND CONTROL STARTED (%s)\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Println("=======================================================")
	log.Printf("Ground Control running at http://localhost%s", *port)
	log.Printf("Connected to Mothership API at %s", finalAPIURL)
	fmt.Println("-------------------------------------------------------")

	err := http.ListenAndServe(*port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
