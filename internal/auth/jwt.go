package auth

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now()
	expires := now.Add(expiresIn)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
		jwt.RegisteredClaims{
			Issuer:    "chirpy",
			IssuedAt:  &jwt.NumericDate{now},
			ExpiresAt: &jwt.NumericDate{expires},
			Subject:   userID.String(),
		})

	key, err := base64.StdEncoding.DecodeString(tokenSecret)
	if err != nil {
		return "", err
	}

	ss, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return ss, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	keyFunc := func(token *jwt.Token) (any, error) {
		return base64.StdEncoding.DecodeString(tokenSecret)
	}

	claims := jwt.RegisteredClaims{}
	t, err := jwt.ParseWithClaims(tokenString, &claims, keyFunc)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("Error parsing token with claims: %w", err)
	}

	subject, err := t.Claims.GetSubject()
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("Error getting subject: %w", err)
	}

	id, err := uuid.Parse(subject)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("Error parsing subject: %w", err)
	}

	return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {
	header := headers.Get("Authorization")
	if header == "" {
		return "", fmt.Errorf("Missing authorization header")
	}

	prefix := "Bearer "
	token, ok := strings.CutPrefix(header, prefix)
	if ok != true {
		return "", fmt.Errorf("Authorization header missing '%v' prefix: %v",
			prefix, token)
	}

	token = strings.TrimSpace(token)

	fmt.Printf("Header: %v\n", header)
	return token, nil
}
