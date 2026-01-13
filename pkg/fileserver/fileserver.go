package fileserver

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const MaxFileSize int64 = 100 << 20

// TemplateData for the HTML UI
type TemplateData struct {
	CurrentPath string
	Files       []FileInfo
}

type FileInfo struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
}

// HTML Template using Tailwind CSS
const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Minecraft File Manager</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css">
</head>
<body class="bg-slate-900 text-slate-100 font-sans">
    <div class="max-w-5xl mx-auto py-10 px-4">
        <header class="mb-8 flex justify-between items-center">
            <div>
                <h1 class="text-3xl font-bold text-green-400"><i class="fas fa-cubes mr-2"></i>World Manager</h1>
                <p class="text-slate-400 mt-1">Path: <span class="font-mono bg-slate-800 px-2 py-1 rounded text-sm text-slate-300">{{ .CurrentPath }}</span></p>
            </div>
            <form action="?upload" method="POST" enctype="multipart/form-data" class="flex gap-2 bg-slate-800 p-2 rounded-lg border border-slate-700">
                <input type="file" name="file" class="text-sm file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-sm file:font-semibold file:bg-green-500 file:text-white hover:file:bg-green-600 cursor-pointer">
                <button type="submit" class="bg-blue-600 px-4 py-2 rounded text-sm font-bold hover:bg-blue-500 transition">Upload</button>
            </form>
        </header>

        <div class="bg-slate-800 rounded-xl shadow-2xl border border-slate-700 overflow-hidden">
            <table class="w-full text-left border-collapse">
                <thead>
                    <tr class="bg-slate-700/50 text-slate-300 uppercase text-xs">
                        <th class="px-6 py-4">Name</th>
                        <th class="px-6 py-4 text-right">Size</th>
                        <th class="px-6 py-4 text-center w-32">Actions</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-slate-700">
                    {{ if ne .CurrentPath "/" }}
                    <tr class="hover:bg-slate-700/30 transition">
                        <td class="px-6 py-4" colspan="3">
                            <a href=".." class="text-blue-400 hover:text-blue-300 flex items-center">
                                <i class="fas fa-level-up-alt mr-3"></i> ..
                            </a>
                        </td>
                    </tr>
                    {{ end }}
                    {{ range .Files }}
                    <tr class="hover:bg-slate-700/30 transition group">
                        <td class="px-6 py-4">
                            <a href="{{ if .IsDir }}{{ .Name }}/{{ else }}{{ .Name }}{{ end }}" class="flex items-center {{ if .IsDir }}text-yellow-400 font-semibold{{ else }}text-slate-200{{ end }} hover:underline">
                                <i class="fas {{ if .IsDir }}fa-folder{{ else }}fa-file-code{{ end }} mr-3"></i>
                                {{ .Name }}
                            </a>
                        </td>
                        <td class="px-6 py-4 text-right font-mono text-sm text-slate-400">
                            {{ if .IsDir }}--{{ else }}{{ .Size }} B{{ end }}
                        </td>
                        <td class="px-6 py-4 text-center">
                            <button onclick="deleteFile('{{ .Name }}')" class="text-slate-500 hover:text-red-500 transition px-2">
                                <i class="fas fa-trash-alt"></i>
                            </button>
                        </td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
        </div>
    </div>

    <script>
        async function deleteFile(name) {
            if (!confirm('Are you sure you want to delete ' + name + '?')) return;
            const res = await fetch(window.location.pathname + (window.location.pathname.endsWith('/') ? '' : '/') + name, { method: 'DELETE' });
            if (res.ok) window.location.reload();
            else alert('Failed to delete');
        }
    </script>
</body>
</html>
`

func GetFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	targetPath := filepath.Join(vol, filepath.Clean(r.URL.Path))

	if !strings.HasPrefix(targetPath, vol) {
		http.Error(rw, "Forbidden", http.StatusForbidden)
		return nil
	}

	info, err := os.Stat(targetPath)
	if os.IsNotExist(err) {
		http.NotFound(rw, r)
		return nil
	}

	if info.IsDir() {
		files, err := os.ReadDir(targetPath)
		if err != nil {
			return err
		}

		var fileList []FileInfo
		for _, f := range files {
			finfo, _ := f.Info()
			fileList = append(fileList, FileInfo{
				Name:  f.Name(),
				IsDir: f.IsDir(),
				Size:  finfo.Size(),
			})
		}

		// Detect if requester wants HTML (Browser) or JSON (API)
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			tmpl := template.Must(template.New("ui").Parse(htmlTemplate))
			rw.Header().Set("Content-Type", "text/html")
			return tmpl.Execute(rw, TemplateData{
				CurrentPath: filepath.Clean(r.URL.Path),
				Files:       fileList,
			})
		}

		rw.Header().Set("Content-Type", "application/json")
		return json.NewEncoder(rw).Encode(fileList)
	}

	http.ServeFile(rw, r, targetPath)
	return nil
}

// ... UploadFile and DeleteFile functions remain same as before ...
func UploadFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	r.Body = http.MaxBytesReader(rw, r.Body, MaxFileSize)
	// Handle regular form upload (browser)
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		err := r.ParseMultipartForm(MaxFileSize)
		if err != nil {
			return err
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			return err
		}
		defer file.Close()
		target := filepath.Join(vol, filepath.Clean(r.URL.Path), header.Filename)
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()
		io.Copy(out, file)
		// Redirect back to folder after web upload
		http.Redirect(rw, r, r.URL.Path, http.StatusSeeOther)
		return nil
	}
	return fmt.Errorf("unsupported upload type")
}

func DeleteFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	targetPath := filepath.Join(vol, filepath.Clean(r.URL.Path))
	if targetPath == vol {
		return fmt.Errorf("cannot delete root")
	}
	return os.RemoveAll(targetPath)
}
