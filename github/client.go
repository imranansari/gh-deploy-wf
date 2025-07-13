package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
	// Cache for installation IDs by organization
	installationCache map[string]int64
}

// NewClientFactory creates a new GitHub client factory
func NewClientFactory(cfg config.GitHubConfig, privateKey []byte, logger zerolog.Logger) *ClientFactory {
	return &ClientFactory{
		config: cfg,
		privateKey: privateKey,
		logger: logger,
		installationCache: make(map[string]int64),
	}
}

// CreateClientForOrg creates a GitHub client based on configuration
// This is the main entry point that routes to either Enterprise or GitHub.com
func (f *ClientFactory) CreateClientForOrg(ctx context.Context, org string) (*github.Client, error) {
	if f.config.EnterpriseURL != "" {
		return f.CreateEnterpriseClientForOrg(ctx, org)
	}
	
	// TODO: Remove this GitHub.com fallback when fully migrated to Enterprise
	// START REMOVE WHEN ENTERPRISE-ONLY
	return f.CreateGitHubComClientForOrg(ctx, org)
	// END REMOVE WHEN ENTERPRISE-ONLY
}

// CreateEnterpriseClientForOrg creates a GitHub Enterprise client for a specific organization
func (f *ClientFactory) CreateEnterpriseClientForOrg(ctx context.Context, org string) (*github.Client, error) {
	if f.config.EnterpriseURL == "" {
		return nil, fmt.Errorf("GitHub Enterprise URL not configured")
	}
	
	// Check if we have a cached installation ID for this org
	if installationID, exists := f.installationCache[org]; exists {
		return f.createEnterpriseInstallationClient(installationID)
	}
	
	// Create GitHub App transport to find installations
	atr, err := ghinstallation.NewAppsTransport(
		http.DefaultTransport,
		f.config.AppID,
		f.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create app transport: %w", err)
	}
	
	// Configure Enterprise base URL
	baseURL := strings.TrimSuffix(f.config.EnterpriseURL, "/")
	atr.BaseURL = baseURL + "/api/v3"
	
	// Create temporary client to list installations
	appClient := github.NewClient(&http.Client{Transport: atr})
	appClient.BaseURL, _ = appClient.BaseURL.Parse(baseURL + "/api/v3/")
	appClient.UploadURL, _ = appClient.UploadURL.Parse(baseURL + "/api/uploads/")
	
	// Find installation for the organization
	installations, _, err := appClient.Apps.ListInstallations(ctx, &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("failed to list app installations on Enterprise: %w", err)
	}
	
	var targetInstallationID int64
	for _, installation := range installations {
		if installation.Account.GetLogin() == org {
			targetInstallationID = installation.GetID()
			// Cache the installation ID
			f.installationCache[org] = targetInstallationID
			break
		}
	}
	
	if targetInstallationID == 0 {
		return nil, fmt.Errorf("no installation found for organization '%s' on Enterprise GitHub %s", org, f.config.EnterpriseURL)
	}
	
	f.logger.Info().
		Int64("app_id", f.config.AppID).
		Int64("installation_id", targetInstallationID).
		Str("organization", org).
		Str("enterprise_url", f.config.EnterpriseURL).
		Msg("Found GitHub Enterprise App installation for organization")
	
	return f.createEnterpriseInstallationClient(targetInstallationID)
}

// createEnterpriseInstallationClient creates a client for a specific Enterprise installation ID
func (f *ClientFactory) createEnterpriseInstallationClient(installationID int64) (*github.Client, error) {
	// Create GitHub App installation transport
	itr, err := ghinstallation.New(
		http.DefaultTransport,
		f.config.AppID,
		installationID,
		f.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %w", err)
	}
	
	// Configure Enterprise base URL
	baseURL := strings.TrimSuffix(f.config.EnterpriseURL, "/")
	itr.BaseURL = baseURL + "/api/v3"
	
	// Create the client
	client := github.NewClient(&http.Client{Transport: itr})
	client.BaseURL, _ = client.BaseURL.Parse(baseURL + "/api/v3/")
	client.UploadURL, _ = client.UploadURL.Parse(baseURL + "/api/uploads/")
	
	f.logger.Info().
		Int64("app_id", f.config.AppID).
		Int64("installation_id", installationID).
		Str("enterprise_url", f.config.EnterpriseURL).
		Msg("GitHub Enterprise installation client created successfully")
	
	return client, nil
}

// TODO: REMOVE ALL CODE BELOW THIS LINE WHEN FULLY MIGRATED TO ENTERPRISE
// ============================================================================
// START REMOVE WHEN ENTERPRISE-ONLY

// CreateGitHubComClientForOrg creates a GitHub.com client for a specific organization
// DEPRECATED: This function will be removed when fully migrated to Enterprise
func (f *ClientFactory) CreateGitHubComClientForOrg(ctx context.Context, org string) (*github.Client, error) {
	// Check if we have a cached installation ID for this org
	if installationID, exists := f.installationCache[org]; exists {
		return f.createGitHubComInstallationClient(installationID)
	}
	
	// Create GitHub App transport to find installations
	atr, err := ghinstallation.NewAppsTransport(
		http.DefaultTransport,
		f.config.AppID,
		f.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create app transport: %w", err)
	}
	
	// Create temporary client to list installations
	appClient := github.NewClient(&http.Client{Transport: atr})
	
	// Find installation for the organization
	installations, _, err := appClient.Apps.ListInstallations(ctx, &github.ListOptions{PerPage: 100})
	if err != nil {
		return nil, fmt.Errorf("failed to list app installations on GitHub.com: %w", err)
	}
	
	var targetInstallationID int64
	for _, installation := range installations {
		if installation.Account.GetLogin() == org {
			targetInstallationID = installation.GetID()
			// Cache the installation ID
			f.installationCache[org] = targetInstallationID
			break
		}
	}
	
	if targetInstallationID == 0 {
		return nil, fmt.Errorf("no installation found for organization '%s' on GitHub.com", org)
	}
	
	f.logger.Info().
		Int64("app_id", f.config.AppID).
		Int64("installation_id", targetInstallationID).
		Str("organization", org).
		Str("github_type", "github.com").
		Msg("Found GitHub.com App installation for organization")
	
	return f.createGitHubComInstallationClient(targetInstallationID)
}

// createGitHubComInstallationClient creates a client for a specific GitHub.com installation ID
// DEPRECATED: This function will be removed when fully migrated to Enterprise
func (f *ClientFactory) createGitHubComInstallationClient(installationID int64) (*github.Client, error) {
	// Create GitHub App installation transport
	itr, err := ghinstallation.New(
		http.DefaultTransport,
		f.config.AppID,
		installationID,
		f.privateKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %w", err)
	}
	
	// Create the client (uses default GitHub.com URLs)
	client := github.NewClient(&http.Client{Transport: itr})
	
	f.logger.Info().
		Int64("app_id", f.config.AppID).
		Int64("installation_id", installationID).
		Str("github_type", "github.com").
		Msg("GitHub.com installation client created successfully")
	
	return client, nil
}

// END REMOVE WHEN ENTERPRISE-ONLY
// ============================================================================

// CreateClient creates a new authenticated GitHub client (DEPRECATED - use CreateClientForOrg)
func (f *ClientFactory) CreateClient(ctx context.Context) (*github.Client, error) {
	return nil, fmt.Errorf("CreateClient is deprecated - use CreateClientForOrg instead")
}