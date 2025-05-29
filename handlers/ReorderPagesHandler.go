package handlers

import (
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

type ErrorResponse3 struct {
	Error string `json:"error"`
}

func jsonError3(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse3{Error: msg})
}

func ReorderPagesHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[ReorderPagesHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		jsonError3(w, "Invalid multipart form data", http.StatusBadRequest)
		return
	}

	meta := r.FormValue("meta")
	if meta == "" {
		jsonError3(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}

	var req ReorderPagesRequest
	if err := json.Unmarshal([]byte(meta), &req); err != nil {
		jsonError3(w, "Invalid JSON in 'meta'", http.StatusBadRequest)
		return
	}

	if len(req.Order) == 0 {
		jsonError3(w, "Order must contain at least one page number", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		jsonError3(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}

	fh := files[0]
	file, err := fh.Open()
	if err != nil {
		jsonError3(w, "Unable to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save input PDF
	inputPath := filepath.Join(os.TempDir(), fmt.Sprintf("uploaded-%d.pdf", time.Now().UnixNano()))
	tmpFile, err := os.Create(inputPath)
	if err != nil {
		jsonError3(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(inputPath)
	defer tmpFile.Close()
	io.Copy(tmpFile, file)

	// Output path
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("reordered-%d.pdf", time.Now().UnixNano()))
	defer os.Remove(outputPath)

	// Build qpdf page order args
	pageOrderStrs := make([]string, len(req.Order))
	for i, n := range req.Order {
		pageOrderStrs[i] = fmt.Sprint(n)
	}

	// qpdf: input.pdf --pages . 3 1 2 -- output.pdf
	args := append([]string{inputPath, "--pages", ".", strings.Join(pageOrderStrs, " "), "--", outputPath})
	cmd := exec.Command("qpdf", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		jsonError3(w, "qpdf failed: "+string(out), http.StatusBadRequest)
		return
	}

	// Upload to R2
	resultFile, err := os.Open(outputPath)
	if err != nil {
		jsonError3(w, "Failed to open result file", http.StatusInternalServerError)
		return
	}
	defer resultFile.Close()

	uploadKey := fmt.Sprintf("reordered/%d.pdf", time.Now().UnixNano())
	url, err := utils.UploadStreamToR2(uploadKey, resultFile)
	if err != nil {
		jsonError3(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}

	// Respond with result
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})

	fmt.Println("[ReorderPagesHandler] ✅ Done in", time.Since(start))
}
