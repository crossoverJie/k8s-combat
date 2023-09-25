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
		url := os.Getenv("PG_URL")
		pwd := os.Getenv("PG_PWD")
		fmt.Fprint(w, fmt.Sprintf("%s-%s-%s", name, url, pwd))
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
