package main

import (
	"encoding/json"
	"errors"
	"context"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Credentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	pool, err := pgxpool.New(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := initDB(context.Background(), pool); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/signup", signupHandler(pool))
	mux.HandleFunc("/login", loginHandler(pool))

	handler := corsMiddleware(mux)
	log.Printf("API listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

func initDB(ctx context.Context, pool *pgxpool.Pool) error {
	const schema = `
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);
`

	_, err := pool.Exec(ctx, schema)
	return err
}

// signupHandler 는 신규 회원을 DB 에 저장한다. 이미 있는 이메일이면 409.
func signupHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, ok := decodeCredentials(w, r)
		if !ok {
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "failed to process password", http.StatusInternalServerError)
			return
		}

		// 이미 가입된 이메일이면 아무 행도 삽입되지 않는다.
		const insertStmt = `
INSERT INTO users (email, password_hash)
VALUES ($1, $2)
ON CONFLICT (email) DO NOTHING
`
		tag, err := pool.Exec(r.Context(), insertStmt, req.Email, string(hash))
		if err != nil {
			http.Error(w, "failed to save user", http.StatusInternalServerError)
			return
		}

		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusConflict, `{"status":"exists","message":"이미 가입된 이메일입니다"}`)
			return
		}

		log.Printf("signup ok email=%s", req.Email)
		writeJSON(w, http.StatusOK, `{"status":"ok"}`)
	}
}

// loginHandler 는 DB 의 저장된 해시와 대조해 일치하면 ok, 아니면 fail.
func loginHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, ok := decodeCredentials(w, r)
		if !ok {
			return
		}

		var hash string
		err := pool.QueryRow(r.Context(),
			`SELECT password_hash FROM users WHERE email = $1`, req.Email).Scan(&hash)
		if errors.Is(err, pgx.ErrNoRows) {
			// 가입되지 않은 이메일 → 로그인 실패
			log.Printf("login fail (no user) email=%s", req.Email)
			writeJSON(w, http.StatusUnauthorized, `{"status":"fail"}`)
			return
		}
		if err != nil {
			http.Error(w, "failed to query user", http.StatusInternalServerError)
			return
		}

		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.Password)) != nil {
			// 비밀번호 불일치 → 로그인 실패
			log.Printf("login fail (bad password) email=%s", req.Email)
			writeJSON(w, http.StatusUnauthorized, `{"status":"fail"}`)
			return
		}

		log.Printf("login ok email=%s", req.Email)
		writeJSON(w, http.StatusOK, `{"status":"ok"}`)
	}
}

// decodeCredentials 는 요청 본문을 파싱/검증한다. 실패 시 응답을 쓰고 false 반환.
func decodeCredentials(w http.ResponseWriter, r *http.Request) (Credentials, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return Credentials{}, false
	}

	var req Credentials
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return Credentials{}, false
	}

	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		http.Error(w, "email and password are required", http.StatusBadRequest)
		return Credentials{}, false
	}

	return req, true
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
