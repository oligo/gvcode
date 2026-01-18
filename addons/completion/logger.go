package completion

import (
	"log/slog"
	"os"
)

const (
	logGroup = "completion"
)

var logger *slog.Logger

func init() {
	// Configure the handler options to set the minimum log level to Debug
	handlerOptions := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	// Create a new logger with the specified options
	// Use NewJSONHandler or NewTextHandler as needed
	logger = slog.New(slog.NewTextHandler(os.Stdout, handlerOptions)).WithGroup(logGroup)

}

func SetLogger(log *slog.Logger) {
	logger = log.WithGroup(logGroup)
}
