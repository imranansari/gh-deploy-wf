package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v58/github"
	"github.com/rs/zerolog"

	"github.com/imranansari/gh-deploy-wf/config"
)

// ClientFactory creates authenticated GitHub clients
type ClientFactory struct {
	config config.GitHubConfig
	privateKey []byte
	logger zerolog.Logger
}

// NewClientFactory creates a new GitHub client factory
func NewClientFactory(cfg config.GitHubConfig, privateKey []byte, logger zerolog.Logger) *ClientFactory {
	return &ClientFactory{
		config: cfg,
		privateKey: privateKey,
		logger: logger,
	}
}

// CreateClient creates a new authenticated GitHub client
func (f *ClientFactory) CreateClient(ctx context.Context) (*github.Client, error) {
	// Create GitHub App installation transport
	itr, err := ghinstallation.New(
		http.DefaultTransport,
		f.config.AppID,
		f.config.InstallationID,
		f.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %w", err)
	}
	
	// Configure base URL for enterprise if needed
	if f.config.Enterprise && f.config.BaseURL != "" {
		itr.BaseURL = f.config.BaseURL
	}
	
	// Create the client
	client := github.NewClient(&http.Client{Transport: itr})
	
	// Set custom URLs if enterprise
	if f.config.Enterprise {
		if f.config.BaseURL != "" {
			client.BaseURL, _ = client.BaseURL.Parse(f.config.BaseURL + "/")
		}
		if f.config.UploadURL != "" {
			client.UploadURL, _ = client.UploadURL.Parse(f.config.UploadURL + "/")
		}
	}
	
	f.logger.Info().
		Int64("app_id", f.config.AppID).
		Int64("installation_id", f.config.InstallationID).
		Str("base_url", f.config.BaseURL).
		Bool("enterprise", f.config.Enterprise).
		Msg("GitHub client created successfully")
	
	return client, nil
}