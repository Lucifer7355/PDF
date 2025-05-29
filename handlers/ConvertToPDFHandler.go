package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/Lucifer7355/PDF/utils"
)

type ConvertPDFError struct {
	Error string `json:"error"`
}

func jsonConvertPDFError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ConvertPDFError{Error: msg})
}

func ConvertToPDFHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[ConvertToPDFHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(20 << 20) // 20MB max
	if err != nil {
		jsonConvertPDFError(w, "Invalid multipart form data", http.StatusBadRequest)
		return
	}

	// Validate file
	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		jsonConvertPDFError(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}

	fh := files[0]
	file, err := fh.Open()
	if err != nil {
		jsonConvertPDFError(w, "Unable to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	ext := filepath.Ext(fh.Filename)
	inputPath := filepath.Join(os.TempDir(), fmt.Sprintf("uploaded-%d%s", time.Now().UnixNano(), ext))
	outputPath := inputPath[:len(inputPath)-len(ext)] + ".pdf"

	// Save uploaded file
	tmpFile, err := os.Create(inputPath)
	if err != nil {
		jsonConvertPDFError(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(inputPath)
	defer tmpFile.Close()

	io.Copy(tmpFile, file)

	// Convert to PDF using unoconv
	cmd := exec.Command("unoconv", "-f", "pdf", inputPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		jsonConvertPDFError(w, "unoconv failed: "+string(out), http.StatusInternalServerError)
		return
	}
	defer os.Remove(outputPath)

	// Upload to R2
	pdfFile, err := os.Open(outputPath)
	if err != nil {
		jsonConvertPDFError(w, "Failed to open converted PDF", http.StatusInternalServerError)
		return
	}
	defer pdfFile.Close()

	uploadKey := fmt.Sprintf("converted/%d.pdf", time.Now().UnixNano())
	url, err := utils.UploadStreamToR2(uploadKey, pdfFile)
	if err != nil {
		jsonConvertPDFError(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}

	// Respond with URL
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})

	fmt.Println("[ConvertToPDFHandler] ✅ Done in", time.Since(start))
}
