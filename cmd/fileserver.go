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
	Long:  "Pod file server for viewing and editing minecraft world data and config files in the minecraft server's data directory",
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
	cfg := config.NewFileServerConfig()
	vol := cfg.GetVolume()

	// Get port from environment or default to 8081
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	http.Handle("/", http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		// All functions now take 'vol' as an argument to look in the right place
		switch r.Method {
		case http.MethodGet:
			if err := fileserver.GetFile(rw, r, vol); err != nil {
				logger.Error("error getting file", zap.Error(err))
			}

		case http.MethodPost, http.MethodPut:
			// POST and PUT both use the same Upload logic
			// (handles both new files and overwriting existing ones)
			if err := fileserver.UploadFile(rw, r, vol); err != nil {
				logger.Error("error uploading/editing file", zap.Error(err))
			}

		case http.MethodDelete:
			if err := fileserver.DeleteFile(rw, r, vol); err != nil {
				logger.Error("error deleting file", zap.Error(err))
			}

		default:
			http.Error(rw, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))

	logger.Info("starting server on :"+port, zap.String("volume", vol))
	return http.ListenAndServe(":"+port, nil)
}
