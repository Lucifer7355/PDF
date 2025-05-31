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

type MetadataRequest struct {
	Title    string `json:"title"`
	Author   string `json:"author"`
	Keywords string `json:"keywords"`
}

type ErrorResponse8 struct {
	Error string `json:"error"`
}

func jsonError8(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func SetMetadataHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[SetMetadataHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		jsonError(w, "Invalid multipart form data", http.StatusBadRequest)
		return
	}
	fmt.Println("[SetMetadataHandler] ✅ Parsed multipart form")

	meta := r.FormValue("meta")
	if meta == "" {
		jsonError(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}

	var metaReq MetadataRequest
	if err := json.Unmarshal([]byte(meta), &metaReq); err != nil {
		jsonError(w, "Invalid JSON in 'meta'", http.StatusBadRequest)
		return
	}

	fh := r.MultipartForm.File["file"][0]
	file, err := fh.Open()
	if err != nil {
		jsonError(w, "Unable to read uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	inputTmp, err := os.CreateTemp("", "metadata-in-*.pdf")
	if err != nil {
		jsonError(w, "Failed to create temp input file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(inputTmp.Name())

	if _, err := io.Copy(inputTmp, file); err != nil {
		jsonError(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	inputTmp.Close()

	metaTxt := inputTmp.Name() + ".txt"
	err = os.WriteFile(metaTxt, []byte(fmt.Sprintf(
		"InfoKey: Title\nInfoValue: %s\nInfoKey: Author\nInfoValue: %s\nInfoKey: Keywords\nInfoValue: %s\n",
		metaReq.Title, metaReq.Author, metaReq.Keywords,
	)), 0644)
	if err != nil {
		jsonError(w, "Failed to create metadata file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(metaTxt)

	outputTmp := inputTmp.Name() + "-output.pdf"
	cmd := exec.Command("pdftk", inputTmp.Name(), "update_info", metaTxt, "output", outputTmp)
	err = cmd.Run()
	if err != nil {
		jsonError(w, "Failed to apply metadata to PDF", http.StatusInternalServerError)
		return
	}
	defer os.Remove(outputTmp)

	outFile, err := os.Open(outputTmp)
	if err != nil {
		jsonError(w, "Failed to open output PDF", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	key := "metadata/" + filepath.Base(outputTmp)
	url, err := utils.UploadStreamToR2(key, outFile)
	if err != nil {
		jsonError(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Metadata added successfully",
		"pdf_url": url,
	})
	fmt.Println("[SetMetadataHandler] ✅ Metadata updated & uploaded to:", url)
}
