package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "password123"
	dbname   = "mydatabase"
)

func enableCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set headers
		w.Header().Set("Access-Control-Allow-Origin", "*") // Allow any domain, adjust if you need more restrictive settings
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")

		// If it's a preflight OPTIONS request, send a simple response and stop processing
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// Call the next handler, which can be another middleware in the chain or the final handler
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Database connection string
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)

	// Connect to the database
	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Test the connection
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// Router configuration
	router := mux.NewRouter()

	handler := enableCORS(router)

	// Define routes
	router.HandleFunc("/favorites", getFavorites(db)).Methods("GET")
	router.HandleFunc("/favorites", postFavorite(db)).Methods("POST")
	router.HandleFunc("/favorites/{favoriteId}", deleteFavorite(db)).Methods("DELETE")
	router.HandleFunc("/items", getItems(db)).Methods("GET")

	// Start the server
	log.Fatal(http.ListenAndServe(":8080", handler))
}

func getFavorites(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Joining favorites with sneakers on item_id to fetch related sneaker details
		query := `
        SELECT f.id, f.item_id, s.title, s.price, s.imageUrl, s.isFavorite, s.favoriteId, s.isAdded
        FROM favorite f
        INNER JOIN sneakers s ON f.item_id = s.id`

		rows, err := db.Query(query)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var favorites []struct {
			ID         int    `json:"id"`
			ItemID     int    `json:"item_id"`
			Title      string `json:"title"`
			Price      int    `json:"price"`
			ImageURL   string `json:"image_url"`
			IsFavorite bool   `json:"is_favorite"`
			FavoriteID *int   `json:"favorite_id"`
			IsAdded    bool   `json:"is_added"`
		}

		for rows.Next() {
			var f struct {
				ID         int    `json:"id"`
				ItemID     int    `json:"item_id"`
				Title      string `json:"title"`
				Price      int    `json:"price"`
				ImageURL   string `json:"image_url"`
				IsFavorite bool   `json:"is_favorite"`
				FavoriteID *int   `json:"favorite_id"`
				IsAdded    bool   `json:"is_added"`
			}
			if err := rows.Scan(&f.ID, &f.ItemID, &f.Title, &f.Price, &f.ImageURL, &f.IsFavorite, &f.FavoriteID, &f.IsAdded); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			favorites = append(favorites, f)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(favorites)
	}
}

func postFavorite(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var data struct {
			ItemID int `json:"item_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		_, err := db.Exec("INSERT INTO favorite (item_id) VALUES ($1)", data.ItemID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
	}
}

func deleteFavorite(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		favoriteId, err := strconv.Atoi(vars["favoriteId"])
		if err != nil {
			http.Error(w, "Invalid favorite ID", http.StatusBadRequest)
			return
		}

		_, err = db.Exec("DELETE FROM favorite WHERE id = $1", favoriteId)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

func getItems(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		params := r.URL.Query()
		sortBy := params.Get("sortBy")
		searchQuery := params.Get("title")

		// Start with the base query
		query := "SELECT * FROM sneakers"

		// Add filtering by title if a search query is provided
		if searchQuery != "" {
			// Use parameterized SQL to safely add user input to the query
			query += " WHERE title ILIKE $1"
		}

		// Add sorting if a sort parameter is provided
		if sortBy != "" {
			query += fmt.Sprintf(" ORDER BY %s", sortBy) // Note: Potential SQL injection vulnerability, see explanation below
		}

		var rows *sql.Rows
		var err error

		if searchQuery != "" {
			// Execute the query with the parameter for safety
			rows, err = db.Query(query, "%"+searchQuery+"%")
		} else {
			// Execute the query without parameters if there is no search query
			rows, err = db.Query(query)
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var items []struct {
			ID         int    `json:"id"`
			Title      string `json:"title"`
			Price      int    `json:"price"`
			ImageURL   string `json:"image_url"`
			IsFavorite bool   `json:"is_favorite"`
			FavoriteID *int   `json:"favorite_id"`
			IsAdded    bool   `json:"is_added"`
		}

		for rows.Next() {
			var i struct {
				ID         int    `json:"id"`
				Title      string `json:"title"`
				Price      int    `json:"price"`
				ImageURL   string `json:"image_url"`
				IsFavorite bool   `json:"is_favorite"`
				FavoriteID *int   `json:"favorite_id"`
				IsAdded    bool   `json:"is_added"`
			}
			if err := rows.Scan(&i.ID, &i.Title, &i.Price, &i.ImageURL, &i.IsFavorite, &i.FavoriteID, &i.IsAdded); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			items = append(items, i)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(items)
	}
}
