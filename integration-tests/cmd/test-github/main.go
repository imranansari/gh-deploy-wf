package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v58/github"
	"github.com/joho/godotenv"
)

func main() {
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	// Parse command line flags
	var (
		owner       = flag.String("owner", os.Getenv("TEST_REPO_OWNER"), "Repository owner")
		repo        = flag.String("repo", os.Getenv("TEST_REPO_NAME"), "Repository name")
		ref         = flag.String("ref", "main", "Git ref (branch, tag, or SHA)")
		environment = flag.String("env", "staging", "Deployment environment")
		description = flag.String("desc", "Test deployment from Go script", "Deployment description")
	)
	flag.Parse()

	// Validate required flags
	if *owner == "" || *repo == "" {
		log.Fatal("Repository owner and name are required. Set TEST_REPO_OWNER and TEST_REPO_NAME in .env or use -owner and -repo flags")
	}

	// Create GitHub client
	client, err := createGitHubClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx := context.Background()

	// Test authentication
	log.Println("Testing GitHub App authentication...")
	app, _, err := client.Apps.Get(ctx, "")
	if err != nil {
		log.Fatalf("Failed to authenticate: %v", err)
	}
	log.Printf("✓ Authenticated as GitHub App: %s (ID: %d)\n", app.GetName(), app.GetID())

	// Get repository to verify access
	log.Printf("Checking access to repository %s/%s...\n", *owner, *repo)
	repoInfo, _, err := client.Repositories.Get(ctx, *owner, *repo)
	if err != nil {
		log.Fatalf("Failed to access repository: %v", err)
	}
	log.Printf("✓ Repository found: %s (default branch: %s)\n", repoInfo.GetFullName(), repoInfo.GetDefaultBranch())

	// If ref is "main" or "master", get the latest commit SHA
	commitSHA := *ref
	if *ref == "main" || *ref == "master" || *ref == repoInfo.GetDefaultBranch() {
		log.Printf("Getting latest commit SHA for branch %s...\n", *ref)
		branch, _, err := client.Repositories.GetBranch(ctx, *owner, *repo, *ref, 0)
		if err != nil {
			log.Fatalf("Failed to get branch: %v", err)
		}
		commitSHA = branch.GetCommit().GetSHA()
		log.Printf("✓ Latest commit: %s\n", commitSHA[:8])
	}

	// Create deployment
	log.Println("\n=== Creating Deployment ===")
	deployment, err := createDeployment(ctx, client, *owner, *repo, commitSHA, *environment, *description)
	if err != nil {
		log.Fatalf("Failed to create deployment: %v", err)
	}
	log.Printf("✓ Deployment created successfully!")
	log.Printf("  ID: %d", deployment.GetID())
	log.Printf("  URL: %s", deployment.GetURL())
	log.Printf("  Environment: %s", deployment.GetEnvironment())
	log.Printf("  Created at: %s", deployment.GetCreatedAt().Format(time.RFC3339))

	// Wait a moment before updating status
	log.Println("\nWaiting 2 seconds before updating status...")
	time.Sleep(2 * time.Second)

	// Update deployment status to "in_progress"
	log.Println("\n=== Updating Deployment Status ===")
	log.Println("Setting status to: in_progress")
	status1, err := updateDeploymentStatus(ctx, client, *owner, *repo, deployment.GetID(), 
		"in_progress", "Deployment started", "")
	if err != nil {
		log.Fatalf("Failed to update deployment status: %v", err)
	}
	log.Printf("✓ Status updated to: %s", status1.GetState())

	// Wait and update to success
	log.Println("\nWaiting 3 seconds before marking as success...")
	time.Sleep(3 * time.Second)

	log.Println("Setting status to: success")
	status2, err := updateDeploymentStatus(ctx, client, *owner, *repo, deployment.GetID(), 
		"success", "Deployment completed successfully", "https://example.com")
	if err != nil {
		log.Fatalf("Failed to update deployment status: %v", err)
	}
	log.Printf("✓ Status updated to: %s", status2.GetState())

	// Print summary
	log.Println("\n=== Summary ===")
	log.Printf("✓ Successfully created and updated deployment %d", deployment.GetID())
	log.Printf("✓ View deployment at: https://github.com/%s/%s/deployments", *owner, *repo)
	log.Printf("✓ View specific deployment: https://github.com/%s/%s/deployments/activity_log?environment=%s", 
		*owner, *repo, *environment)
}

func createGitHubClient() (*github.Client, error) {
	// Get configuration from environment
	appIDStr := os.Getenv("GITHUB_APP_ID")
	if appIDStr == "" {
		return nil, fmt.Errorf("GITHUB_APP_ID not set")
	}
	
	appID, err := strconv.ParseInt(appIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %v", err)
	}

	installationIDStr := os.Getenv("GITHUB_INSTALLATION_ID")
	if installationIDStr == "" {
		return nil, fmt.Errorf("GITHUB_INSTALLATION_ID not set")
	}
	
	installationID, err := strconv.ParseInt(installationIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_INSTALLATION_ID: %v", err)
	}

	privateKeyPath := os.Getenv("GITHUB_PRIVATE_KEY_PATH")
	if privateKeyPath == "" {
		return nil, fmt.Errorf("GITHUB_PRIVATE_KEY_PATH not set")
	}

	privateKey, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}

	// Create installation transport
	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %v", err)
	}

	// Create GitHub client
	client := github.NewClient(&http.Client{Transport: itr})
	return client, nil
}

func createDeployment(ctx context.Context, client *github.Client, owner, repo, ref, environment, description string) (*github.Deployment, error) {
	deploymentReq := &github.DeploymentRequest{
		Ref:                   github.String(ref),
		Task:                  github.String("deploy"),
		AutoMerge:             github.Bool(false),
		RequiredContexts:      &[]string{}, // Empty array to bypass status checks
		Environment:           github.String(environment),
		Description:           github.String(description),
		TransientEnvironment:  github.Bool(false),
		ProductionEnvironment: github.Bool(environment == "production"),
	}

	deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repo, deploymentReq)
	return deployment, err
}

func updateDeploymentStatus(ctx context.Context, client *github.Client, owner, repo string, deploymentID int64, state, description, environmentURL string) (*github.DeploymentStatus, error) {
	statusReq := &github.DeploymentStatusRequest{
		State:          github.String(state),
		Description:    github.String(description),
		AutoInactive:   github.Bool(true),
	}

	if environmentURL != "" {
		statusReq.EnvironmentURL = github.String(environmentURL)
	}

	status, _, err := client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deploymentID, statusReq)
	return status, err
}