package mothership

import (
	"log"
	"net/http"
	"time"
)

func StartHTTPServer(m *Mothership) {
	mux := http.NewServeMux()
	RegisterHTTPRoutes(mux, m)

	srv := &http.Server{
		Addr:         ":8080",
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Println("Servidor HTTP na porta 8080")
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("Erro ao iniciar HTTP server: %v", err)
	}
}
