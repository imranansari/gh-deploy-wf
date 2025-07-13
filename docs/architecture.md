# Architecture Documentation

## C4 Component Diagram

```mermaid
C4Component
    title GitHub Deployment Workflow - Component Diagram

    Person(dev, "Developer", "Creates PR, triggers pipeline")
    
    System_Boundary(harness, "Harness CI/CD") {
        Component(pipeline, "Pipeline", "Go", "Builds and deploys application")
        Component(events, "Cloud Events", "CloudEvents", "Publishes deployment lifecycle events")
    }
    
    System_Boundary(temporal_system, "Temporal System") {
        Component(worker, "Temporal Worker", "Go", "Executes workflows and activities")
        Component(server, "Temporal Server", "Go", "Orchestrates workflow execution")
        
        Container_Boundary(workflows, "Workflows") {
            Component(create_wf, "GitHubDeploymentWorkflow", "Go", "Creates initial deployment")
            Component(update_wf, "UpdateDeploymentWorkflow", "Go", "Updates deployment status")
        }
        
        Container_Boundary(activities, "Activities") {
            Component(create_act, "CreateGitHubDeployment", "Go", "Creates GitHub deployment")
            Component(find_act, "FindGitHubDeployment", "Go", "Finds existing deployment") 
            Component(update_act, "UpdateGitHubDeploymentStatus", "Go", "Updates deployment status")
        }
        
        Component(client_factory, "GitHub Client Factory", "Go", "Manages GitHub App authentication")
    }
    
    System_Ext(github, "GitHub API", "Stores deployment information and status")
    
    System_Boundary(event_trigger, "Event Processing") {
        Component(event_handler, "Cloud Event Handler", "Go", "Receives Harness events, triggers workflows")
    }

    Rel(dev, pipeline, "Triggers via PR/push")
    Rel(pipeline, events, "Publishes lifecycle events")
    Rel(events, event_handler, "Delivers cloud events")
    
    Rel(event_handler, create_wf, "Triggers initial deployment")
    Rel(event_handler, update_wf, "Triggers status updates")
    
    Rel(worker, server, "Polls for tasks")
    Rel(create_wf, create_act, "Executes")
    Rel(update_wf, find_act, "Executes") 
    Rel(update_wf, update_act, "Executes")
    
    Rel(create_act, client_factory, "Uses")
    Rel(find_act, client_factory, "Uses")
    Rel(update_act, client_factory, "Uses")
    
    Rel(client_factory, github, "Authenticates & calls API")
    
    UpdateLayoutConfig($c4ShapeInRow="3", $c4BoundaryInRow="2")
```

## High-Level Sequence Diagram

```mermaid
sequenceDiagram
    participant Dev as Developer
    participant Harness as Harness Pipeline
    participant Events as Cloud Events
    participant EventHandler as Event Handler
    participant Temporal as Temporal Server
    participant Worker as Temporal Worker
    participant GitHub as GitHub API

    Note over Dev, GitHub: Complete Deployment Flow

    Dev->>Harness: Create PR / Push to branch
    activate Harness
    
    Harness->>Events: Publish "build.started" event
    Events->>EventHandler: Deliver cloud event
    EventHandler->>Temporal: Start GitHubDeploymentWorkflow
    
    Temporal->>Worker: Schedule CreateGitHubDeployment activity
    Worker->>GitHub: POST /repos/{owner}/{repo}/deployments
    GitHub-->>Worker: Return deployment ID
    Worker->>GitHub: POST /repos/{owner}/{repo}/deployments/{id}/statuses (pending)
    GitHub-->>Worker: Status updated
    Worker-->>Temporal: Deployment created (ID: 123)
    
    Note over Harness: CI Build Process
    Harness->>Harness: Run tests, build artifacts
    
    Harness->>Events: Publish "deployment.started" event
    Events->>EventHandler: Deliver cloud event
    EventHandler->>Temporal: Start UpdateDeploymentWorkflow (state: in_progress)
    
    Temporal->>Worker: Schedule FindGitHubDeployment activity
    Worker->>GitHub: GET /repos/{owner}/{repo}/deployments?sha=abc123&environment=pr-preview
    GitHub-->>Worker: Return deployment ID: 123
    
    Temporal->>Worker: Schedule UpdateGitHubDeploymentStatus activity  
    Worker->>GitHub: POST /repos/{owner}/{repo}/deployments/123/statuses (in_progress)
    GitHub-->>Worker: Status updated (shows spinning indicator)
    Worker-->>Temporal: Status updated to in_progress
    
    Note over Harness: Deployment Process
    Harness->>Harness: Deploy to environment
    
    Harness->>Events: Publish "deployment.completed" event
    Events->>EventHandler: Deliver cloud event
    EventHandler->>Temporal: Start UpdateDeploymentWorkflow (state: success)
    
    Temporal->>Worker: Schedule FindGitHubDeployment activity
    Worker->>GitHub: GET /repos/{owner}/{repo}/deployments?sha=abc123&environment=pr-preview
    GitHub-->>Worker: Return deployment ID: 123
    
    Temporal->>Worker: Schedule UpdateGitHubDeploymentStatus activity
    Worker->>GitHub: POST /repos/{owner}/{repo}/deployments/123/statuses (success)
    GitHub-->>Worker: Status updated (green checkmark)
    Worker-->>Temporal: Status updated to success
    
    deactivate Harness
    
    Note over Dev, GitHub: Developer sees deployment status in GitHub UI
```

## Low-Level Workflow Sequence

### GitHubDeploymentWorkflow (Creation)

```mermaid
sequenceDiagram
    participant Client as Workflow Client
    participant Temporal as Temporal Server
    participant Worker as Worker Process
    participant Activity as GitHub Activities
    participant GitHubAPI as GitHub API

    Client->>Temporal: StartWorkflow(GitHubDeploymentWorkflow, input)
    activate Temporal
    
    Temporal->>Worker: Schedule workflow task
    activate Worker
    
    Note over Worker: Workflow Execution Begins
    Worker->>Worker: Initialize workflow context & logger
    Worker->>Worker: Set activity options (retries, timeouts)
    
    Note over Worker: Step 1: Create Deployment
    Worker->>Temporal: Schedule CreateGitHubDeployment activity
    Temporal->>Worker: Execute activity
    activate Activity
    
    Activity->>Activity: Create GitHub client with App authentication
    Activity->>GitHubAPI: POST /repos/{owner}/{repo}/deployments
    activate GitHubAPI
    GitHubAPI-->>Activity: 201 Created {id: 123, url: "...", environment: "pr-preview"}
    deactivate GitHubAPI
    Activity-->>Worker: CreateDeploymentResult{DeploymentID: 123, URL: "...", Environment: "pr-preview"}
    deactivate Activity
    
    Worker->>Worker: Store deployment ID in workflow state
    Worker->>Worker: Log: "GitHub deployment created, deployment_id=123"
    
    Note over Worker: Step 2: Set Initial Status
    Worker->>Temporal: Schedule UpdateGitHubDeploymentStatus activity
    Temporal->>Worker: Execute activity
    activate Activity
    
    Activity->>Activity: Create GitHub client
    Activity->>GitHubAPI: POST /repos/{owner}/{repo}/deployments/123/statuses
    activate GitHubAPI
    Note right of GitHubAPI: Body: {state: "pending", description: "Deployment queued"}
    GitHubAPI-->>Activity: 201 Created
    deactivate GitHubAPI
    Activity-->>Worker: Status updated successfully
    deactivate Activity
    
    Worker->>Worker: Increment status_updates counter
    Worker->>Worker: Log: "Updated deployment to initial status, status=pending"
    
    Note over Worker: Step 3: Simulate MVP Success (2s delay)
    Worker->>Worker: workflow.Sleep(2 * time.Second)
    
    Note over Worker: Step 4: Final Status Update
    Worker->>Temporal: Schedule UpdateGitHubDeploymentStatus activity
    Temporal->>Worker: Execute activity
    activate Activity
    
    Activity->>Activity: Create GitHub client
    Activity->>GitHubAPI: POST /repos/{owner}/{repo}/deployments/123/statuses
    activate GitHubAPI
    Note right of GitHubAPI: Body: {state: "success", description: "Successfully deployed", environment_url: "https://..."}
    GitHubAPI-->>Activity: 201 Created
    deactivate GitHubAPI
    Activity-->>Worker: Status updated successfully
    deactivate Activity
    
    Worker->>Worker: Calculate workflow duration
    Worker->>Worker: Build final result object
    Worker->>Worker: Log: "Deployment workflow completed, deployment_id=123, final_status=success"
    
    Worker-->>Temporal: Return DeploymentWorkflowResult
    deactivate Worker
    Temporal-->>Client: Workflow completed with result
    deactivate Temporal
```

### UpdateDeploymentWorkflow (Status Update)

```mermaid
sequenceDiagram
    participant CloudEvent as Cloud Event Handler
    participant Temporal as Temporal Server  
    participant Worker as Worker Process
    participant FindActivity as FindGitHubDeployment
    participant UpdateActivity as UpdateGitHubDeploymentStatus
    participant GitHubAPI as GitHub API

    CloudEvent->>Temporal: StartWorkflow(UpdateDeploymentWorkflow, {state: "in_progress", commit: "abc123", env: "pr-preview"})
    activate Temporal
    
    Temporal->>Worker: Schedule workflow task
    activate Worker
    
    Note over Worker: Workflow Execution Begins
    Worker->>Worker: Initialize workflow context & logger
    Worker->>Worker: Set activity options (1min timeout, 3 retries)
    Worker->>Worker: Log: "Starting deployment status update, new_state=in_progress"
    
    Note over Worker: Step 1: Find Existing Deployment
    Worker->>Temporal: Schedule FindGitHubDeployment activity
    Temporal->>Worker: Execute activity
    activate FindActivity
    
    FindActivity->>FindActivity: Create GitHub client with App authentication
    FindActivity->>GitHubAPI: GET /repos/{owner}/{repo}/deployments?sha=abc123&environment=pr-preview&per_page=10
    activate GitHubAPI
    GitHubAPI-->>FindActivity: 200 OK [deployment{id: 123, sha: "abc123", environment: "pr-preview", created_at: "..."}]
    deactivate GitHubAPI
    
    FindActivity->>FindActivity: Extract most recent deployment ID (123)
    FindActivity->>FindActivity: Log: "Successfully found GitHub deployment, deployment_id=123, total_found=1"
    FindActivity-->>Worker: Return deployment ID: 123
    deactivate FindActivity
    
    Worker->>Worker: Log: "Found GitHub deployment, deployment_id=123"
    
    Note over Worker: Step 2: Update Deployment Status
    Worker->>Temporal: Schedule UpdateGitHubDeploymentStatus activity
    Temporal->>Worker: Execute activity
    activate UpdateActivity
    
    UpdateActivity->>UpdateActivity: Create GitHub client
    UpdateActivity->>UpdateActivity: Build status request with truncated description
    UpdateActivity->>GitHubAPI: POST /repos/{owner}/{repo}/deployments/123/statuses
    activate GitHubAPI
    Note right of GitHubAPI: Body: {state: "in_progress", description: "Deploying to pr-preview environment", log_url: "https://ci.harness.io/builds/123", auto_inactive: true}
    GitHubAPI-->>UpdateActivity: 201 Created {id: 456, state: "in_progress", url: "...", updated_at: "..."}
    deactivate GitHubAPI
    
    UpdateActivity->>UpdateActivity: Log: "Successfully updated GitHub deployment status, deployment_id=123, state=in_progress, status_id=456"
    UpdateActivity-->>Worker: Status updated successfully
    deactivate UpdateActivity
    
    Worker->>Worker: Log: "Successfully updated deployment status, deployment_id=123, new_state=in_progress"
    
    Worker-->>Temporal: Workflow completed successfully
    deactivate Worker
    Temporal-->>CloudEvent: Workflow completed
    deactivate Temporal
    
    Note over GitHubAPI: GitHub UI now shows spinning indicator for deployment
```