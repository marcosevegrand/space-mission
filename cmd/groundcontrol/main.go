package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings" // <--- Import this
)

func main() {
	port := flag.String("port", ":3000", "Port to serve the website on")
	apiURLInput := flag.String("api", "http://localhost:8080", "Mothership API URL")
	flag.Parse()

	// If the user forgot "http://", add it automatically.
	finalAPIURL := *apiURLInput
	if !strings.HasPrefix(finalAPIURL, "http://") && !strings.HasPrefix(finalAPIURL, "https://") {
		finalAPIURL = "http://" + finalAPIURL
	}

	// Handle the dynamic config file
	http.HandleFunc("/config.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		// Use finalAPIURL here
		fmt.Fprintf(w, "window.ENV = { API_URL: '%s' };", finalAPIURL)
	})

	// Serve static files
	fs := http.FileServer(http.Dir("./groundcontrol-ui/dist"))
	http.Handle("/", fs)

	log.Printf("Ground Control running at http://localhost%s", *port)
	log.Printf("Connected to Mothership at %s", finalAPIURL)

	err := http.ListenAndServe(*port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
