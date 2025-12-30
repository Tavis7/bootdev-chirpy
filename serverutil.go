package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type chirpyServerErrorResponse struct {
	Reason string `json:"error"`
}

// The reason will be sent to the client but err will only be logged
// fmt.Errorf() can be used to log additional information not intended for the response
func chirpySendErrorResponse(w http.ResponseWriter,
	status int, reason string, err error) {
	if err != nil {
		log.Printf("Error: %v: %v", reason, err)
	}

	data := chirpyServerErrorResponse{}
	data.Reason = reason

	res, err := chirpyEncodeJsonResponse(status, data)
	if err != nil {
		log.Printf("Error: %v", err)
	}

	chirpySendResponse(w, res)
	return
}

type chirpyEncodedJsonResponse struct {
	data       []byte // Should always contain valid json
	statusCode int    // Needed for returning error status codes
}

// Always returns a sendable response
// statusCode and data are set appropriately to indicate errors
func chirpyEncodeJsonResponse(statusCode int, t any) (chirpyEncodedJsonResponse, error) {
	res := chirpyEncodedJsonResponse{
		data:       []byte(`{"error":"unexpected failure"}`),
		statusCode: statusCode,
	}

	r, err := json.Marshal(t)
	if err != nil {
		res.data = []byte(`{"error":"json encoding failure"}`)
		res.statusCode = 500
		return res, fmt.Errorf("Failed to marshal json: %w", err)
	}

	res.data = r
	return res, nil
}

func chirpySendResponse(w http.ResponseWriter, res chirpyEncodedJsonResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.statusCode)
	w.Write(res.data)
}

// todo: return a sendable error response instead of error
func chirpyDecodeJsonRequest(r *http.Request, t any) error {
	content, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(content, t)
	if err != nil {
		return fmt.Errorf("Failed to decode json: %w", err)
	}
	return nil
}
