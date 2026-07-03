# Step-by-Step Deployment & Operation Demo Guide

This guide provides comprehensive, command-by-command instructions for setting up the **Autonomous Game Assist** platform infrastructure, deploying the agent runner in a Google Kubernetes Engine sandbox, and executing end-to-end jobs using the developer CLI.

---

## Prerequisites & Local Setup

Before beginning setup, ensure you have:
* A **Google Cloud Project** with billing enabled.
* The **Google Cloud SDK (`gcloud`)** installed and authenticated.
* **Kubernetes CLI (`kubectl`)** installed and configured for your GKE cluster.
* **Go 1.23+** installed locally.

### 1. Local Shell Context Configuration
Set environment variables for deployment. Replace placeholders with your actual GCP details:

```bash
# Core GCP configurations
export GCP_PROJECT=<INSERT YOUR PROJECT ID>
export GCP_LOCATION="us-central1"
export ENV="dev"

# Storage delivery configurations
export GCS_BUCKET="${GCP_PROJECT}-${ENV}-${GCP_LOCATION}-gameassist-bucket"

# Vertex AI Vector Search 2.0 Collection
export VECTOR_COLLECTION_ID="${GCP_PROJECT}-${ENV}-${GCP_LOCATION}-gameassist-collection"

# GitHub Repository Details (For PR Agent review delivery)
export GITHUB_OWNER="iuriikogan" ## replace with your own repo
export GITHUB_REPO="OpenWorldRPG"
export GITHUB_BASE_BRANCH="main"
export GITHUB_TOKEN_SECRET_PATH="projects/${GCP_PROJECT}/secrets/github-token/versions/latest"
```

Authenticate your shell environment:
```bash
gcloud auth login
gcloud config set project ${GCP_PROJECT}
```

---

## Section 1: Provisioning Vertex AI Vector Search 2.0

We utilize serverless **Vector Search 2.0 Collections** with automatic **Gemini Text Embeddings** (`gemini-embedding-2-preview`) to index and search Unreal Engine source code and Blueprint context.

### 1. Create the Vector Search Collection
Run the following `gcloud` command to create the serverless collection:

```bash
# Define Schema for structured elements
DATA_SCHEMA='{
  "type": "object",
  "properties": {
    "path": { "type": "string" },
    "type": { "type": "string" },
    "description": { "type": "string" }
  },
  "required": ["path", "type", "description"]
}'

# Define Gemini Auto-Embedding Vector configurations
VECTOR_SCHEMA='{
  "asset_embedding": {
    "denseVector": {
      "dimensions": 3072,
      "vertexEmbeddingConfig": {
        "modelId": "gemini-embedding-2-preview",
        "textTemplate": "Asset Path: {path}\nType: {type}\nDescription: {description}",
        "taskType": "RETRIEVAL_DOCUMENT"
      }
    }
  }
}'

# Provision the Vector Search 2.0 Collection
gcloud beta vector-search collections create "${VECTOR_COLLECTION_ID}" \
  --project="${GCP_PROJECT}" \
  --location="${GCP_LOCATION}" \
  --description="OpenWorldRPG structural semantic code and blueprint asset collection" \
  --data-schema="${DATA_SCHEMA}" \
  --vector-schema="${VECTOR_SCHEMA}" \
  --labels="environment=${ENV},owner=ikogan,cost-center=gaming-assist-ai,managed-by=vector-indexer"
```

### 2. Verify Collection Status
Check status until the collection state is `READY` or `ACTIVE`:
```bash
gcloud beta vector-search collections describe "${VECTOR_COLLECTION_ID}" \
  --location="${GCP_LOCATION}" \
  --project="${GCP_PROJECT}"
```

### 3. Populate Collection via `vector-indexer`
Compile and execute `cmd/vector-indexer` to scan target codebase directories (`Source/` and `Content/`), generate embeddings, and populate the Vector Collection:

```bash
# Build the vector-indexer executable
go build -o vector-indexer ./cmd/vector-indexer

# Run the indexer against OpenWorldRPG target repository
./vector-indexer \
  --src "scratch/OpenWorldRPG" \
  --project "${GCP_PROJECT}" \
  --location "${GCP_LOCATION}" \
  --collection "${VECTOR_COLLECTION_ID}"
```

---

## Section 2: Provisioning Storage, Secrets & IAM

### 1. Create the GCS Deliverables Bucket
```bash
gcloud storage buckets create "gs://${GCS_BUCKET}" \
  --location="${GCP_LOCATION}" \
  --uniform-bucket-level-access

# Enforce public access prevention
gcloud storage buckets update "gs://${GCS_BUCKET}" \
  --public-access-prevention
```

### 2. Create GitHub Access Secret in Secret Manager
```bash
# Create secret container
gcloud secrets create github-token \
  --project="${GCP_PROJECT}" \
  --replication-policy="automatic" \
  --labels="environment=${ENV},owner=ikogan,cost-center=gaming-assist-ai,managed-by=gcloud"

# Add secret version
echo -n "YOUR_GITHUB_TOKEN" | gcloud secrets versions add github-token --data-file=-
```
## Section 3: GKE Hardening & Workload Identity Setup

### 1. Create Namespace & Service Accounts
```bash
kubectl apply -f deployments/kubernetes/namespace.yaml

# Create Least-Privilege Google Service Account (GSA)
gcloud iam service-accounts create game-assist-runner-gsa \
  --project="${GCP_PROJECT}" \
  --description="Google Service Account for GKE Agent Runner Pods" \
  --display-name="Game Assist Runner GSA"
```

### 2. Bind IAM Roles to GSA
```bash
# Vertex AI User (for LLM reasoning & Vector Search querying)
gcloud projects add-iam-policy-binding "${GCP_PROJECT}" \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/aiplatform.user"

# Storage Object Admin (to deliver assets to GCS)
gcloud storage buckets add-iam-policy-binding "gs://${GCS_BUCKET}" \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin"

# Secret Manager Accessor
gcloud secrets add-iam-policy-binding github-token \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# Cloud Trace Agent
gcloud projects add-iam-policy-binding "${GCP_PROJECT}" \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/cloudtrace.agent"
```

### 3. Bind Workload Identity (KSA to GSA)
```bash
gcloud iam service-accounts add-iam-policy-binding "game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/iam.workloadIdentityUser" \
  --member="serviceAccount:${GCP_PROJECT}.svc.id.goog[game-assist/game-assist-agent-runner]"

# Apply annotated Kubernetes Service Account
sed -i "s/YOUR_GCP_PROJECT_ID/${GCP_PROJECT}/g" deployments/kubernetes/service_account.yaml
kubectl apply -f deployments/kubernetes/service_account.yaml
```

## Section 4: Building & Deploying the Agent Runner

### 1. Create Artifact Registry Repository
```bash
gcloud artifacts repositories create autonomous-game-assist \
  --repository-format=docker \
  --location="${GCP_LOCATION}" \
  --description="Secure repository for Game Assist agent runner containers"
```

### 2. Submit Build to Cloud Build
```bash
gcloud builds submit --config=cloudbuild.yaml \
  --substitutions=_REGISTRY_LOCATION="${GCP_LOCATION}",_REPOSITORY_NAME="autonomous-game-assist"
```

---

## Section 5: Running the Demo via `game-assist` CLI

### 1. Compile CLI Tool
```bash
go build -o game-assist ./cmd/game-assist
```

### 2. Dispatch Natural Language Integration Request
Dispatch a job to select a pre-existing Foley sound effect and build a UE5 Python automation script using Gemini 3.1 Pro and Vector Search 2.0:

```bash
RUNNER_IMAGE="${GCP_LOCATION}-docker.pkg.dev/${GCP_PROJECT}/autonomous-game-assist/agent-runner:latest"

./game-assist generate "Integrate existing metallic steel footstep sound effect that plays when an actor overlaps the trigger volume" \
  --user "dev_ikogan" \
  --image "${RUNNER_IMAGE}"
```

### 3. Monitoring & Results
The `game-assist` CLI streams logs directly from the GKE gVisor sandbox. The job will:
1. Query Vector Search 2.0 for target Blueprint classes & generate UE5 Python integration script using Gemini 3.1 Pro (`gemini-3.1-pro-preview`).
2. Validate the UE5 Python integration script in the isolated subprocess sandbox.
3. Upload deliverables (WAV audio asset and validated Python script) to GCS.
4. Open a GitHub Pull Request with asset download links.

### 4. Downloading Synthesized Deliverables
Download deliverables using output Session ID:

```bash
./game-assist download \
  --session "session-1713919121" \
  --bucket "${GCS_BUCKET}" \
  --dir "./deliverables"
```

---

## Section 6: Resource Governance & Teardown

To avoid ongoing resource costs, clean up provisioned infrastructure:

```bash
# 1. Delete GKE Namespace
kubectl delete namespace game-assist

# 2. Delete GCS Deliverables Bucket
gcloud storage rm --recursive "gs://${GCS_BUCKET}"

# 3. Delete Secret Manager secret
gcloud secrets delete github-token --quiet

# 4. Delete Vector Search 2.0 Collection
gcloud beta vector-search collections delete "${VECTOR_COLLECTION_ID}" \
  --location="${GCP_LOCATION}" \
  --quiet

# 5. Delete Google Service Account
gcloud iam service-accounts delete "game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" --quiet

# 6. Delete Artifact Registry repository
gcloud artifacts repositories delete autonomous-game-assist \
  --location="${GCP_LOCATION}" \
  --quiet
```
