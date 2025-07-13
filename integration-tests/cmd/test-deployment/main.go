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
		log.Fatal("Repository owner and name are required")
	}

	// Create GitHub client
	client, err := createGitHubClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx := context.Background()

	// Verify repository access
	log.Printf("Verifying access to %s/%s...\n", *owner, *repo)
	repoInfo, _, err := client.Repositories.Get(ctx, *owner, *repo)
	if err != nil {
		log.Fatalf("Failed to access repository: %v", err)
	}
	log.Printf("✓ Repository found: %s\n", repoInfo.GetFullName())

	// Get the latest commit SHA if using branch name
	commitSHA := *ref
	if *ref == "main" || *ref == "master" || *ref == repoInfo.GetDefaultBranch() {
		branch, _, err := client.Repositories.GetBranch(ctx, *owner, *repo, *ref, 0)
		if err != nil {
			log.Fatalf("Failed to get branch: %v", err)
		}
		commitSHA = branch.GetCommit().GetSHA()
		log.Printf("✓ Using commit: %s\n", commitSHA[:8])
	}

	// Create deployment
	log.Println("\n=== Creating Deployment ===")
	deployment, err := createDeployment(ctx, client, *owner, *repo, commitSHA, *environment, *description)
	if err != nil {
		log.Fatalf("Failed to create deployment: %v", err)
	}
	
	log.Printf("✓ Deployment created!")
	log.Printf("  ID: %d", deployment.GetID())
	log.Printf("  Environment: %s", deployment.GetEnvironment())
	log.Printf("  URL: https://github.com/%s/%s/deployments", *owner, *repo)

	// Update deployment status
	log.Println("\n=== Updating Deployment Status ===")
	
	// Set to "queued"
	log.Println("1. Setting status to: queued")
	if err := updateStatus(ctx, client, *owner, *repo, deployment.GetID(), "queued", "Deployment queued"); err != nil {
		log.Printf("Warning: Failed to set queued status: %v", err)
	} else {
		log.Println("✓ Status: queued")
	}
	time.Sleep(2 * time.Second)

	// Set to "in_progress"
	log.Println("2. Setting status to: in_progress")
	if err := updateStatus(ctx, client, *owner, *repo, deployment.GetID(), "in_progress", "Deployment in progress"); err != nil {
		log.Printf("Warning: Failed to set in_progress status: %v", err)
	} else {
		log.Println("✓ Status: in_progress")
	}
	time.Sleep(3 * time.Second)

	// Set to "success"
	log.Println("3. Setting status to: success")
	if err := updateStatus(ctx, client, *owner, *repo, deployment.GetID(), "success", "Deployment completed successfully"); err != nil {
		log.Printf("Warning: Failed to set success status: %v", err)
	} else {
		log.Println("✓ Status: success")
	}

	// Summary
	log.Println("\n=== Summary ===")
	log.Printf("✓ Successfully created and updated deployment %d", deployment.GetID())
	log.Printf("✓ View deployment at: https://github.com/%s/%s/deployments", *owner, *repo)
	log.Printf("✓ View deployment activity: https://github.com/%s/%s/deployments/activity_log?environment=%s", 
		*owner, *repo, *environment)
}

func createGitHubClient() (*github.Client, error) {
	appID, err := strconv.ParseInt(os.Getenv("GITHUB_APP_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_APP_ID: %v", err)
	}

	installationID, err := strconv.ParseInt(os.Getenv("GITHUB_INSTALLATION_ID"), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GITHUB_INSTALLATION_ID: %v", err)
	}

	privateKey, err := os.ReadFile(os.Getenv("GITHUB_PRIVATE_KEY_PATH"))
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}

	itr, err := ghinstallation.New(http.DefaultTransport, appID, installationID, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create installation transport: %v", err)
	}

	return github.NewClient(&http.Client{Transport: itr}), nil
}

func createDeployment(ctx context.Context, client *github.Client, owner, repo, ref, environment, description string) (*github.Deployment, error) {
	req := &github.DeploymentRequest{
		Ref:                   github.String(ref),
		Task:                  github.String("deploy"),
		AutoMerge:             github.Bool(false),
		RequiredContexts:      &[]string{}, // Bypass status checks
		Environment:           github.String(environment),
		Description:           github.String(description),
		TransientEnvironment:  github.Bool(false),
		ProductionEnvironment: github.Bool(environment == "production"),
	}

	deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repo, req)
	return deployment, err
}

func updateStatus(ctx context.Context, client *github.Client, owner, repo string, deploymentID int64, state, description string) error {
	req := &github.DeploymentStatusRequest{
		State:        github.String(state),
		Description:  github.String(description),
		AutoInactive: github.Bool(false), // Don't auto-inactivate for this test
	}

	// Add log URL for in_progress and success states
	if state == "in_progress" || state == "success" {
		req.LogURL = github.String(fmt.Sprintf("https://example.com/logs/%d", deploymentID))
	}

	// Add environment URL for success state
	if state == "success" {
		req.EnvironmentURL = github.String(fmt.Sprintf("https://%s.example.com", os.Getenv("TEST_REPO_NAME")))
	}

	_, _, err := client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deploymentID, req)
	return err
}