package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Lucifer7355/PDF/utils"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

type SplitRequest struct {
	Mode   string   `json:"mode"`   // "range" or "count"
	Ranges []string `json:"ranges"` // used if mode == "range"
	Count  int      `json:"count"`  // used if mode == "count"
}

type FileUpload struct {
	Filename string `json:"filename"`
	URL      string `json:"url"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

func normalizeRanges(textualRanges []string) []string {
	var output []string
	rangePattern := regexp.MustCompile(`(?i)page(?:s)?\s+(\d+)(?:\s*(?:to|through|-)\s*(\d+))?`)
	for _, raw := range textualRanges {
		raw = strings.TrimSpace(raw)
		matches := rangePattern.FindAllStringSubmatch(raw, -1)
		for _, m := range matches {
			from := m[1]
			to := m[2]
			if to == "" {
				output = append(output, from)
			} else {
				output = append(output, from+"-"+to)
			}
		}
	}
	return output
}

func validateRanges(ranges []string, totalPages int) error {
	for _, r := range ranges {
		parts := strings.Split(r, "-")
		if len(parts) == 1 {
			page, err := strconv.Atoi(parts[0])
			if err != nil || page < 1 || page > totalPages {
				return fmt.Errorf("Invalid page number: %s (out of bounds)", r)
			}
		} else if len(parts) == 2 {
			from, err1 := strconv.Atoi(parts[0])
			to, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil || from < 1 || to > totalPages || from > to {
				return fmt.Errorf("Invalid range: %s (out of bounds or reversed)", r)
			}
		} else {
			return fmt.Errorf("Invalid range format: %s", r)
		}
	}
	return nil
}

func SplitHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	fmt.Println("[SplitHandler] ➜ Received request at", start.Format(time.RFC3339))

	err := r.ParseMultipartForm(20 << 20)
	if err != nil {
		jsonError(w, "Invalid multipart form data", http.StatusBadRequest)
		return
	}

	meta := r.FormValue("meta")
	if meta == "" {
		jsonError(w, "Missing 'meta' field", http.StatusBadRequest)
		return
	}

	var req SplitRequest
	if err := json.Unmarshal([]byte(meta), &req); err != nil {
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

	inputTmp, err := os.CreateTemp("", "split-*.pdf")
	if err != nil {
		jsonError(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(inputTmp.Name())

	if _, err := io.Copy(inputTmp, file); err != nil {
		jsonError(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}
	inputTmp.Close()

	outputDir, err := os.MkdirTemp("", "pdfsplit-")
	if err != nil {
		jsonError(w, "Failed to create output directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(outputDir)

	switch req.Mode {
	case "count":
		if req.Count <= 0 {
			jsonError(w, "Invalid 'count' value", http.StatusBadRequest)
			return
		}
		err = api.SplitFile(inputTmp.Name(), outputDir, req.Count, nil)

	case "range":
		if len(req.Ranges) == 0 {
			jsonError(w, "No 'ranges' provided", http.StatusBadRequest)
			return
		}

		normalized := normalizeRanges(req.Ranges)

		pageCount, err := api.PageCountFile(inputTmp.Name())
		if err != nil {
			jsonError(w, "Failed to read PDF page count", http.StatusInternalServerError)
			return
		}

		if err := validateRanges(normalized, pageCount); err != nil {
			jsonError(w, err.Error(), http.StatusBadRequest)
			return
		}

		for _, r := range normalized {
			err = api.ExtractPagesFile(inputTmp.Name(), outputDir, []string{r}, nil)
			if err != nil {
				break
			}
		}

	default:
		jsonError(w, "Invalid split mode. Use 'range' or 'count'", http.StatusBadRequest)
		return
	}

	if err != nil {
		jsonError(w, "Failed to split PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	files, err := os.ReadDir(outputDir)
	if err != nil {
		jsonError(w, "Error reading split files", http.StatusInternalServerError)
		return
	}

	var uploads []FileUpload
	for _, f := range files {
		path := filepath.Join(outputDir, f.Name())
		splitFile, err := os.Open(path)
		if err != nil {
			continue
		}
		defer splitFile.Close()

		url, err := utils.UploadStreamToR2("split/"+f.Name(), splitFile)
		if err != nil {
			continue
		}

		uploads = append(uploads, FileUpload{
			Filename: f.Name(),
			URL:      url,
		})
		fmt.Printf("[SplitHandler] ✅ Uploaded split part: %s\n", url)
	}

	// ✅ Return consistent JSON response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": uploads,
	})
	fmt.Println("[SplitHandler] ✅ Split & upload done in", time.Since(start))
}
