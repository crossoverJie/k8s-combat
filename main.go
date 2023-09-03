package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		log.Println("ping")
		fmt.Fprint(w, "pong")
	})

	http.ListenAndServe(":8081", nil)
}
