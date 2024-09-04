package server

import (
	"fmt"
	"log"
	"net/http"
)

const Port = "8080"
const root = "."

var mux = http.NewServeMux()
var Srv = &http.Server{
	Handler: mux,
	Addr:    ":" + Port,
}

func Initialize() {
	mux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir(root))))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(fmt.Append([]byte{}, "OK"))
	})

	fmt.Println("Serving on port: ", Port)
	log.Fatal(Srv.ListenAndServe())
}
