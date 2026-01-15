package auth

import (
	"net/http"
	"fmt"
	"strings"
)

func GetAPIKey(headers http.Header) (string, error){
	header := headers.Get("Authorization")
	if header == "" {
		return "", fmt.Errorf("Missing authorization header")
	}

	prefix := "ApiKey "
	value, ok := strings.CutPrefix(header, prefix)
	if ok != true {
		return "", fmt.Errorf("Authorization header missing '%v' prefix: %v",
			prefix, value)
	}

	key := strings.TrimSpace(value)

	return key, nil
}
