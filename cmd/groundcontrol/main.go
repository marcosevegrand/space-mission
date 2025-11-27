package main

import (
	"log"
	"net/http"
)

func main() {
	// The port we want the website to run on
	port := ":3000"

	// 1. Create a file server handler
	// This tells Go to look into the "groundcontrol-ui/dist" folder for files
	fs := http.FileServer(http.Dir("./groundcontrol-ui/dist"))

	// 2. Strip the prefix so accessing "/" serves index.html
	http.Handle("/", fs)

	log.Printf("Ground Control Website running at http://localhost%s", port)

	// 3. Start the server
	err := http.ListenAndServe(port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
