package cmd

import (
	"net/http"
	"os"

	"github.com/raefon/agones-mc/internal/config"
	"github.com/raefon/agones-mc/pkg/fileserver"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var fileServerCmd = cobra.Command{
	Use:   "fileserver",
	Short: "Minecraft GameServer pod file server",
	Long:  "A web-based file manager for viewing, editing, and managing Minecraft world data inside the Agones GameServer pod.",
	Run: func(cmd *cobra.Command, args []string) {
		if err := Run(); err != nil {
			logger.Fatal("file server error", zap.Error(err))
		}
	},
}

func init() {
	RootCmd.AddCommand(&fileServerCmd)
}

func Run() error {
	// 1. Load Configuration (Volume path, etc.)
	cfg := config.NewFileServerConfig()
	vol := cfg.GetVolume() // Usually "/data"

	// 2. Resolve Port
	// We default to 8081 because Agones Sidecar uses 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	// 3. Define the Request Handler
	http.HandleFunc("/", func(rw http.ResponseWriter, r *http.Request) {
		var err error

		switch r.Method {
		case http.MethodGet:
			// List directory (JSON or HTML UI) or download file
			err = fileserver.GetFile(rw, r, vol)

		case http.MethodPost, http.MethodPut:
			// Handles Uploads, Editing existing files, and ZIP extraction
			err = fileserver.UploadFile(rw, r, vol)

		case "MKCOL":
			// Custom method for Creating Folders (Directory Creation)
			err = fileserver.UploadFile(rw, r, vol)

		case http.MethodDelete:
			// Remove files or folders
			err = fileserver.DeleteFile(rw, r, vol)

		default:
			http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		}

		// Log any errors that occurred during the request
		if err != nil {
			logger.Error("request error",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Error(err),
			)
		}
	})

	// 4. Start the Server
	logger.Info("starting web file manager",
		zap.String("port", port),
		zap.String("volume", vol),
	)

	return http.ListenAndServe(":"+port, nil)
}
