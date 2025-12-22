package main

import (
	"fmt"
	"net/http"
)


func main() {
	fmt.Println("Starting server")

	serveMux := http.NewServeMux()

	server := http.Server {
		Handler: serveMux,
		Addr: ":8080",
	}

	err := server.ListenAndServe()
	if err != nil {
		fmt.Println("Error: %v", err)
		return
	}
}
