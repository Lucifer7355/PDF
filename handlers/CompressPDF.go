package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/Lucifer7355/PDF/utils"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func CompressHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[CompressHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(10 << 20) // 10 MB
	if err != nil {
		fmt.Println("[CompressHandler] ❌ Error parsing form:", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	fh := r.MultipartForm.File["file"][0]
	file, err := fh.Open()
	if err != nil {
		fmt.Println("[CompressHandler] ❌ Error opening uploaded file:", err)
		http.Error(w, "Could not read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	inFile, err := os.CreateTemp("", "compress-in-*.pdf")
	if err != nil {
		fmt.Println("[CompressHandler] ❌ Error creating temp input file:", err)
		http.Error(w, "Could not create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(inFile.Name())

	_, err = io.Copy(inFile, file)
	if err != nil {
		fmt.Println("[CompressHandler] ❌ Error saving uploaded file:", err)
		inFile.Close()
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	inFile.Close()

	fmt.Println("[CompressHandler] ✅ Uploaded file saved to:", inFile.Name())

	outFile := filepath.Join(os.TempDir(), "compressed.pdf")
	defer os.Remove(outFile)

	err = api.OptimizeFile(inFile.Name(), outFile, nil)
	if err != nil {
		fmt.Println("[CompressHandler] ❌ PDF compression failed:", err)
		http.Error(w, "Failed to compress PDF", http.StatusInternalServerError)
		return
	}

	fmt.Println("[CompressHandler] ✅ Compression successful:", outFile)

	// Stream compressed file instead of reading it entirely into memory
	f, err := os.Open(outFile)
	if err != nil {
		fmt.Println("[CompressHandler] ❌ Error opening compressed file:", err)
		http.Error(w, "Failed to open compressed PDF", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	url, err := utils.UploadStreamToR2("compressed/compressed.pdf", f)
	if err != nil {
		fmt.Println("[CompressHandler] ❌ Failed to upload to R2:", err)
		http.Error(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}

	fmt.Printf("[CompressHandler] ✅ Compressed PDF uploaded to R2: %s\n", url)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"url":"` + url + `"}`))

	fmt.Println("[CompressHandler] ✅ Response sent in", time.Since(start))
}
