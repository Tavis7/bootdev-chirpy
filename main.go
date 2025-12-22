package main

import (
	"fmt"
	"net/http"
)

func main() {
	fmt.Println("Starting server")

	serveMux := http.NewServeMux()
	serveMux.Handle("/healthz", http.HandlerFunc(healthHandler))
	serveMux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir("."))))

	server := &http.Server {
		Handler: serveMux,
		Addr: ":8080",
	}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Error: %v", err)
		return
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}
