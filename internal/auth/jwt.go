package auth

import (
	"time"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
	now := time.Now();
	expires := now.Add(expiresIn)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256,
	jwt.RegisteredClaims{
		Issuer: "chirpy",
		IssuedAt: &jwt.NumericDate{now},
		ExpiresAt: &jwt.NumericDate{expires},
		Subject: userID.String(),
	})
	key := []byte(tokenSecret)
	ss, err := token.SignedString(key)
	if err != nil {
		return "", err
	}
	return ss, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	keyFunc := func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
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
