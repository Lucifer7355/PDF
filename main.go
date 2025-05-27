// main.go
package main

import (
	"log"
	"net/http"

	"github.com/Lucifer7355/PDF/handlers"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ .env file not found, relying on system env vars")
	}
	http.HandleFunc("/health", handlers.HealthHandler)
	http.HandleFunc("/merge", handlers.MergeHandler)
	http.HandleFunc("/compress", handlers.CompressHandler)
	http.HandleFunc("/split", handlers.SplitHandler)
	http.HandleFunc("/pdf-security", handlers.EncryptOrDecryptHandler)

	log.Println("PDF Toolbox running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
