package auth

import (
	"github.com/alexedwards/argon2id"
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password,
		&argon2id.Params{
			Memory:      800000,
			Iterations:  1,
			Parallelism: 1,
			SaltLength:  16,
			KeyLength:   16,
		})
	if err != nil {
		return "", err
	}
	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	matches, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return false, err
	}
	return matches, nil
}
