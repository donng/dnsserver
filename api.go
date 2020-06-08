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
	http.HandleFunc("/remove", func(writer http.ResponseWriter, request *http.Request) {
		request.ParseForm()
		if domain, ok := request.Form["domain"]; ok {
			store.Delete(domain[0])
			fmt.Fprintf(writer, "domain %s delete success!", domain[0])
			return
		}
		fmt.Fprintln(writer, "please check domain params!")
	})
	go http.ListenAndServe(":8089", nil)

	log.Printf("api service start, port: %d", 8089)
}