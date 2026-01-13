package fileserver

import (
	"archive/zip"
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const MaxFileSize int64 = 100 << 20

type TemplateData struct {
	CurrentPath string
	Files       []FileInfo
	EditFile    *EditData
}

type EditData struct {
	Name    string
	Content string
}

type FileInfo struct {
	Name  string `json:"name"`
	IsDir bool   `json:"isDir"`
	Size  int64  `json:"size"`
	Ext   string `json:"ext"`
}

const htmlTemplate = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>MC Manager - {{ .CurrentPath }}</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css">
    <link rel="stylesheet" data-name="vs/editor/editor.main" href="https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.33.0/min/vs/editor/editor.main.min.css">
</head>
<body class="bg-[#0d1117] text-slate-200 font-sans">
    <div class="max-w-6xl mx-auto py-8 px-4">
        <!-- Header -->
        <div class="flex flex-wrap justify-between items-end gap-4 mb-6">
            <div>
                <h1 class="text-2xl font-bold text-green-400 flex items-center gap-2">
                    <i class="fas fa-server"></i> Minecraft File Manager
                </h1>
                <p class="text-slate-500 font-mono text-sm mt-1">/data{{ .CurrentPath }}</p>
            </div>
            <div class="flex gap-2">
                <button onclick="promptMkdir()" class="bg-slate-700 hover:bg-slate-600 px-4 py-2 rounded text-sm font-semibold">
                    <i class="fas fa-folder-plus mr-2"></i>New Folder
                </button>
                <form action="?upload" method="POST" enctype="multipart/form-data" class="flex bg-slate-800 rounded border border-slate-700">
                    <input type="file" name="file" class="text-xs p-1">
                    <button type="submit" class="bg-green-600 hover:bg-green-500 px-3 py-1 text-sm font-bold">Upload</button>
                </form>
            </div>
        </div>

        <!-- Explorer -->
        <div class="bg-[#161b22] rounded-lg border border-slate-700 shadow-xl overflow-hidden">
            <table class="w-full text-left">
                <thead class="bg-slate-800/50 text-slate-400 text-xs uppercase">
                    <tr>
                        <th class="px-6 py-3">Name</th>
                        <th class="px-6 py-3 text-right">Size</th>
                        <th class="px-6 py-3 text-right">Actions</th>
                    </tr>
                </thead>
                <tbody class="divide-y divide-slate-800">
                    {{ if ne .CurrentPath "/" }}
                    <tr class="hover:bg-slate-800/50 cursor-pointer" onclick="window.location.href='..'">
                        <td class="px-6 py-3 text-blue-400"><i class="fas fa-arrow-left mr-2"></i> ..</td>
                        <td colspan="2"></td>
                    </tr>
                    {{ end }}
                    {{ range .Files }}
                    <tr class="hover:bg-slate-800/50 group">
                        <td class="px-6 py-3">
                            <a href="{{ if .IsDir }}{{ .Name }}/{{ else }}{{ .Name }}{{ end }}" class="flex items-center {{ if .IsDir }}text-yellow-500{{ else }}text-slate-300{{ end }}">
                                <i class="fas {{ if .IsDir }}fa-folder{{ else }}fa-file-alt{{ end }} mr-3"></i>
                                {{ .Name }}
                            </a>
                        </td>
                        <td class="px-6 py-3 text-right text-xs text-slate-500 font-mono">
                            {{ if .IsDir }}--{{ else }}{{ .Size }} B{{ end }}
                        </td>
                        <td class="px-6 py-3 text-right space-x-3">
                            {{ if eq .Ext ".zip" }}
                            <button onclick="extractZip('{{ .Name }}')" class="text-orange-400 hover:text-orange-300 text-sm" title="Extract">
                                <i class="fas fa-file-archive"></i>
                            </button>
                            {{ end }}
                            {{ if not .IsDir }}
                            <a href="?edit={{ .Name }}" class="text-blue-400 hover:text-blue-300 text-sm" title="Edit">
                                <i class="fas fa-edit"></i>
                            </a>
                            {{ end }}
                            <button onclick="deleteItem('{{ .Name }}')" class="text-slate-600 hover:text-red-500 text-sm">
                                <i class="fas fa-trash"></i>
                            </button>
                        </td>
                    </tr>
                    {{ end }}
                </tbody>
            </table>
        </div>
    </div>

    <!-- Editor Modal -->
    {{ if .EditFile }}
    <div class="fixed inset-0 bg-black/80 z-50 flex items-center justify-center p-4">
        <div class="bg-[#0d1117] w-full h-full max-w-5xl rounded-xl border border-slate-700 flex flex-col">
            <div class="p-4 border-b border-slate-700 flex justify-between items-center">
                <h3 class="font-bold text-slate-300">Editing: {{ .EditFile.Name }}</h3>
                <div class="flex gap-2">
                    <button onclick="saveFile()" class="bg-blue-600 hover:bg-blue-500 px-4 py-1 rounded text-sm font-bold">Save</button>
                    <button onclick="window.location.href=window.location.pathname" class="bg-slate-700 px-4 py-1 rounded text-sm">Cancel</button>
                </div>
            </div>
            <div id="editor-container" class="flex-grow"></div>
        </div>
    </div>
    <script src="https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.33.0/min/vs/loader.min.js"></script>
    <script>
        require.config({ paths: { 'vs': 'https://cdnjs.cloudflare.com/ajax/libs/monaco-editor/0.33.0/min/vs' }});
        require(['vs/editor/editor.main'], function() {
            window.editor = monaco.editor.create(document.getElementById('editor-container'), {
                value: {{ .EditFile.Content }},
                language: 'javascript', // or detect based on extension
                theme: 'vs-dark',
                automaticLayout: true
            });
        });
        async function saveFile() {
            const content = window.editor.getValue();
            const res = await fetch(window.location.pathname + "?edit={{ .EditFile.Name }}", {
                method: 'POST',
                body: content
            });
            if (res.ok) window.location.href = window.location.pathname;
            else alert('Save failed');
        }
    </script>
    {{ end }}

    <script>
        async function deleteItem(name) {
            if (confirm('Delete ' + name + '?')) {
                await fetch(window.location.pathname + (window.location.pathname.endsWith('/') ? '' : '/') + name, { method: 'DELETE' });
                location.reload();
            }
        }
        async function extractZip(name) {
            await fetch(window.location.pathname + (window.location.pathname.endsWith('/') ? '' : '/') + name + "?extract=true", { method: 'POST' });
            location.reload();
        }
        async function promptMkdir() {
            const name = prompt('New folder name:');
            if (name) {
                await fetch(window.location.pathname + (window.location.pathname.endsWith('/') ? '' : '/') + name, { method: 'MKCOL' });
                location.reload();
            }
        }
    </script>
</body>
</html>
`

func GetFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	targetPath := filepath.Join(vol, filepath.Clean(r.URL.Path))
	editName := r.URL.Query().Get("edit")

	// 1. Handle File Editing
	if editName != "" && strings.Contains(r.Header.Get("Accept"), "text/html") {
		content, _ := os.ReadFile(filepath.Join(targetPath, editName))
		files := getFiles(targetPath)
		tmpl := template.Must(template.New("ui").Parse(htmlTemplate))
		return tmpl.Execute(rw, TemplateData{
			CurrentPath: filepath.Clean(r.URL.Path),
			Files:       files,
			EditFile:    &EditData{Name: editName, Content: string(content)},
		})
	}

	// 2. Standard Directory Listing
	info, _ := os.Stat(targetPath)
	if info != nil && info.IsDir() {
		files := getFiles(targetPath)
		if strings.Contains(r.Header.Get("Accept"), "text/html") {
			tmpl := template.Must(template.New("ui").Parse(htmlTemplate))
			return tmpl.Execute(rw, TemplateData{CurrentPath: filepath.Clean(r.URL.Path), Files: files})
		}
		return json.NewEncoder(rw).Encode(files)
	}

	http.ServeFile(rw, r, targetPath)
	return nil
}

func getFiles(p string) []FileInfo {
	entries, _ := os.ReadDir(p)
	var list []FileInfo
	for _, e := range entries {
		info, _ := e.Info()
		list = append(list, FileInfo{
			Name:  e.Name(),
			IsDir: e.IsDir(),
			Size:  info.Size(),
			Ext:   filepath.Ext(e.Name()),
		})
	}
	return list
}

func UploadFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	targetPath := filepath.Join(vol, filepath.Clean(r.URL.Path))

	// Handle Folder Creation (Custom MKCOL method)
	if r.Method == "MKCOL" {
		return os.MkdirAll(targetPath, 0755)
	}

	// Handle Zip Extraction
	if r.URL.Query().Get("extract") == "true" {
		return unzip(targetPath, filepath.Dir(targetPath))
	}

	// Handle File Save (from Editor)
	if r.URL.Query().Get("edit") != "" {
		content, _ := io.ReadAll(r.Body)
		return os.WriteFile(filepath.Join(targetPath, r.URL.Query().Get("edit")), content, 0644)
	}

	// Handle Normal Multipart Upload
	if strings.Contains(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.ParseMultipartForm(MaxFileSize)
		file, header, _ := r.FormFile("file")
		defer file.Close()
		out, _ := os.Create(filepath.Join(targetPath, header.Filename))
		defer out.Close()
		io.Copy(out, file)
		http.Redirect(rw, r, r.URL.Path, http.StatusSeeOther)
	}
	return nil
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}
		os.MkdirAll(filepath.Dir(fpath), os.ModePerm)
		outFile, _ := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		rc, _ := f.Open()
		io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()
	}
	return nil
}

func DeleteFile(rw http.ResponseWriter, r *http.Request, vol string) error {
	return os.RemoveAll(filepath.Join(vol, filepath.Clean(r.URL.Path)))
}
