package logging

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// InitLogger initializes zerolog with the specified configuration
func InitLogger(level string, format string) {
	// Set time format
	zerolog.TimeFieldFormat = time.RFC3339Nano
	
	// Parse log level
	logLevel, err := zerolog.ParseLevel(level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)
	
	// Configure output format
	if format == "console" {
		log.Logger = log.Output(zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		})
	} else {
		// JSON format (default)
		log.Logger = zerolog.New(os.Stdout).With().
			Timestamp().
			Caller().
			Logger()
	}
	
	// Add service metadata
	log.Logger = log.With().
		Str("service", "github-deployment-tracker").
		Logger()
}

// WorkflowLogger creates a logger for Temporal workflows
func WorkflowLogger(workflowID string, runID string) zerolog.Logger {
	return log.With().
		Str("workflow_id", workflowID).
		Str("run_id", runID).
		Str("component", "workflow").
		Logger()
}

// ActivityLogger creates a logger for Temporal activities
func ActivityLogger(activityName string, workflowID string, runID string) zerolog.Logger {
	return log.With().
		Str("activity", activityName).
		Str("workflow_id", workflowID).
		Str("run_id", runID).
		Str("component", "activity").
		Logger()
}

// GitHubLogger creates a logger for GitHub API operations
func GitHubLogger() zerolog.Logger {
	return log.With().
		Str("component", "github").
		Logger()
}