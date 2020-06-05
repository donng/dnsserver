package main

import (
	"fmt"
	"log"
	"net/http"
)

func startApi() {
	// flush cache
	http.HandleFunc("/flush", func(writer http.ResponseWriter, request *http.Request) {
		store.Flush()
		fmt.Fprintln(writer, "cache flushed!")
	})
	go http.ListenAndServe(":8089", nil)

	log.Printf("api service start, port: %d", 8089)
}