package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/lib/pq"

	"github.com/Tavis7/bootdev-chirpy/internal/database"
)

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
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(
		http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))

	serveMux.Handle("GET /api/healthz", http.HandlerFunc(healthHandler))
	serveMux.Handle("POST /api/users", http.HandlerFunc(cfg.userCreateHandler))
	serveMux.Handle("POST /api/chirps", http.HandlerFunc(cfg.chirpCreateHandler))
	serveMux.Handle("GET /api/chirps", http.HandlerFunc(cfg.chirpGetHandler))

	serveMux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.getStatsHandler))
	serveMux.Handle("POST /admin/reset", http.HandlerFunc(cfg.resetHandler))

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

	_, err = cfg.dbQueries.ResetChirps(r.Context())
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to delete users", err)
		return
	}

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

type chirp struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Body      string `json:"body"`
	UserID    string `json:"user_id"`
}

func (cfg *apiConfig) chirpCreateHandler(w http.ResponseWriter, r *http.Request) {
	c := chirp{}

	err := chirpyDecodeJsonRequest(r, &c)
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to read request", err)
		return
	}

	const maxChirpLength = 140
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
		chirpySendErrorResponse(w, 400, "Chirp is too long", nil)
		return
	}

	cleanedBody := strings.Join(newWords, " ")

	userID, err := uuid.Parse(c.UserID)
	if err != nil {
		chirpySendErrorResponse(w, 400, "Invalid UUID", err)
		return
	}

	dbStatus, err := cfg.dbQueries.CreateChirp(r.Context(),
		database.CreateChirpParams{
			cleanedBody,
			userID,
		})

	response := chirp{
		ID:        dbStatus.ID.String(),
		CreatedAt: dbStatus.CreatedAt.String(),
		UpdatedAt: dbStatus.UpdatedAt.String(),
		Body:      dbStatus.Body,
		UserID:    dbStatus.UserID.String(),
	}

	res, err := chirpyEncodeJsonResponse(201, response)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}

func (cfg *apiConfig) chirpGetHandler(w http.ResponseWriter, r *http.Request) {
	dbStatus, err := cfg.dbQueries.GetAllChirps(r.Context())
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to get chirps", err)
		return
	}

	response := []chirp{}

	for _, c := range dbStatus {
		response = append(response, chirp{
			ID:        c.ID.String(),
			CreatedAt: c.CreatedAt.String(),
			UpdatedAt: c.UpdatedAt.String(),
			Body:      c.Body,
			UserID:    c.UserID.String(),
		})
	}

	res, err := chirpyEncodeJsonResponse(200, response)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}
