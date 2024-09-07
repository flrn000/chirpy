package server

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type apiConfig struct {
	fileServerHits int
	jwtSecret      string
}

type requestBody struct {
	Body *string `json:"body"`
}

type user struct {
	id             int
	email          string
	password       []byte
	refreshToken   string
	tokenExpiresAt time.Time
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

func Initialize(secret string) {
	cfg := apiConfig{fileServerHits: 0, jwtSecret: secret}

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
	mux.HandleFunc("PUT /api/users", cfg.update)
	mux.HandleFunc("POST /api/login", cfg.login)

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
		tempDB[userInfo.Email] = user{id: len(tempDB) + 1, email: userInfo.Email, password: hashedPassword}

		resp, err := json.Marshal(response{ID: tempDB[userInfo.Email].id, Email: userInfo.Email})
		if err != nil {
			log.Printf("Error marshalling JSON: %s", err)
			w.WriteHeader(500)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write(resp)
	} else {
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(fmt.Sprintf("Error user with email: %v already exists", userInfo.Email)))
	}
}

func (cfg *apiConfig) login(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	type response struct {
		ID           int    `json:"id"`
		Email        string `json:"email"`
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
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

		t := jwt.NewWithClaims(
			jwt.SigningMethodHS256,
			jwt.RegisteredClaims{
				Issuer:    "chirpy",
				IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
				Subject:   string(dbUser.email),
			})
		s, err := t.SignedString([]byte(cfg.jwtSecret))
		if err != nil {
			log.Printf("error signing token: %v", err)
			w.WriteHeader(500)
			return
		}

		buf := make([]byte, 32)
		_, err = rand.Read(buf)
		if err != nil {
			log.Printf("error generating random string: %v", err)
			w.WriteHeader(500)
			return
		}
		refreshToken := hex.EncodeToString(buf)

		dbUser.refreshToken = refreshToken
		dbUser.tokenExpiresAt = time.Now().Add((24 * time.Hour) * 60)
		tempDB[userInfo.Email] = dbUser

		resp, err := json.Marshal(response{ID: dbUser.id, Email: dbUser.email, Token: s, RefreshToken: refreshToken})
		if err != nil {
			log.Printf("Error marshalling response: %v", err)
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)

	} else {
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte(fmt.Sprintf("No user found with the email: %s", userInfo.Email)))
	}
}

func (cfg *apiConfig) update(w http.ResponseWriter, r *http.Request) {
	type requestBody struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	type response struct {
		ID    int    `json:"id"`
		Email string `json:"email"`
	}

	authHeader := r.Header.Get("Authorization")
	if len(authHeader) == 0 {
		log.Println("No authorization header provided")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	token := strings.Fields(authHeader)[1]
	fmt.Println("token is: \n", token)
	parsedToken, err := jwt.ParseWithClaims(token, &jwt.RegisteredClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(cfg.jwtSecret), nil
	})
	if err != nil {
		log.Printf("Error parsing token: %v", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	userEmail, err := parsedToken.Claims.GetSubject()
	if err != nil {
		log.Printf("Error getting claims subject: %v", err)
		w.WriteHeader(500)
		return
	}

	var reqBody requestBody
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Printf("Error parsing request body: %v", err)
		w.WriteHeader(500)
		return
	}

	dbUser, exists := tempDB[userEmail]
	if exists {
		// in a normal app we would update the fields in the db -- I'll just send it back as is for now
		resp, err := json.Marshal(response{ID: dbUser.id, Email: dbUser.email})
		if err != nil {
			log.Printf("Error marshaling response: %v", err)
		}

		w.Write(resp)
	}
}
