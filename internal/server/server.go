package server

import "net/http"

const Port = "8080"

var mux = http.NewServeMux()
var S = http.Server{Handler: mux, Addr: ":" + Port}
