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
	mux.Handle("/", http.FileServer(http.Dir(root)))

	fmt.Println("Serving on port: ", Port)
	log.Fatal(Srv.ListenAndServe())
}
