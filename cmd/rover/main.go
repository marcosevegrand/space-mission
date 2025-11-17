package main

import (
	"log"
	"space-mission/internal/rover"
	"time"
)

func main() {
	r, err := rover.NewRover(
		1,
		1*time.Second,
		5*time.Second,
		"localhost:8001",
		"localhost:9002",
	)
	if err != nil {
		log.Fatal(err)
	}

	if err := r.Start(3*time.Second, "localhost:9001"); err != nil {
		log.Fatal(err)
	}

	c := make(chan struct{})

	<-c
	log.Println("Mothership stopped")
}
