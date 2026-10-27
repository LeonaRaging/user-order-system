package main

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Dashboard struct {
	User   json.RawMessage `json:"user"`
	Orders json.RawMessage `json:"orders"`
}

func fetchJSON(ctx context.Context, client *http.Client, target string) (json.RawMessage, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(resp.Status)
	}

	var result json.RawMessage
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return result, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("invalid PEM public key")
	}

	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	publicKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("key is not RSA")
	}

	return publicKey, nil
}

func authMiddleware(publicKey *rsa.PublicKey, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			http.Error(w, "missing bearer token", http.StatusUnauthorized)
			return
		}

		tokenString := strings.TrimPrefix(header, "Bearer ")

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
			if token.Method != jwt.SigningMethodRS256 {
				return nil, errors.New("unexpected signing method")
			}
			return publicKey, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func proxyHandler(target string) (http.Handler, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(targetURL), nil
}

func dashboardHandler(client *http.Client) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("userId")
		if userID == "" {
			http.Error(w, "missing userId", http.StatusBadRequest)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		encodedID := url.QueryEscape(userID)

		var (
			user     json.RawMessage
			orders   json.RawMessage
			userErr  error
			orderErr error
		)

		var group sync.WaitGroup
		group.Add(2)

		go func() {
			defer group.Done()
			user, userErr = fetchJSON(
				ctx,
				client,
				"http://user-service:8081/users/"+url.PathEscape(userID),
			)
		}()

		go func() {
			defer group.Done()
			orders, orderErr = fetchJSON(
				ctx,
				client,
				"http://order-service:8082/orders?userId="+encodedID,
			)
		}()

		group.Wait()

		if userErr != nil || orderErr != nil {
			http.Error(w, "failed to build dashboard", http.StatusBadGateway)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(Dashboard{
			User: user, Orders: orders,
		})
	})
}

func main() {
	publicKeyPath := os.Getenv("JWT_PUBLIC_KEY_PATH")
	if publicKeyPath == "" {
		publicKeyPath = "/keys/public.pem"
	}

	publicKey, err := loadPublicKey(publicKeyPath)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	client := &http.Client{Timeout: 2 * time.Second}

	userProxy, err := proxyHandler("http://user-service:8081")
	if err != nil {
		log.Fatal(err)
	}

	orderProxy, err := proxyHandler("http://order-service:8082")
	if err != nil {
		log.Fatal(err)
	}

	mux.Handle("/users/", userProxy)
	mux.Handle("/orders", orderProxy)
	mux.Handle("/avatar-url", userProxy)
	mux.Handle("/dashboard", dashboardHandler(client))

	log.Println("gateway listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", authMiddleware(publicKey, mux)))
}
