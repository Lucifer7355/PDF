package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Lucifer7355/PDF/utils"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
)

type ReorderPagesRequest struct {
	Order []int `json:"order"`
}

func jsonError6(w http.ResponseWriter, msg string, code int) {
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
		jsonError(w, "Invalid multipart form data", http.StatusBadRequest)
		return
	}
	fmt.Println("[ReorderPagesHandler] ✅ Parsed multipart form")

	meta := r.FormValue("meta")
	if meta == "" {
		jsonError(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}

	var req ReorderPagesRequest
	if err := json.Unmarshal([]byte(meta), &req); err != nil {
		jsonError(w, "Invalid JSON in 'meta'", http.StatusBadRequest)
		return
	}
	fmt.Printf("[ReorderPagesHandler] ✅ Parsed order: %v\n", req.Order)

	files := r.MultipartForm.File["file"]
	if len(files) == 0 {
		jsonError(w, "Missing 'file' field", http.StatusBadRequest)
		return
	}

	fh := files[0]
	src, err := fh.Open()
	if err != nil {
		jsonError(w, "Failed to open uploaded file", http.StatusBadRequest)
		return
	}
	defer src.Close()

	tmpDir := os.TempDir()
	inputPath := filepath.Join(tmpDir, fmt.Sprintf("input-%d.pdf", time.Now().UnixNano()))
	outDir := filepath.Join(tmpDir, fmt.Sprintf("extract-%d", time.Now().UnixNano()))
	outputPath := filepath.Join(tmpDir, fmt.Sprintf("reordered-%d.pdf", time.Now().UnixNano()))

	err = os.MkdirAll(outDir, os.ModePerm)
	if err != nil {
		jsonError(w, "Failed to create extraction dir", http.StatusInternalServerError)
		return
	}

	outFile, err := os.Create(inputPath)
	if err != nil {
		jsonError(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	_, _ = io.Copy(outFile, src)
	outFile.Close()

	fmt.Printf("[ReorderPagesHandler] ✅ Uploaded file saved: %s\n", inputPath)

	conf := model.NewDefaultConfiguration()

	// Step 1: Get total pages
	ctx, err := api.ReadContextFile(inputPath)
	if err != nil {
		jsonError(w, "Failed to read PDF context", http.StatusInternalServerError)
		return
	}
	totalPages := ctx.PageCount
	fmt.Printf("[ReorderPagesHandler] 📄 Total pages in input PDF: %d\n", totalPages)

	// Step 2: Create final reordered list
	orderedMap := make(map[int]bool)
	finalOrder := append([]int{}, req.Order...)
	for _, p := range req.Order {
		orderedMap[p] = true
	}
	for i := 1; i <= totalPages; i++ {
		if !orderedMap[i] {
			finalOrder = append(finalOrder, i)
		}
	}
	fmt.Printf("[ReorderPagesHandler] 🧩 Final page order: %v\n", finalOrder)

	// Step 3: Extract and append all in correct order
	var extractedFiles []string
	for _, page := range finalOrder {
		pageStr := fmt.Sprintf("%d", page)
		err := api.ExtractPagesFile(inputPath, outDir, []string{pageStr}, conf)
		if err != nil {
			jsonError(w, fmt.Sprintf("pdfcpu extract failed: %v", err), http.StatusInternalServerError)
			return
		}
		filename := fmt.Sprintf("%s_page_%d.pdf", strings.TrimSuffix(filepath.Base(inputPath), ".pdf"), page)
		extractedPath := filepath.Join(outDir, filename)
		extractedFiles = append(extractedFiles, extractedPath)
		fmt.Printf("[ReorderPagesHandler] ✅ Extracted page %d to %s\n", page, extractedPath)
	}

	err = api.MergeCreateFile(extractedFiles, outputPath, false, conf)
	if err != nil {
		jsonError(w, fmt.Sprintf("pdfcpu reorder failed: %v", err), http.StatusInternalServerError)
		return
	}
	fmt.Printf("[ReorderPagesHandler] ✅ Reordered PDF created: %s\n", outputPath)

	finalFile, err := os.Open(outputPath)
	if err != nil {
		jsonError(w, "Failed to open final output file", http.StatusInternalServerError)
		return
	}
	defer finalFile.Close()

	uploadKey := fmt.Sprintf("reordered/%d.pdf", time.Now().UnixNano())
	url, err := utils.UploadStreamToR2(uploadKey, finalFile)
	if err != nil {
		jsonError(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}
	fmt.Printf("[ReorderPagesHandler] ✅ Uploaded to R2 at: %s\n", url)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"url": url})
	fmt.Println("[ReorderPagesHandler] ✅ Completed in", time.Since(start))
}
