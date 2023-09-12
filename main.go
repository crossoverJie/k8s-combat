package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name, _ := os.Hostname()
		fmt.Fprint(w, name)
	})
	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		name, _ := os.Hostname()
		log.Printf("%s ping", name)
		fmt.Fprint(w, "pong")
	})
	http.HandleFunc("/service", func(w http.ResponseWriter, r *http.Request) {
		resp, err := http.Get("http://k8s-combat-service:8081/ping")
		if err != nil {
			log.Println(err)
			fmt.Fprint(w, err)
			return
		}
		fmt.Fprint(w, resp.Status)
	})

	http.ListenAndServe(":8081", nil)
}
