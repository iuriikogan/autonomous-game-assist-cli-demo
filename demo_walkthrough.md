# Step-by-Step Deployment & Operation Demo Guide

This guide provides comprehensive, command-by-command instructions for setting up the **Autonomous Game Assist** platform infrastructure, deploying the agent runner in a Google Kubernetes Engine sandbox, and executing end-to-end jobs using the developer CLI.

---

## Prerequisites & Local Setup

Before beginning the setup, ensure you have:
* A **Google Cloud Project** with billing enabled.
* The **Google Cloud SDK (`gcloud`)** installed and authenticated.
* **Kubernetes CLI (`kubectl`)** installed and configured to access your GKE cluster.
* **Go 1.23** installed locally.

### 1. Local Shell Context Configuration
Run the following commands in your terminal to set up the deployment environment variables. Replace placeholders with your actual project settings:

```bash
# Core GCP configurations
export GCP_PROJECT=<INSERT YOUR PROJECT ID>
export GCP_LOCATION="us-central1"
export ENV="dev"

# Storage delivery configurations
export GCS_BUCKET="${GCP_PROJECT}-${ENV}-${GCP_LOCATION}-gameassist-bucket"

# Vector Search 2.0 Configurations
export VECTOR_COLLECTION_ID="${GCP_PROJECT}-${ENV}-${GCP_LOCATION}-gameassist-collection"

# GitHub Repository Details (For PR Agent review delivery)
export GITHUB_OWNER="iuriikogan" ## replace with your own repo
export GITHUB_REPO="OpenWorldRPG"
export GITHUB_BASE_BRANCH="main"
export GITHUB_TOKEN_SECRET_PATH="projects/${GCP_PROJECT}/secrets/github-token/versions/latest"
```

Authenticate your terminal environment to GCP:
```bash
gcloud auth login
gcloud config set project ${GCP_PROJECT}
```

---

## Section 1: Provisioning Vertex AI Vector Search 2.0

We utilize serverless **Vector Search 2.0 Collections** with automatic **Gemini Text Embeddings** (`gemini-embedding-2-preview`) to store and retrieve Unreal Engine source code and Blueprint context.

### 1. Create the Vector Search Collection
Run the following `gcloud` command to create the collection asynchronously:

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
Creating the collection is a long-running operation. Check its status to ensure it is in the `READY` or `ACTIVE` state before indexing:
```bash
gcloud beta vector-search collections describe "${VECTOR_COLLECTION_ID}" \
  --location="${GCP_LOCATION}" \
  --project="${GCP_PROJECT}"
```

### 3. Populate the Collection via the Codebase Indexer
Compile and run the local `vector-indexer` utility to scan your target codebase (`Source/` and `Content/` directories), generate Gemini technical summaries, and insert them into your Vector Collection.

```bash
# Build the vector-indexer executable
go build -o vector-indexer ./cmd/vector-indexer

# Run the indexer (points by default to scratch/OpenWorldRPG repository)
./vector-indexer \
  --src "scratch/OpenWorldRPG" \
  --project "${GCP_PROJECT}" \
  --location "${GCP_LOCATION}" \
  --collection "${VECTOR_COLLECTION_ID}"
```

---

## Section 2: Provisioning Storage, Secrets & IAM

The agent requires a Google Cloud Storage delivery bucket for Foley WAV sounds/Unreal Python scripts, and access to a Secret Manager secret containing your GitHub Personal Access Token.

### 1. Create the GCS Deliverables Bucket
Create a hardened GCS bucket with uniform bucket-level access and public access prevention:
```bash
gcloud storage buckets create "gs://${GCS_BUCKET}" \
  --location="${GCP_LOCATION}" \
  --uniform-bucket-level-access

# Enforce public access prevention for shift-left security compliance
gcloud storage buckets update "gs://${GCS_BUCKET}" \
  --public-access-prevention
```

### 2. Create the GitHub Secret in Secret Manager
Store your GitHub Personal Access Token (with repository write scopes) securely:
```bash
# Create the secret container
gcloud secrets create github-token \
  --project="${GCP_PROJECT}" \
  --replication-policy="automatic" \
  --labels="environment=${ENV},owner=ikogan"

# Add your actual token as version 1
# Note: Replace YOUR_GITHUB_TOKEN with your actual personal access token
echo -n "YOUR_GITHUB_TOKEN" | gcloud secrets versions add github-token --data-file=-
```

---

## Section 3: GKE Hardening & Agent Runner Security Setup

We implement GKE **Workload Identity Federation** to allow GKE Pods to securely authenticate to Google Cloud APIs using least-privilege Google Service Accounts, running workloads under the **gVisor** sandbox.

### 1. Create the Isolated Namespace
```bash
kubectl apply -f deployments/kubernetes/namespace.yaml
```

### 2. Create the Least-Privilege Google Service Account (GSA)
```bash
gcloud iam service-accounts create game-assist-runner-gsa \
  --project="${GCP_PROJECT}" \
  --description="Google Service Account for GKE Agent Runner Pods" \
  --display-name="Game Assist Runner GSA"
```

### 3. Bind Necessary IAM Permissions to the GSA
Grant the runner GSA permissions to access Vertex AI, write to the GCS bucket, retrieve secrets, and export distributed traces to Cloud Trace:

```bash
# Vertex AI User (for LLM reasoning & Vector Search querying)
gcloud projects add-iam-policy-binding "${GCP_PROJECT}" \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/aiplatform.user"

# Storage Object Admin (to upload synthesized game assets to GCS)
gcloud storage buckets add-iam-policy-binding "gs://${GCS_BUCKET}" \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/storage.objectAdmin"

# Secret Manager Accessor (to retrieve the GitHub token version)
gcloud secrets add-iam-policy-binding github-token \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/secretmanager.secretAccessor"

# Cloud Trace Agent (for distributed OpenTelemetry tracing)
gcloud projects add-iam-policy-binding "${GCP_PROJECT}" \
  --member="serviceAccount:game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/cloudtrace.agent"
```

### 4. Configure Workload Identity Binding
Enable the Kubernetes Service Account (`game-assist-agent-runner` in namespace `game-assist`) to impersonate the Google Service Account:

```bash
gcloud iam service-accounts add-iam-policy-binding "game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" \
  --role="roles/iam.workloadIdentityUser" \
  --member="serviceAccount:${GCP_PROJECT}.svc.id.goog[game-assist/game-assist-agent-runner]"
```

### 5. Create & Apply the Annotated Kubernetes Service Account
Apply the Kubernetes Service Account manifest containing the GSA impersonation mapping.

Ensure you replace `YOUR_GCP_PROJECT_ID` in `deployments/kubernetes/service_account.yaml` with `${GCP_PROJECT}`:
```bash
# Update the template with your real Project ID
sed -i "s/YOUR_GCP_PROJECT_ID/${GCP_PROJECT}/g" deployments/kubernetes/service_account.yaml

# Apply to cluster
kubectl apply -f deployments/kubernetes/service_account.yaml
```

---

## Section 4: Building the Agent Runner Container

Create a target Docker repository inside Artifact Registry and submit the build request to Cloud Build to compile the code, scan the container for vulnerabilities using Trivy and Wiz, and push it securely to Artifact Registry.

### 1. Create Artifact Registry Repository
```bash
gcloud artifacts repositories create autonomous-game-assist \
  --repository-format=docker \
  --location="${GCP_LOCATION}" \
  --description="Secure repository for Game Assist agent runner containers"
```

### 2. Submit Build to Cloud Build
Ensure you have configured the substitution arguments correctly:
```bash
gcloud builds submit --config=cloudbuild.yaml \
  --substitutions=_REGISTRY_LOCATION="${GCP_LOCATION}",_REPOSITORY_NAME="autonomous-game-assist"
```

> [!NOTE]
> Make sure the Wiz Client ID and Client Secret are stored inside your project's Secret Manager in order for the `wiz-container-scan` step to authenticate successfully, or modify `cloudbuild.yaml` to skip the scanning steps if not applicable to your testing environment.

---

## Section 5: Running the Demo via `game-assist` CLI

With infrastructure and containers successfully provisioned, you are now ready to dispatch and monitor your autonomous gaming assist workloads.

### 1. Compile the CLI Tool
```bash
go build -o game-assist ./cmd/game-assist
```

### 2. Dispatch a New Natural Language Request
Launch a secure GKE job inside the `gvisor` sandboxed runtime to generate a custom Foley sound effect and build a level automation script using Vertex AI:

```bash
# Set runner image destination
RUNNER_IMAGE="${GCP_LOCATION}-docker.pkg.dev/${GCP_PROJECT}/autonomous-game-assist/agent-runner:latest"

# Dispatched Generation Request
./game-assist generate "Generate a heavy, metallic steel footstep sound effect that plays when an actor overlaps the trigger volume" \
  --user "dev_ikogan" \
  --image "${RUNNER_IMAGE}"
```

### 3. Streaming & Monitoring
The `game-assist` CLI automatically:
1. Dispatches the secured Kubernetes Job `game-assist-<session_id>` in the `game-assist` namespace.
2. Detects GKE Pod creation and streams standard output/logs directly to your local terminal.
3. Execution results in:
   - Prompt expansion with Gemini 3.1 Pro.
   - Multimodal WAV sound generation.
   - Vector retrieval of RPG base classes and overlap event templates.
   - Generation and sandboxed validation of the Python automation script.
   - Delivery of assets to the GCS bucket.
   - Opening a new Pull Request in GitHub containing the Python file and public listening links.

### 4. Downloading local synthesized deliverables
Once the generation job completes successfully, note the output Session ID (e.g. `session-1713919121`). You can download the deliverables locally:

```bash
./game-assist download \
  --session "session-1713919121" \
  --bucket "${GCS_BUCKET}" \
  --dir "./deliverables"
```

Check the `./deliverables/` directory to view your generated technical Unreal python automation script and listen to your synthesized game sound effects.

---

## Section 6: Resource Governance & Teardown

To avoid unnecessary charges, execute the following commands when your demo session is complete to tear down all provisioned resources:

```bash
# 1. Delete GKE Namespace & active workloads
kubectl delete namespace game-assist

# 2. Empty and delete the GCS deliverables bucket
gcloud storage rm --recursive "gs://${GCS_BUCKET}"

# 3. Delete the Secret Manager secret
gcloud secrets delete github-token --quiet

# 4. Delete the Vector Search 2.0 Collection
gcloud beta vector-search collections delete "${VECTOR_COLLECTION_ID}" \
  --location="${GCP_LOCATION}" \
  --quiet

# 5. Delete the Google Service Account (GSA)
gcloud iam service-accounts delete "game-assist-runner-gsa@${GCP_PROJECT}.iam.gserviceaccount.com" --quiet

# 6. Delete the Artifact Registry repository
gcloud artifacts repositories delete autonomous-game-assist \
  --location="${GCP_LOCATION}" \
  --quiet
```
