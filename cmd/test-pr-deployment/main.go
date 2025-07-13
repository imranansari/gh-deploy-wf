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
		owner  = flag.String("owner", os.Getenv("TEST_REPO_OWNER"), "Repository owner")
		repo   = flag.String("repo", os.Getenv("TEST_REPO_NAME"), "Repository name")
		prNum  = flag.Int("pr", 0, "Pull request number")
		listPRs = flag.Bool("list", false, "List open pull requests")
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

	// If list flag is set, show open PRs
	if *listPRs {
		listOpenPRs(ctx, client, *owner, *repo)
		return
	}

	// If no PR number provided, list PRs and exit
	if *prNum == 0 {
		log.Println("No PR number provided. Here are the open PRs:")
		listOpenPRs(ctx, client, *owner, *repo)
		log.Println("\nRun with -pr=<number> to deploy a specific PR")
		return
	}

	// Get PR details
	log.Printf("Getting PR #%d details...\n", *prNum)
	pr, _, err := client.PullRequests.Get(ctx, *owner, *repo, *prNum)
	if err != nil {
		log.Fatalf("Failed to get PR: %v", err)
	}

	log.Printf("✓ PR #%d: %s\n", pr.GetNumber(), pr.GetTitle())
	log.Printf("  Author: %s\n", pr.GetUser().GetLogin())
	log.Printf("  Branch: %s → %s\n", pr.GetHead().GetRef(), pr.GetBase().GetRef())
	log.Printf("  Commit: %s\n", pr.GetHead().GetSHA()[:8])

	// Simulate different deployment scenarios
	scenarios := []struct {
		environment string
		delay       time.Duration
		statuses    []deploymentStatus
	}{
		{
			environment: "preview",
			delay:       1 * time.Second,
			statuses: []deploymentStatus{
				{state: "queued", description: "Preview deployment queued", delay: 2 * time.Second},
				{state: "in_progress", description: "Building preview environment", delay: 3 * time.Second},
				{state: "success", description: "Preview ready", environmentURL: fmt.Sprintf("https://pr-%d.preview.example.com", *prNum)},
			},
		},
		{
			environment: "staging",
			delay:       5 * time.Second,
			statuses: []deploymentStatus{
				{state: "queued", description: "Staging deployment queued", delay: 2 * time.Second},
				{state: "in_progress", description: "Deploying to staging", delay: 4 * time.Second},
				{state: "success", description: "Staging deployment complete", environmentURL: "https://staging.example.com"},
			},
		},
	}

	log.Println("\n=== Starting PR Deployment Simulation ===")
	log.Printf("This will create deployments for PR #%d and show how they appear in the PR\n", *prNum)

	for _, scenario := range scenarios {
		log.Printf("\n--- Deploying to %s environment ---\n", scenario.environment)
		
		// Create deployment
		deployment, err := createDeployment(ctx, client, *owner, *repo, pr.GetHead().GetSHA(), scenario.environment, 
			fmt.Sprintf("Deploy PR #%d to %s", *prNum, scenario.environment))
		if err != nil {
			log.Printf("Failed to create %s deployment: %v\n", scenario.environment, err)
			continue
		}
		
		log.Printf("✓ Created %s deployment (ID: %d)\n", scenario.environment, deployment.GetID())

		// Update deployment status through stages
		for _, status := range scenario.statuses {
			time.Sleep(status.delay)
			log.Printf("  → Status: %s - %s\n", status.state, status.description)
			
			err := updateDeploymentStatus(ctx, client, *owner, *repo, deployment.GetID(), 
				status.state, status.description, status.environmentURL, *prNum)
			if err != nil {
				log.Printf("    Failed to update status: %v\n", err)
			}
		}

		if scenario.delay > 0 {
			log.Printf("Waiting %v before next environment...\n", scenario.delay)
			time.Sleep(scenario.delay)
		}
	}

	// Final summary
	log.Println("\n=== Deployment Summary ===")
	log.Printf("✓ Created deployments for PR #%d\n", *prNum)
	log.Printf("✓ View PR: https://github.com/%s/%s/pull/%d\n", *owner, *repo, *prNum)
	log.Printf("✓ View deployments: https://github.com/%s/%s/deployments\n", *owner, *repo)
	log.Println("\nCheck the PR page to see deployment status indicators!")
}

type deploymentStatus struct {
	state          string
	description    string
	environmentURL string
	delay          time.Duration
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

func listOpenPRs(ctx context.Context, client *github.Client, owner, repo string) {
	opts := &github.PullRequestListOptions{
		State: "open",
		ListOptions: github.ListOptions{
			PerPage: 10,
		},
	}

	prs, _, err := client.PullRequests.List(ctx, owner, repo, opts)
	if err != nil {
		log.Fatalf("Failed to list PRs: %v", err)
	}

	if len(prs) == 0 {
		log.Println("No open pull requests found")
		return
	}

	log.Printf("Open pull requests in %s/%s:\n", owner, repo)
	for _, pr := range prs {
		log.Printf("  PR #%d: %s (by %s)\n", pr.GetNumber(), pr.GetTitle(), pr.GetUser().GetLogin())
		log.Printf("    Branch: %s → %s\n", pr.GetHead().GetRef(), pr.GetBase().GetRef())
	}
}

func createDeployment(ctx context.Context, client *github.Client, owner, repo, ref, environment, description string) (*github.Deployment, error) {
	req := &github.DeploymentRequest{
		Ref:                   github.String(ref),
		Task:                  github.String("deploy"),
		AutoMerge:             github.Bool(false),
		RequiredContexts:      &[]string{}, // Bypass status checks
		Environment:           github.String(environment),
		Description:           github.String(description),
		TransientEnvironment:  github.Bool(environment == "preview"),
		ProductionEnvironment: github.Bool(environment == "production"),
	}

	deployment, _, err := client.Repositories.CreateDeployment(ctx, owner, repo, req)
	return deployment, err
}

func updateDeploymentStatus(ctx context.Context, client *github.Client, owner, repo string, 
	deploymentID int64, state, description, environmentURL string, prNum int) error {
	
	req := &github.DeploymentStatusRequest{
		State:        github.String(state),
		Description:  github.String(description),
		AutoInactive: github.Bool(true),
	}

	// Add URLs for better visibility
	if state == "in_progress" || state == "success" {
		req.LogURL = github.String(fmt.Sprintf("https://example.com/logs/pr-%d/%d", prNum, deploymentID))
	}

	if environmentURL != "" && state == "success" {
		req.EnvironmentURL = github.String(environmentURL)
	}

	_, _, err := client.Repositories.CreateDeploymentStatus(ctx, owner, repo, deploymentID, req)
	return err
}