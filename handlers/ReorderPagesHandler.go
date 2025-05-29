package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lucifer7355/PDF/utils"
)

type ReorderPagesRequest struct {
	Order []int `json:"order"`
}

func jsonError5(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
	fmt.Printf("[ReorderPagesHandler] ❌ Error (%d): %s\n", code, msg)
}

func ReorderPagesHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[ReorderPagesHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		jsonError(w, "Invalid form data", http.StatusBadRequest)
		return
	}
	fmt.Println("[ReorderPagesHandler] ✅ Parsed multipart form successfully")

	meta := r.FormValue("meta")
	if meta == "" {
		jsonError(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}
	fmt.Println("[ReorderPagesHandler] ✅ Received meta field")

	var req ReorderPagesRequest
	if err := json.Unmarshal([]byte(meta), &req); err != nil {
		jsonError(w, "Invalid JSON in 'meta'", http.StatusBadRequest)
		return
	}
	fmt.Printf("[ReorderPagesHandler] ✅ Parsed order: %v\n", req.Order)

	if len(req.Order) == 0 {
		jsonError(w, "Page order is required", http.StatusBadRequest)
		return
	}

	fh := r.MultipartForm.File["file"][0]
	src, err := fh.Open()
	if err != nil {
		jsonError(w, "Failed to open uploaded file", http.StatusBadRequest)
		return
	}
	defer src.Close()
	fmt.Printf("[ReorderPagesHandler] ✅ File uploaded: %s (%d bytes)\n", fh.Filename, fh.Size)

	inputPath := filepath.Join(os.TempDir(), fmt.Sprintf("input-%d.pdf", time.Now().UnixNano()))
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("output-%d.pdf", time.Now().UnixNano()))
	fmt.Printf("[ReorderPagesHandler] ➜ Saving uploaded file to: %s\n", inputPath)

	inFile, err := os.Create(inputPath)
	if err != nil {
		jsonError(w, "Failed to save input file", http.StatusInternalServerError)
		return
	}
	_, _ = io.Copy(inFile, src)
	inFile.Close()
	defer os.Remove(inputPath)
	defer os.Remove(outputPath)
	fmt.Println("[ReorderPagesHandler] ✅ File saved successfully")

	orderStrs := make([]string, len(req.Order))
	for i, v := range req.Order {
		orderStrs[i] = fmt.Sprintf("%d", v)
	}
	orderArg := strings.Join(orderStrs, ",")
	fmt.Printf("[ReorderPagesHandler] ➜ Running pdfcpu selectedPages with order: %s\n", orderArg)

	cmd := exec.Command("pdfcpu", "selectedPages", inputPath, outputPath, orderArg)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	fmt.Printf("[ReorderPagesHandler] 💡 Executing command: %s\n", cmd.String())
	if err := cmd.Run(); err != nil {
		fmt.Println("[ReorderPagesHandler] ❌ pdfcpu error output:", stderr.String())
		jsonError(w, "pdfcpu failed: "+stderr.String(), http.StatusInternalServerError)
		return
	}
	fmt.Println("[ReorderPagesHandler] ✅ pdfcpu command executed successfully")

	outFile, err := os.Open(outputPath)
	if err != nil {
		jsonError(w, "Failed to open output file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()
	fmt.Println("[ReorderPagesHandler] ✅ Output file opened successfully")

	uploadKey := fmt.Sprintf("reordered/%d.pdf", time.Now().UnixNano())
	fmt.Printf("[ReorderPagesHandler] ➜ Uploading to R2 with key: %s\n", uploadKey)
	url, err := utils.UploadStreamToR2(uploadKey, outFile)
	if err != nil {
		jsonError(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}
	fmt.Printf("[ReorderPagesHandler] ✅ Uploaded successfully. URL: %s\n", url)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})

	fmt.Println("[ReorderPagesHandler] ✅ Completed in", time.Since(start))
}
