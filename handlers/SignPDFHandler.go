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

type SignMeta struct {
	Page  int     `json:"page"`
	X     int     `json:"x"`
	Y     int     `json:"y"`
	Scale float64 `json:"scale"`
}

type ErrorResponse4 struct {
	Error string `json:"error"`
}

func jsonError4(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse4{Error: msg})
}

func SignPDFHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[SignPDFHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(20 << 20)
	if err != nil {
		jsonError4(w, "Invalid multipart form data", http.StatusBadRequest)
		return
	}

	metaStr := r.FormValue("meta")
	if metaStr == "" {
		jsonError4(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}

	var meta SignMeta
	if err := json.Unmarshal([]byte(metaStr), &meta); err != nil {
		jsonError4(w, "Invalid JSON in 'meta'", http.StatusBadRequest)
		return
	}

	if meta.Page < 1 || meta.X < 0 || meta.Y < 0 || meta.Scale <= 0 {
		jsonError4(w, "Invalid meta parameters", http.StatusBadRequest)
		return
	}

	// Get and save PDF
	pdfFile, _, err := r.FormFile("file")
	if err != nil {
		jsonError4(w, "Missing or invalid 'file'", http.StatusBadRequest)
		return
	}
	defer pdfFile.Close()

	pdfPath := filepath.Join(os.TempDir(), fmt.Sprintf("input-%d.pdf", time.Now().UnixNano()))
	tmpPDF, err := os.Create(pdfPath)
	if err != nil {
		jsonError4(w, "Failed to save PDF", http.StatusInternalServerError)
		return
	}
	defer tmpPDF.Close()
	defer os.Remove(pdfPath)
	io.Copy(tmpPDF, pdfFile)

	// Get and save signature image
	sigFile, _, err := r.FormFile("signature")
	if err != nil {
		jsonError4(w, "Missing or invalid 'signature'", http.StatusBadRequest)
		return
	}
	defer sigFile.Close()

	sigPath := filepath.Join(os.TempDir(), fmt.Sprintf("sig-%d.png", time.Now().UnixNano()))
	tmpSig, err := os.Create(sigPath)
	if err != nil {
		jsonError4(w, "Failed to save signature image", http.StatusInternalServerError)
		return
	}
	defer tmpSig.Close()
	defer os.Remove(sigPath)
	io.Copy(tmpSig, sigFile)

	// Prepare output file
	outPath := filepath.Join(os.TempDir(), fmt.Sprintf("signed-%d.pdf", time.Now().UnixNano()))
	defer os.Remove(outPath)

	// Run pdfcpu insert command
	cmd := exec.Command("pdfcpu", "insert", "image", sigPath,
		"in", pdfPath,
		fmt.Sprintf("pos:bl"),
		fmt.Sprintf("page:%d", meta.Page),
		fmt.Sprintf("dx:%d", meta.X),
		fmt.Sprintf("dy:%d", meta.Y),
		fmt.Sprintf("scale:%.2f", meta.Scale),
		"output", outPath,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		jsonError4(w, "pdfcpu failed: "+string(out), http.StatusInternalServerError)
		return
	}

	// Upload to R2
	signedFile, err := os.Open(outPath)
	if err != nil {
		jsonError4(w, "Failed to open signed file", http.StatusInternalServerError)
		return
	}
	defer signedFile.Close()

	uploadKey := fmt.Sprintf("signed/%d.pdf", time.Now().UnixNano())
	url, err := utils.UploadStreamToR2(uploadKey, signedFile)
	if err != nil {
		jsonError4(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}

	// Respond
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})

	fmt.Println("[SignPDFHandler] ✅ Done in", time.Since(start))
}
