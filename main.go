package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Lucifer7355/PDF/handlers"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env only if not running in Railway
	if os.Getenv("RAILWAY_ENVIRONMENT") == "" {
		err := godotenv.Load()
		if err != nil {
			log.Println("⚠️  .env file not found, relying on system env vars")
		} else {
			log.Println("✅ .env file loaded successfully")
		}
	} else {
		log.Println("🏭 Running in Railway — using injected system env vars")
	}

	// Register routes
	http.HandleFunc("/health", handlers.HealthHandler)
	http.HandleFunc("/merge", handlers.MergeHandler)
	http.HandleFunc("/compress", handlers.CompressHandler)
	http.HandleFunc("/split", handlers.SplitHandler)
	http.HandleFunc("/pdf-security", handlers.EncryptOrDecryptHandler)
	http.HandleFunc("/convert-to-pdf", handlers.ConvertToPDFHandler)
	http.HandleFunc("/reorder-pages", handlers.ReorderPagesHandler)
	http.HandleFunc("/setMetadata", handlers.SetMetadataHandler)

	log.Println("📦 PDF Toolbox running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
