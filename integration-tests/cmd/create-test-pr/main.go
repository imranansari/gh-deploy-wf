package main

import (
	"context"
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

	client, err := createGitHubClient()
	if err != nil {
		log.Fatalf("Failed to create GitHub client: %v", err)
	}

	ctx := context.Background()
	owner := os.Getenv("TEST_REPO_OWNER")
	repo := os.Getenv("TEST_REPO_NAME")

	// Get default branch
	repoInfo, _, err := client.Repositories.Get(ctx, owner, repo)
	if err != nil {
		log.Fatalf("Failed to get repository: %v", err)
	}
	defaultBranch := repoInfo.GetDefaultBranch()

	// Get the latest commit on default branch
	ref, _, err := client.Git.GetRef(ctx, owner, repo, "refs/heads/"+defaultBranch)
	if err != nil {
		log.Fatalf("Failed to get ref: %v", err)
	}
	baseSHA := ref.GetObject().GetSHA()

	// Create a new branch
	branchName := fmt.Sprintf("test-deployment-%d", time.Now().Unix())
	newRef := &github.Reference{
		Ref: github.String("refs/heads/" + branchName),
		Object: &github.GitObject{
			SHA: github.String(baseSHA),
		},
	}
	
	_, _, err = client.Git.CreateRef(ctx, owner, repo, newRef)
	if err != nil {
		log.Fatalf("Failed to create branch: %v", err)
	}
	log.Printf("✓ Created branch: %s\n", branchName)

	// Get the tree
	commit, _, err := client.Git.GetCommit(ctx, owner, repo, baseSHA)
	if err != nil {
		log.Fatalf("Failed to get commit: %v", err)
	}

	// Create a new file
	fileContent := fmt.Sprintf(`# Test Deployment

This file was created at %s to test deployment status on PRs.

## Testing
- Preview deployments
- Staging deployments
- PR status checks
`, time.Now().Format(time.RFC3339))

	blob, _, err := client.Git.CreateBlob(ctx, owner, repo, &github.Blob{
		Content:  github.String(fileContent),
		Encoding: github.String("utf-8"),
	})
	if err != nil {
		log.Fatalf("Failed to create blob: %v", err)
	}

	// Create new tree with the file
	entries := []*github.TreeEntry{
		{
			Path: github.String("test-deployment.md"),
			Mode: github.String("100644"),
			Type: github.String("blob"),
			SHA:  blob.SHA,
		},
	}

	tree, _, err := client.Git.CreateTree(ctx, owner, repo, commit.GetTree().GetSHA(), entries)
	if err != nil {
		log.Fatalf("Failed to create tree: %v", err)
	}

	// Create commit
	newCommit := &github.Commit{
		Message: github.String("Add test deployment file"),
		Tree:    tree,
		Parents: []*github.Commit{{SHA: github.String(baseSHA)}},
	}

	commitResult, _, err := client.Git.CreateCommit(ctx, owner, repo, newCommit, nil)
	if err != nil {
		log.Fatalf("Failed to create commit: %v", err)
	}

	// Update branch to point to new commit
	ref.Object.SHA = commitResult.SHA
	_, _, err = client.Git.UpdateRef(ctx, owner, repo, ref, false)
	if err != nil {
		log.Fatalf("Failed to update ref: %v", err)
	}

	// Create pull request
	pr, _, err := client.PullRequests.Create(ctx, owner, repo, &github.NewPullRequest{
		Title: github.String("Test PR for Deployment Status"),
		Head:  github.String(branchName),
		Base:  github.String(defaultBranch),
		Body:  github.String("This PR is created to test how deployment statuses appear on pull requests.\n\n- Preview deployments\n- Staging deployments\n- Multiple environments"),
	})
	if err != nil {
		log.Fatalf("Failed to create PR: %v", err)
	}

	log.Printf("\n✓ Successfully created PR #%d\n", pr.GetNumber())
	log.Printf("  Title: %s\n", pr.GetTitle())
	log.Printf("  URL: %s\n", pr.GetHTMLURL())
	log.Printf("\nNow run: go run cmd/test-pr-deployment/main.go -pr=%d\n", pr.GetNumber())
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