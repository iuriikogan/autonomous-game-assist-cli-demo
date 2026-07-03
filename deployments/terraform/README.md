# Autonomous Game Assist CLI Infrastructure Deployment

This directory contains the Infrastructure-as-Code (IaC) manifests for provisioning security-hardened, production-ready infrastructure for the **Autonomous Game Assist CLI** on Google Cloud Platform using Terraform.

---

## Security & FinOps Architecture

The infrastructure configuration complies with **Google Cloud Security Best Practices**, **Principle of Least Privilege (PoLP)**, and enterprise FinOps labeling guidelines:

1. **Workload Identity Federation (WIF)**: Enables keyless authentication between GitHub Actions and Google Cloud, bound explicitly to `ikogan/autonomous-game-assist-cli`.
2. **Sandboxed GKE Node Pool**: Configures node pools with **gVisor** enabled (`sandbox_type = "gvisor"`) to isolate container workloads from host kernel execution.
3. **Least Privilege Service Account**: Dedicated node service account (`ga-node-sa`) granted strictly required logging, monitoring, and Artifact Registry reader permissions.
4. **Hardened GCS Bucket**: Enforces uniform bucket-level access and public access prevention.
5. **Mandatory FinOps Labeling**: Injects default labels across all resources (`environment`, `owner`, `cost-center`, `managed-by = "terraform"`).

---

## Prerequisites

1. **Google Cloud SDK (`gcloud`)** installed and authenticated.
2. **Terraform CLI** (version `>= 1.0.0`) installed.
3. An existing **GKE Cluster** to attach the hardened node pool to.

---

## Local Deployment Instructions

### 1. Configure Environment Variables
Copy sample variables file and populate your project configuration:
```bash
cp terraform.tfvars.sample terraform.tfvars
```

Edit `terraform.tfvars`:
```hcl
project_id       = "your-gcp-project-id"
region           = "us-central1"
env              = "dev"
gke_cluster_name = "your-existing-gke-cluster"
```

### 2. Initialize Terraform
```bash
terraform init
```

### 3. Validate Configuration
```bash
terraform validate
```

### 4. Security & Compliance Scan (Trivy / tfsec)
```bash
trivy config .
```

### 5. Plan Deployment
```bash
terraform plan
```

### 6. Apply Manifests
```bash
terraform apply
```

---

## Key Terraform Outputs

Upon successful deployment, Terraform emits outputs required by GitHub Actions CI/CD workflows:

| Output Name | Description | Target Secret |
| :--- | :--- | :--- |
| `wif_provider_name` | Full resource path of Workload Identity Provider | `GCP_WIF_PROVIDER` |
| `github_actions_service_account_email` | Service account email for GitHub Actions | `GCP_SA_EMAIL` |

---

## CI/CD Pipeline Validation

The directory is integrated with `cloudbuild.yaml` to automate:
1. **Syntax Validation**: Runs `terraform validate`.
2. **IaC Vulnerability Scan**: Uses **Aquasecurity Trivy** to scan manifests for security misconfigurations.

Submit local validation to Cloud Build:
```bash
gcloud builds submit --config=cloudbuild.yaml .
```
