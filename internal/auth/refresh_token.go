package auth

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"fmt"
	"strings"
)

func MakeRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	str := hex.EncodeToString(bytes)

	return str, nil
}

func GetRefreshToken(headers http.Header) (string, error) {
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
