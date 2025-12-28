package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/Tavis7/bootdev-chirpy/internal/database"

	"github.com/joho/godotenv"
)
import _ "github.com/lib/pq"

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

func main() {
	cfg := &apiConfig{}

	godotenv.Load()
	dbUrl := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		fmt.Println("Erorr: %v", err)
		return
	}
	cfg.dbQueries = database.New(db)

	fmt.Println("Starting server")
	fmt.Printf("DB url: %v\n", dbUrl)
	fmt.Printf("DB queries: %v\n", cfg.dbQueries)

	serveMux := http.NewServeMux()
	serveMux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.getStatsHandler))
	serveMux.Handle("POST /admin/reset", http.HandlerFunc(cfg.resetStatsHandler))
	serveMux.Handle("GET /api/healthz", http.HandlerFunc(healthHandler))
	serveMux.Handle("POST /api/validate_chirp", http.HandlerFunc(chirpValidatorHandler))
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(
		http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))

	server := &http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}

	err = server.ListenAndServe()
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

func chirpValidatorHandler(w http.ResponseWriter, r *http.Request) {
	type chirp struct {
		Body string `json:body`
	}
	type apiSuccess struct {
		Error       string `json:"error,omitempty"`
		Valid       bool   `json:"valid,omitempty"`
		CleanedBody string `json:"cleaned_body,omitempty"`
	}

	response := apiSuccess{}

	body := r.Body
	content, err := io.ReadAll(body)
	if err != nil {
		log.Printf("Error reading request: %v", err)
		response.Error = fmt.Sprintf("Error reading request")
		res, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling json: %v", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(res)
		return
	}

	c := chirp{}
	err = json.Unmarshal(content, &c)
	if err != nil {
		log.Printf("Error decodign json: %v", err)
		response.Error = fmt.Sprintf("Error decoding json")
		res, err := json.Marshal(response)
		if err != nil {
			log.Printf("Error marshaling json: %v", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(500)
		w.Write(res)
		return
	}

	const maxChirpLength = 140
	response.Valid = true
	status := 200
	words := strings.Split(c.Body, " ")
	newWords := []string{}
	badWords := []string{
		"kerfuffle",
		"sharbert",
		"fornax",
	}

	for _, word := range words {
		for _, badWord := range badWords {
			if strings.ToLower(word) == badWord {
				word = "****"
				break
			}
		}
		newWords = append(newWords, word)
	}

	if len(c.Body) > maxChirpLength {
		response.Valid = false
		response.Error = "Chirp is too long"
		status = 400
	}

	if response.Valid {
		response.CleanedBody = strings.Join(newWords, " ")
	}

	res, err := json.Marshal(response)
	if err != nil {
		log.Printf("Error marshaling json: %v", err)
		w.WriteHeader(500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(res)
}
