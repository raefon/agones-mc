package fileserver

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// MaxFileSize: 100MB (5KB was too small for Minecraft worlds/configs)
const MaxFileSize int64 = 100 << 20

func GetFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	// 1. Clean and join the volume path with the URL path
	// This prevents ".." directory traversal attacks
	targetPath := filepath.Join(vol, filepath.Clean(r.URL.Path))

	// Ensure the user hasn't tried to escape the volume directory
	if !strings.HasPrefix(targetPath, vol) {
		http.Error(rw, "Forbidden", http.StatusForbidden)
		return nil
	}

	// 2. Check if the path is a directory or a file
	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		http.NotFound(rw, r)
		return nil
	}

	if info.IsDir() {
		// 3. If Directory: Return JSON list of files
		files, err := os.ReadDir(targetPath)
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return err
		}

		fileList := []map[string]interface{}{}
		for _, f := range files {
			finfo, _ := f.Info()
			fileList = append(fileList, map[string]interface{}{
				"name":  f.Name(),
				"isDir": f.IsDir(),
				"size":  finfo.Size(),
			})
		}

		rw.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(rw).Encode(fileList)
	}

	// 4. If File: Serve the actual file content (for downloading)
	http.ServeFile(rw, r, targetPath)
	return nil
}

func UploadFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	// Limit upload size
	r.Body = http.MaxBytesReader(rw, r.Body, MaxFileSize)

	if err := r.ParseMultipartForm(MaxFileSize); err != nil {
		http.Error(rw, "File too large or invalid form", http.StatusRequestEntityTooLarge)
		return err
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(rw, "Missing file field in form", http.StatusBadRequest)
		return err
	}
	defer file.Close()

	// Determine destination: if URL is /data/subfolder, put it there
	targetDir := filepath.Join(vol, filepath.Clean(r.URL.Path))
	// If the URL path is a file, we use the directory containing it
	stat, err := os.Stat(targetDir)
	if err == nil && !stat.IsDir() {
		targetDir = filepath.Dir(targetDir)
	}

	destPath := filepath.Join(targetDir, header.Filename)

	f, err := os.Create(destPath)
	if err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, file); err != nil {
		http.Error(rw, "Failed to save file", http.StatusInternalServerError)
		return err
	}

	rw.WriteHeader(http.StatusCreated)
	return nil
}

func DeleteFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	targetPath := filepath.Join(vol, filepath.Clean(r.URL.Path))

	// Safety check: Don't allow deleting the root volume itself
	if targetPath == vol {
		http.Error(rw, "Cannot delete volume root", http.StatusForbidden)
		return nil
	}

	if err := os.RemoveAll(targetPath); err != nil {
		http.Error(rw, err.Error(), http.StatusInternalServerError)
		return err
	}

	rw.WriteHeader(http.StatusNoContent)
	return nil
}
