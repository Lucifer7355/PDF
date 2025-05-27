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

func MergeHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[MergeHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(20 << 20) // 20MB memory limit for form parsing
	if err != nil {
		fmt.Println("[MergeHandler] ❌ Error parsing form:", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) < 2 {
		fmt.Println("[MergeHandler] ❌ Less than 2 files uploaded")
		http.Error(w, "Please upload at least 2 PDF files to merge", http.StatusBadRequest)
		return
	}

	var inputPaths []string
	for i, fh := range files {
		file, err := fh.Open()
		if err != nil {
			fmt.Printf("[MergeHandler] ❌ Error opening file %d: %v\n", i+1, err)
			continue
		}
		defer file.Close()

		tmp, err := os.CreateTemp("", "merge-*.pdf")
		if err != nil {
			fmt.Printf("[MergeHandler] ❌ Error creating temp file for file %d: %v\n", i+1, err)
			continue
		}
		defer os.Remove(tmp.Name()) // cleanup after handler exits

		_, err = io.Copy(tmp, file)
		if err != nil {
			fmt.Printf("[MergeHandler] ❌ Error copying file %d: %v\n", i+1, err)
			tmp.Close()
			continue
		}
		tmp.Close()

		inputPaths = append(inputPaths, tmp.Name())
		fmt.Printf("[MergeHandler] ✅ File %d saved to: %s\n", i+1, tmp.Name())
	}

	out := filepath.Join(os.TempDir(), "merged.pdf")
	defer os.Remove(out)

	err = api.MergeCreateFile(inputPaths, out, false, nil)
	if err != nil {
		fmt.Println("[MergeHandler] ❌ Merge failed:", err)
		http.Error(w, "Failed to merge PDFs", http.StatusInternalServerError)
		return
	}
	fmt.Println("[MergeHandler] ✅ Merge successful. Output file:", out)

	// Open file for streaming
	f, err := os.Open(out)
	if err != nil {
		fmt.Println("[MergeHandler] ❌ Failed to open merged file:", err)
		http.Error(w, "Failed to open merged PDF", http.StatusInternalServerError)
		return
	}
	defer f.Close()

	url, err := utils.UploadStreamToR2("merged/merged.pdf", f)
	if err != nil {
		fmt.Println("[MergeHandler] ❌ Failed to upload to R2:", err)
		http.Error(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}

	fmt.Printf("[MergeHandler] ✅ Merged PDF uploaded to R2: %s\n", url)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"url":"` + url + `"}`))

	fmt.Println("[MergeHandler] ✅ Response sent in", time.Since(start))
}
