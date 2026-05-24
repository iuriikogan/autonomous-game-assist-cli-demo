# Autonomous Game Assist CLI & Runner

An enterprise-grade, cloud-native AI agent runner built in Go using the Google ADK (Agent Development Kit) framework and Gemini 3.1 Pro.

## High-Level Architecture

```mermaid
graph TD
    User[Game Developer] -->|CLI Prompt| Runner[cmd/agent-runner]
    Runner -->|Initiates| Coordinator[internal/agent/coordinator]
    
    subgraph Coordinator Workflow
        direction TB
        P1[Prompt Crafter Sub-Agent] -->|1. Expand Foley Description| P2[Creative Audio Sub-Agent]
        P2 -->|2. Synthesize WAV Output| P3[Unreal Agent]
        P3 -->|3. Retrieve Context via Vector Search| P3
        P3 -->|4. Generate UE5 Python Script| P4[Validation Sub-Agent]
        P4 -->|5. Dry-Run via Subprocess Sandbox| P4
        P4 -->|6. Verify Success / Auto-Correct| P5[GCS Uploader Sub-Agent]
    end
    
    P5 -->|7. Upload Assets| GCS[Google Cloud Storage]
    P3 -->|Query Blueprint context| GVS[Google Cloud Vector Search]
```

## Prerequisites

1. **Google Cloud Project**: Ensure standard billing is enabled.
2. **Workload Identity Federation**: Enable inside your GKE cluster to run runner pods without mounting hardcoded credentials.
3. **API Services**:
   - Gemini Enterprise Agent Platform AI API
   - Cloud Storage API
   - Secret Manager API

## Local & Container Environment Configuration

Export the following environment variables:

```bash
# GCP project configurations
export GCP_PROJECT="your-gcp-project-id"
export GCP_LOCATION="us-central1"

# GCS storage delivery bucket
export GCS_BUCKET="autonomous-game-assist-deliverables-dev"

# Vector Search index configurations
export VECTOR_API_ENDPOINT="us-central1-aiplatform.googleapis.com"
export VECTOR_INDEX_ENDPOINT="projects/123456789/locations/us-central1/indexEndpoints/987654321"
export DEPLOYED_INDEX_ID="my_deployed_index_1"

# Optional secret version path for Gemini API Key (if not using ADC)
export GEMINI_API_KEY_SECRET_PATH="projects/your-gcp-project-id/secrets/gemini-key/versions/latest"
```

## Running the Binary Locally

Compile and run the coordinator synchronously:

```bash
# Build
go build -o agent-runner ./cmd/agent-runner

# Run
./agent-runner -prompt "Generate metal footstep sound links on trigger overlap"
```

## Docker Compilation

Build the multi-stage container:

```bash
docker build -t gcr.io/your-gcp-project-id/agent-runner:latest .
```

## CI/CD Pipeline (Cloud Build)

Run Cloud Build to automatically run checks, Trivy security scans, compile the image, and push it to Artifact Registry:

```bash
gcloud builds submit --config=cloudbuild.yaml \
  --substitutions=_REGISTRY_LOCATION="us-central1",_REPOSITORY_NAME="autonomous-game-assist"
```

## GitHub Actions CI/CD with Workload Identity Federation (WIF)

We utilize GitHub Actions to trigger Cloud Build pipelines securely without storing long-lived GCP service account keys. This is achieved using Workload Identity Federation (WIF).

### Setup Instructions

#### 1. Provision WIF Infrastructure
Apply the Terraform configuration in `deployments/terraform/` to create the Workload Identity Pool, Provider, and Service Account.

```bash
cd deployments/terraform
terraform init
# Ensure you provide your GKE cluster name to satisfy the variable
terraform apply -var="project_id=develop-491110" -var="gke_cluster_name=your-gke-cluster"
```

#### 2. Retrieve Terraform Outputs
After successful application, note the following outputs:
- `wif_provider_name`
- `github_actions_service_account_email`

#### 3. Configure GitHub Secrets
In your GitHub repository, navigate to **Settings > Secrets and variables > Actions** and add the following secrets:

| Secret Name | Value | Description |
| :--- | :--- | :--- |
| `GCP_PROJECT_ID` | `develop-491110` | Your GCP Project ID |
| `GCP_WIF_PROVIDER` | *Value from `wif_provider_name` output* | Full resource name of the WIF Provider |
| `GCP_SA_EMAIL` | *Value from `github_actions_service_account_email` output* | Email of the CI/CD Service Account |

### How it Works
The workflow in `.github/workflows/gcp-cloudbuild.yml` triggers on:
- Pushes to `main`
- Pull Requests targeting `main`

It authenticates to GCP using WIF and then submits the build to Cloud Build, executing all steps defined in `cloudbuild.yaml` (linting, testing, security scanning, and pushing the Docker image to Artifact Registry).
