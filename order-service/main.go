package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type Order struct {
	ID     string  `json:"id"`
	UserID string  `json:"userId"`
	Total  float64 `json:"total"`
	Status string  `json:"status"`
}

var orders = []Order{
	{ID: "o1", UserID: "u1", Total: 120000, Status: "paid"},
	{ID: "o2", UserID: "u1", Total: 85000, Status: "shipped"},
	{ID: "o3", UserID: "u2", Total: 50000, Status: "pending"},
}

func main() {
	http.HandleFunc("/orders", func(w http.ResponseWriter, r *http.Request) {
		userID := r.URL.Query().Get("userId")

		result := make([]Order, 0)
		for _, order := range orders {
			if order.UserID == userID {
				result = append(result, order)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	log.Println("order service listening on :8082")
	log.Fatal(http.ListenAndServe(":8082", nil))
}
