package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/lib/pq"

	"github.com/Tavis7/bootdev-chirpy/internal/auth"
	"github.com/Tavis7/bootdev-chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
	isDevPlatform  bool
	jwtSecret      string
	jwtDuration time.Duration
}

func main() {
	cfg := &apiConfig{}

	godotenv.Load()
	platform := os.Getenv("PLATFORM")
	if platform == "dev" {
		fmt.Println("Warning: running as dev environment")
		cfg.isDevPlatform = true
	}

	dbUrl := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbUrl)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	cfg.dbQueries = database.New(db)

	cfg.jwtSecret = os.Getenv("JWT_SECRET")
	cfg.jwtDuration = time.Hour * 1

	fmt.Println("Starting server")
	fmt.Printf("DB url: %v\n", dbUrl)
	fmt.Printf("DB queries: %v\n", cfg.dbQueries)

	serveMux := http.NewServeMux()
	serveMux.Handle("/app/", cfg.middlewareMetricsInc(
		http.StripPrefix("/app/", http.FileServer(http.Dir(".")))))

	serveMux.Handle("GET /api/healthz", http.HandlerFunc(healthHandler))

	serveMux.Handle("POST /api/users", http.HandlerFunc(cfg.userCreateHandler))
	serveMux.Handle("PUT /api/users", http.HandlerFunc(cfg.userModifyHandler))
	serveMux.Handle("POST /api/login", http.HandlerFunc(cfg.userLoginHandler))
	serveMux.Handle("POST /api/refresh", http.HandlerFunc(cfg.userAuthRefreshHandler))
	serveMux.Handle("POST /api/revoke", http.HandlerFunc(cfg.userAuthRevokeHandler))

	serveMux.Handle("GET /api/chirps", http.HandlerFunc(cfg.chirpsGetHandler))
	serveMux.Handle("POST /api/chirps", http.HandlerFunc(cfg.chirpCreateHandler))
	serveMux.Handle("GET /api/chirps/{id}", http.HandlerFunc(cfg.chirpGetHandler))
	serveMux.Handle("DELETE /api/chirps/{id}", http.HandlerFunc(cfg.chirpDeleteHandler))

	serveMux.Handle("POST /api/polka/webhooks", http.HandlerFunc(cfg.upgradeUserToChirpyRedHandler))

	serveMux.Handle("GET /admin/metrics", http.HandlerFunc(cfg.getStatsHandler))
	serveMux.Handle("POST /admin/reset", http.HandlerFunc(cfg.resetHandler))

	server := &http.Server{
		Handler: serveMux,
		Addr:    ":8080",
	}

	err = server.ListenAndServe()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
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
		chirpySendErrorResponse(w, 403, "Not a dev environment", nil)
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

type userAuthInfo struct {
	Email            string `"json:email"`
	Password         string `"json:password"`
}

type chirpyUserInfo struct {
	Id        string `json:"id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Email     string `json:"email"`
	Token     string `json:"token,omitempty"`
	RefreshToken string `json:"refresh_token"`
	IsChirpyRed bool `json:"is_chirpy_red"`
}

func (cfg *apiConfig) userCreateHandler(w http.ResponseWriter, r *http.Request) {
	req := userAuthInfo{}

	err := chirpyDecodeJsonRequest(r, &req)
	if err != nil {
		chirpySendErrorResponse(w, 400, "Invalid request", err)
		return
	}

	if len(req.Password) == 0 {
		chirpySendErrorResponse(w, 400, "Password required", err)
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to create user", err)
		return
	}

	dbUserRow, err := cfg.dbQueries.CreateUser(r.Context(),
		database.CreateUserParams{
			req.Email,
			passwordHash,
		})
	if err != nil {
		e, ok := err.(*pq.Error)
		if ok &&
			e.Code.Name() == "unique_violation" &&
			e.Constraint == "users_email_key" {

			chirpySendErrorResponse(w, 400, "User already exists", e)
			return
		}
		chirpySendErrorResponse(w, 500, "Error creating user", e)
		return
	}

	createdUser := chirpyUserInfo{
		Id:        dbUserRow.ID.String(),
		CreatedAt: dbUserRow.CreatedAt.String(),
		UpdatedAt: dbUserRow.UpdatedAt.String(),
		Email:     dbUserRow.Email,
		IsChirpyRed: dbUserRow.IsChirpyRed,
	}

	res, err := chirpyEncodeJsonResponse(201, createdUser)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}

func (cfg *apiConfig) userModifyHandler(w http.ResponseWriter, r *http.Request) {
	req := userAuthInfo{}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Authorization failed", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Authorization failed", err)
		return
	}

	err = chirpyDecodeJsonRequest(r, &req)
	if err != nil {
		chirpySendErrorResponse(w, 400, "Invalid request", err)
		return
	}

	if len(req.Password) == 0 {
		chirpySendErrorResponse(w, 400, "Password required", err)
		return
	}

	if len(req.Email) == 0 {
		chirpySendErrorResponse(w, 400, "Email required", err)
		return
	}

	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to update user", err)
		return
	}

	dbUserRow, err := cfg.dbQueries.UpdateUserEmailAndPassword(r.Context(),
		database.UpdateUserEmailAndPasswordParams{
			userID,
			req.Email,
			passwordHash,
		})
	if err != nil {
		e, ok := err.(*pq.Error)
		if ok &&
			e.Code.Name() == "unique_violation" &&
			e.Constraint == "users_email_key" {

			chirpySendErrorResponse(w, 400, "User already exists", e)
			return
		}
		chirpySendErrorResponse(w, 500, "Error creating user", e)
		return
	}

	updatedUser := chirpyUserInfo{
		Id:        dbUserRow.ID.String(),
		CreatedAt: dbUserRow.CreatedAt.String(),
		UpdatedAt: dbUserRow.UpdatedAt.String(),
		Email:     dbUserRow.Email,
		IsChirpyRed: dbUserRow.IsChirpyRed,
	}

	res, err := chirpyEncodeJsonResponse(200, updatedUser)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}

func (cfg *apiConfig) userLoginHandler(w http.ResponseWriter, r *http.Request) {
	req := userAuthInfo{}

	err := chirpyDecodeJsonRequest(r, &req)
	if err != nil {
		chirpySendErrorResponse(w, 400, "Invalid request", err)
		return
	}

	dbUserRow, err := cfg.dbQueries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		// todo Some errors should 500
		chirpySendErrorResponse(w, 401, "Incorrect email or password", err)
		return
	}

	matches, err := auth.CheckPasswordHash(req.Password, dbUserRow.HashedPassword)
	if err != nil {
		chirpySendErrorResponse(w, 500, "Authentication failed", err)
		return
	}

	if !matches {
		chirpySendErrorResponse(w, 401, "Incorrect email or password", err)
		return
	}

	token, err := auth.MakeJWT(dbUserRow.ID, cfg.jwtSecret, cfg.jwtDuration)
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to generate auth token", err)
	}

	refresh_token, err := auth.MakeRefreshToken()
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to generate refresh token", err)
	}

	_, err = cfg.dbQueries.StoreRefreshToken(r.Context(),
	database.StoreRefreshTokenParams{
		refresh_token,
		dbUserRow.ID,
		dbUserRow.CreatedAt.Add(time.Hour * 24 * 60),
	})
	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to store refresh token", err)
	}

	createdUser := chirpyUserInfo{
		Id:        dbUserRow.ID.String(),
		CreatedAt: dbUserRow.CreatedAt.String(),
		UpdatedAt: dbUserRow.UpdatedAt.String(),
		Email:     dbUserRow.Email,
		Token:     token,
		RefreshToken: refresh_token,
		IsChirpyRed: dbUserRow.IsChirpyRed,
	}

	res, err := chirpyEncodeJsonResponse(200, createdUser)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}


func (cfg *apiConfig) userAuthRefreshHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetRefreshToken(r.Header)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Missing authorization header", err)
		return
	}

	dbTokenRow, err := cfg.dbQueries.GetRefreshToken(r.Context(), token)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Token refresh failed", err)
		return
	}

	now := time.Now()

	if dbTokenRow.RevokedAt.Valid || now.After(dbTokenRow.ExpiresAt) {
		chirpySendErrorResponse(w, 401, "Token refresh failed", err)
		return
	}

	jwt, err := auth.MakeJWT(dbTokenRow.UserID, cfg.jwtSecret, cfg.jwtDuration)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Token refresh failed", err)
		return
	}

	type chirpyJWT struct {
		Token string `json:"token"`
	}
	response := chirpyJWT{
		Token: jwt,
	}

	res, err := chirpyEncodeJsonResponse(200, response)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}

func (cfg *apiConfig) userAuthRevokeHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetRefreshToken(r.Header)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Missing authorization header", err)
		return
	}

	dbTokenRow, err := cfg.dbQueries.RevokeRefreshToken(r.Context(), token)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Failed to revoke token", err)
		return
	}

	if !dbTokenRow.RevokedAt.Valid {
		chirpySendErrorResponse(w, 500, "Failed to revoke token", err)
		return
	}

	w.WriteHeader(204)
	w.Write([]byte{})
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

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Authorization failed", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Authorization failed", err)
		return
	}

	err = chirpyDecodeJsonRequest(r, &c)
	if err != nil {
		chirpySendErrorResponse(w, 400, "Invalid request", err)
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

func (cfg *apiConfig) chirpsGetHandler(w http.ResponseWriter, r *http.Request) {
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

func (cfg *apiConfig) chirpDeleteHandler(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Authorization failed", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		chirpySendErrorResponse(w, 401, "Authorization failed", err)
		return
	}

	id := r.PathValue("id")
	fmt.Printf("Path: %v\n", id)

	chirpID, err := uuid.Parse(id)
	if err != nil {
		chirpySendErrorResponse(w, 404, "Chirp not found", err)
		return
	}

	dbChirpRow, err := cfg.dbQueries.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		chirpySendErrorResponse(w, 404, "Chirp not found", err)
		return
	}

	if dbChirpRow.UserID != userID {
		chirpySendErrorResponse(w, 403, "Unauthorized", err)
		return
	}

	_, err = cfg.dbQueries.DeleteChirpByID(r.Context(),
	database.DeleteChirpByIDParams{
		chirpID,
		userID,
	})

	if err != nil {
		chirpySendErrorResponse(w, 500, "Failed to delete chirp", err)
		return
	}

	w.WriteHeader(204)
	w.Write([]byte{})
}

func (cfg *apiConfig) chirpGetHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	fmt.Printf("Path: %v\n", id)

	chirpID, err := uuid.Parse(id)
	if err != nil {
		chirpySendErrorResponse(w, 404, "Chirp not found", err)
		return
	}

	dbStatus, err := cfg.dbQueries.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		chirpySendErrorResponse(w, 404, "Chirp not found", err)
		return
	}

	response := chirp{
		ID:        dbStatus.ID.String(),
		CreatedAt: dbStatus.CreatedAt.String(),
		UpdatedAt: dbStatus.UpdatedAt.String(),
		Body:      dbStatus.Body,
		UserID:    dbStatus.UserID.String(),
	}

	res, err := chirpyEncodeJsonResponse(200, response)
	if err != nil {
		log.Printf("Error: %v", err)
		// continue
	}

	chirpySendResponse(w, res)
}

func (cfg *apiConfig) upgradeUserToChirpyRedHandler(w http.ResponseWriter, r *http.Request) {
	type chirpyRedWebhook struct{
		Event string `json:"event"`
		Data struct{
			UserID string `json:"user_id"`
		} `json:"data"`
	}

	req := chirpyRedWebhook{}

	err := chirpyDecodeJsonRequest(r, &req)
	if err != nil {
		chirpySendErrorResponse(w, 400, "Invalid request", err)
		return
	}

	if req.Event != "user.upgraded" {
		w.WriteHeader(204)
		w.Write([]byte{})
		return
	}

	userID, err := uuid.Parse(req.Data.UserID)
	if err != nil {
		chirpySendErrorResponse(w, 400, "Invalid request", err)
		return
	}

	if req.Event == "user.upgraded" {
		dbUserRow, err := cfg.dbQueries.UpgradeToChirpyRed(r.Context(), userID)
		if err != nil {
			chirpySendErrorResponse(w, 404, "Not found", err)
			return
		}

		if !dbUserRow.IsChirpyRed {
			chirpySendErrorResponse(w, 500, "Failed to update user", err)
			return
		}

		w.WriteHeader(204)
		w.Write([]byte{})
		return
	}

	chirpySendErrorResponse(w, 404, "Unknown event", nil)
}
