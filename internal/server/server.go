package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

type apiConfig struct {
	fileServerHits int
}

type requestBody struct {
	Body *string `json:"body"`
}

type user struct {
	email    string
	password []byte
}

const Port = "8080"
const root = "."
const maxChirpLength = 140

var mux = http.NewServeMux()
var Srv = &http.Server{
	Handler: mux,
	Addr:    ":" + Port,
}
var tempDB = make(map[string]user)

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
	mux.HandleFunc("GET /api/{id}/test/", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		log.Printf("id is: %v", id)
		w.Write([]byte(fmt.Sprintf("id: is %v", id)))
	})
	mux.HandleFunc("POST /api/users", users)
	mux.HandleFunc("POST /api/login", login)

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

func users(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	type response struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}
	var userInfo requestBody

	err := json.NewDecoder(r.Body).Decode(&userInfo)
	if err != nil {
		log.Printf("Error decoding request body: %v", err)
		w.WriteHeader(500)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(userInfo.Password), 15)
	if err != nil {
		log.Printf("Error could not encrypt password: %v", err)
		w.WriteHeader(500)
		return
	}

	if _, exists := tempDB[userInfo.Email]; !exists {
		tempDB[userInfo.Email] = user{email: userInfo.Email, password: hashedPassword}

		resp, err := json.Marshal(response{ID: 1, Email: userInfo.Email})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}

		w.WriteHeader(http.StatusCreated)
		w.Write(resp)
	} else {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(fmt.Sprintf("Error user with email: %v already exists", userInfo.Email)))
	}
}

func login(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	type response struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}
	var userInfo requestBody

	err := json.NewDecoder(r.Body).Decode(&userInfo)
	if err != nil {
		log.Printf("Error decoding request body: %v", err)
		w.WriteHeader(500)
		return
	}

	if dbUser, exists := tempDB[userInfo.Email]; exists {
		err := bcrypt.CompareHashAndPassword(dbUser.password, []byte(userInfo.Password))
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp, err := json.Marshal(response{ID: 1, Email: dbUser.email})
		if err != nil {
			log.Printf("Error marshalling response: %v", err)
			w.WriteHeader(500)
			return
		}

		w.Write(resp)

	} else {
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte(fmt.Sprintf("No user found with the email: %s", userInfo.Email)))
	}
}
