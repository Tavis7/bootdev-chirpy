package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/Tavis7/bootdev-chirpy/internal/database"

	"github.com/joho/godotenv"
)
import "github.com/lib/pq"

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	isDevPlatform  bool
}

func main() {
	cfg := &apiConfig{}

	godotenv.Load()
	dbUrl := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	if platform == "dev" {
		fmt.Println("Warning: running as dev environment")
		cfg.isDevPlatform = true
	}
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		fmt.Println("Error: %v", err)
		return
	}
	cfg.dbQueries = database.New(db)

	fmt.Println("Starting server")
	fmt.Printf("DB url: %v\n", dbUrl)
	fmt.Printf("DB queries: %v\n", cfg.dbQueries)

	serveMux := http.NewServeMux()
	serveMux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.getStatsHandler))
	serveMux.Handle("POST /admin/reset", http.HandlerFunc(cfg.resetHandler))
	serveMux.Handle("GET /api/healthz", http.HandlerFunc(healthHandler))
	serveMux.Handle("POST /api/validate_chirp", http.HandlerFunc(chirpValidatorHandler))
	serveMux.Handle("POST /api/users", http.HandlerFunc(cfg.userCreateHandler))
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

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if !cfg.isDevPlatform {
		chirpySendErrorResponse(w, 500, "Not a dev environment", nil)
		return
	}

	_, err := cfg.dbQueries.ResetUsers(r.Context())
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to delete users", err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(200)
	cfg.fileserverHits.Store(0)
	w.Write([]byte{})
}

func (cfg *apiConfig) userCreateHandler(w http.ResponseWriter, r *http.Request) {
	type user struct {
		Email string `json:email`
	}

	type response struct {
		Id        string `json:"id"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Email     string `json:"email"`
	}

	createdUser := response{}
	req := user{}

	err := chirpyDecodeJsonRequest(r, &req)
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to read request", err)
		return
	}

	dbStatus, err := cfg.dbQueries.CreateUser(r.Context(), req.Email)
	if err != nil {
		e, ok := err.(*pq.Error)
		if ok &&
			e.Code.Name() == "unique_violation" &&
			e.Constraint == "users_email_key" {

			chirpySendErrorResponse(w, 500, "User already exists", e)
			return
		}
		chirpySendErrorResponse(w, 500, "Error creating user", e)
		return
	}

	createdUser.Id = dbStatus.ID.String()
	createdUser.CreatedAt = dbStatus.CreatedAt.String()
	createdUser.UpdatedAt = dbStatus.UpdatedAt.String()
	createdUser.Email = dbStatus.Email

	res, err := chirpyEncodeJsonResponse(201, createdUser)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
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
	c := chirp{}

	err := chirpyDecodeJsonRequest(r, &c)
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to read request", err)
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

	res, err := chirpyEncodeJsonResponse(status, response)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}
