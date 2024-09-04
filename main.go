package main

import (
	"fmt"

	"github.com/flrn000/chirpy/internal/server"
)

func main() {
	fmt.Println("Serving on port: ", server.Port)
	server.S.ListenAndServe()
}
