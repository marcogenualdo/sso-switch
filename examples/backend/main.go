package main

import (
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Incoming request: %s %s", r.Method, r.URL.Path)

	for name, values := range r.Header {
		for _, value := range values {
			log.Printf("Header: %s=%s", name, value)
		}
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func main() {
	http.HandleFunc("/", handler)

	addr := ":8080"
	log.Printf("Listening on %s", addr)

	log.Fatal(http.ListenAndServe(addr, nil))
}
