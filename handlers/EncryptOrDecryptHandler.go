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

type PDFSecurityRequest struct {
	Mode          string `json:"mode"` // "encrypt" or "decrypt"
	UserPassword  string `json:"userPassword"`
	OwnerPassword string `json:"ownerPassword,omitempty"`
	URL           string `json:"url,omitempty"` // Optional: PDF source URL
}

type ErrorResponse2 struct {
	Error string `json:"error"`
}

func jsonError2(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func EncryptOrDecryptHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[PDFSecurityHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		jsonError(w, "Invalid multipart form data", http.StatusBadRequest)
		return
	}

	meta := r.FormValue("meta")
	if meta == "" {
		jsonError(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}

	var req PDFSecurityRequest
	if err := json.Unmarshal([]byte(meta), &req); err != nil {
		jsonError(w, "Invalid JSON in 'meta'", http.StatusBadRequest)
		return
	}

	if req.Mode != "encrypt" && req.Mode != "decrypt" {
		jsonError(w, "Invalid mode. Use 'encrypt' or 'decrypt'", http.StatusBadRequest)
		return
	}

	if req.UserPassword == "" {
		jsonError(w, "userPassword is required", http.StatusBadRequest)
		return
	}

	inputPath := ""
	if req.URL != "" {
		resp, err := http.Get(req.URL)
		if err != nil || resp.StatusCode != 200 {
			jsonError(w, "Failed to fetch PDF from URL", http.StatusBadRequest)
			return
		}
		defer resp.Body.Close()

		tmp, err := os.CreateTemp("", "from-url-*.pdf")
		if err != nil {
			jsonError(w, "Failed to create temp file", http.StatusInternalServerError)
			return
		}
		defer tmp.Close()
		io.Copy(tmp, resp.Body)
		inputPath = tmp.Name()
	} else {
		fh := r.MultipartForm.File["file"][0]
		file, err := fh.Open()
		if err != nil {
			jsonError(w, "Unable to read uploaded file", http.StatusBadRequest)
			return
		}
		defer file.Close()

		tmpFile, err := os.CreateTemp("", "uploaded-*.pdf")
		if err != nil {
			jsonError(w, "Failed to save uploaded file", http.StatusInternalServerError)
			return
		}
		defer tmpFile.Close()
		io.Copy(tmpFile, file)
		inputPath = tmpFile.Name()
	}
	defer os.Remove(inputPath)

	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s-output.pdf", req.Mode))
	defer os.Remove(outputPath)

	var cmd *exec.Cmd
	if req.Mode == "encrypt" {
		owner := req.OwnerPassword
		if owner == "" {
			owner = req.UserPassword
		}
		cmd = exec.Command("qpdf", "--encrypt", req.UserPassword, owner, "256", "--", inputPath, outputPath)
	} else {
		cmd = exec.Command("qpdf", "--password="+req.UserPassword, "--decrypt", inputPath, outputPath)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		jsonError(w, "qpdf failed: "+string(out), http.StatusUnauthorized)
		return
	}

	resultFile, err := os.Open(outputPath)
	if err != nil {
		jsonError(w, "Failed to open result file", http.StatusInternalServerError)
		return
	}
	defer resultFile.Close()

	uploadKey := fmt.Sprintf("%s/result.pdf", req.Mode)
	url, err := utils.UploadStreamToR2(uploadKey, resultFile)
	if err != nil {
		jsonError(w, "Failed to upload to R2", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"url": url,
	})

	fmt.Println("[PDFSecurityHandler] ✅ Done in", time.Since(start))
}
