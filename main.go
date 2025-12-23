package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func main() {
	fmt.Println("Starting server")

	cfg := &apiConfig{}
	serveMux := http.NewServeMux()
	serveMux.Handle("GET /api/healthz", http.HandlerFunc(healthHandler))
	serveMux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.getStatsHandler))
	serveMux.Handle("POST /admin/reset", http.HandlerFunc(cfg.resetStatsHandler))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(
		http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))

	server := &http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Error: %v", err)
		return
	}
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) getStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf(
		""+
			"<html>\n"+
			"    <body>\n"+
			"        <h1>Welcome, Chirpy Admin</h1>\n"+
			"        <p>Chirpy has been visited %v times!</p>\n"+
			"    </body>\n"+
			"</html>\n",
		cfg.fileserverHits.Load())))
}

func (cfg *apiConfig) resetStatsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	cfg.fileserverHits.Store(0)
	w.Write([]byte{})
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}
