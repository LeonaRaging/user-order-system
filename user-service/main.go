package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type User struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

var users = map[string]User{
	"u1": {ID: "u1", Name: "Alice"},
	"u2": {ID: "u2", Name: "Bob"},
}

func main() {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		endpoint = "localhost:9000"
	}

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(getenv("MINIO_ACCESS_KEY", "minioadmin"), getenv("MINIO_SECRET_KEY", "minioadmin"), ""),
		Secure: getenv("MINIO_USE_SSL", "false") == "true",
	})

	if err != nil {
		log.Fatal(err)
	}

	bucket := getenv("MINIO_BUCKET", "avatars")
	ctx := context.Background()

	for attempt := 0; attempt < 30; attempt++ {
		exists, err := client.BucketExists(ctx, bucket)
		if err == nil {
			if !exists {
				err = client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{})
			}
			if err == nil {
				break
			}
		}
		if attempt == 29 {
			log.Fatal(err)
		}
		time.Sleep(2 * time.Second)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/users/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/users/")
		user, ok := users[id]
		if !ok {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(user)
	})

	mux.HandleFunc("/avatar-url", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID := r.URL.Query().Get("userId")
		if _, ok := users[userID]; !ok {
			http.NotFound(w, r)
			return
		}

		objectName := userID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10) + ".jpg"

		presignedURL, err := client.PresignedPutObject(
			ctx,
			bucket,
			objectName,
			5*time.Minute,
		)
		if err != nil {
			http.Error(w, "failed to create upload URL", http.StatusInternalServerError)
			return
		}
		if publicEndpoint := os.Getenv("MINIO_PUBLIC_ENDPOINT"); publicEndpoint != "" {
			publicURLValue := publicEndpoint
			if !strings.Contains(publicURLValue, "://") {
				publicURLValue = "http://" + publicURLValue
			}
			publicURL, err := url.Parse(publicURLValue)
			if err != nil {
				http.Error(w, "invalid public MinIO endpoint", http.StatusInternalServerError)
				return
			}
			presignedURL.Host = publicURL.Host
			presignedURL.Scheme = publicURL.Scheme
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"uploadUrl": presignedURL.String(),
			"expiresIn": 300,
			"object":    objectName,
		})
	})

	log.Println("user service listening on :8081")
	log.Fatal(http.ListenAndServe(":8081", mux))
}

func getenv(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
