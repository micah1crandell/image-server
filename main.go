package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	mu           sync.RWMutex
	currentImage string
	uploadDir    = "uploads"
	allowedTypes = map[string]bool{
		"image/jpeg":               true,
		"image/png":                true,
		"image/gif":                true,
		"image/webp":               true,
		"application/octet-stream": true,
	}
)

type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func main() {
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	http.Handle("/", http.FileServer(http.Dir("static")))
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/stream", streamHandler)
	http.HandleFunc("/select-image", selectImageHandler)
	http.HandleFunc("/images", listImagesHandler)
	http.HandleFunc("/current-image", currentImageHandler)

	http.Handle("/uploads/", http.StripPrefix("/uploads/",
		http.FileServer(http.Dir(uploadDir))),
	)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Failed to create upload directory: %v", err)
	}

	if err := os.WriteFile(filepath.Join(uploadDir, "test.txt"), []byte("test"), 0644); err != nil {
		log.Fatalf("Upload directory not writable: %v", err)
	}
	os.Remove(filepath.Join(uploadDir, "test.txt"))

	log.Println("Server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func respondJSON(w http.ResponseWriter, status int, response Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, Response{
			Success: false,
			Message: "Method not allowed",
		})
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Invalid file upload",
		})
		return
	}
	defer file.Close()

	// Verify MIME type
	buff := make([]byte, 512)
	if _, err = file.Read(buff); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Error reading file content",
		})
		return
	}

	mimeType := http.DetectContentType(buff)
	if !allowedTypes[mimeType] {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Unsupported file type: " + mimeType,
		})
		return
	}

	if _, err = file.Seek(0, io.SeekStart); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Error processing file",
		})
		return
	}

	// Generate unique filename
	filename := fmt.Sprintf("%d-%s", time.Now().UnixNano(), header.Filename)
	dstPath := filepath.Join(uploadDir, filename)
	dst, err := os.Create(dstPath)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Error creating file on server",
		})
		return
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Error saving file content",
		})
		return
	}

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "File uploaded successfully",
		Data:    map[string]string{"filename": filename},
	})
}

func streamHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	filename := currentImage
	mu.RUnlock()

	if filename == "" {
		http.Error(w, "No image currently selected", http.StatusNotFound)
		return
	}

	filePath := filepath.Join(uploadDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Selected image no longer exists", http.StatusNotFound)
		return
	}

	http.ServeFile(w, r, filePath)
}

func selectImageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		respondJSON(w, http.StatusMethodNotAllowed, Response{
			Success: false,
			Message: "Method not allowed",
		})
		return
	}

	filename := r.FormValue("filename")
	if filename == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Message: "Missing filename parameter",
		})
		return
	}

	filePath := filepath.Join(uploadDir, filename)
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Message: "File not found",
		})
		return
	}

	mu.Lock()
	currentImage = filename
	mu.Unlock()

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "Image selected successfully",
	})
}

func listImagesHandler(w http.ResponseWriter, r *http.Request) {
	files, err := os.ReadDir(uploadDir)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Message: "Error reading upload directory",
		})
		return
	}

	filenames := make([]string, 0, len(files))
	for _, file := range files {
		if !file.IsDir() {
			filenames = append(filenames, file.Name())
		}
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    filenames,
	})
}

func currentImageHandler(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	current := currentImage
	mu.RUnlock()

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    map[string]string{"current": current},
	})
}
