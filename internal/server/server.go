package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type apiConfig struct {
	fileServerHits int
}

type requestBody struct {
	Body *string `json:"body"`
}

const Port = "8080"
const root = "."
const maxChirpLength = 140

var mux = http.NewServeMux()
var Srv = &http.Server{
	Handler: mux,
	Addr:    ":" + Port,
}

func Initialize() {
	cfg := apiConfig{fileServerHits: 0}
	mux.Handle("/app/", cfg.middlewareMetrics(http.StripPrefix("/app/", http.FileServer(http.Dir(root)))))
	mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(http.StatusText(http.StatusOK)))
	})
	mux.HandleFunc("GET /admin/metrics", cfg.handleMetrics)
	mux.HandleFunc("GET /api/reset", cfg.resetMetrics)
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	fmt.Println("Serving on port: ", Port)
	log.Fatal(Srv.ListenAndServe())
}

func (cfg *apiConfig) middlewareMetrics(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits++
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(fmt.Sprintf(`
		<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!
		</body
		</html>
	`,
		cfg.fileServerHits)))
}

func (cfg *apiConfig) resetMetrics(w http.ResponseWriter, r *http.Request) {
	cfg.fileServerHits = 0
	w.Write([]byte("Reset hits to 0"))
}

func validateChirp(w http.ResponseWriter, r *http.Request) {
	var data requestBody
	type errResponse struct {
		Error string `json:"error"`
	}
	type validResponse struct {
		Valid bool `json:"valid"`
	}

	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		log.Printf("Error decoding request body: %s", err)
		w.WriteHeader(500)
		return
	}

	if len(*data.Body) > maxChirpLength {
		respBody, err := json.Marshal(errResponse{Error: "Chirp is too long"})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(http.StatusBadRequest)
		w.Write(respBody)
	} else {
		resp, err := json.Marshal(validResponse{Valid: true})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
	}
}
