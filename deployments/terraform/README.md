# Autonomous Game Assist CLI Infrastructure Deployment

This directory contains the Infrastructure-as-Code (IaC) manifests for provisioning the security-hardened, production-ready infrastructure for the **Autonomous Game Assist CLI** on Google Cloud Platform.

---

## Security Architecture

The infrastructure is designed in alignment with **Google Cloud Security Best Practices** and the **Principle of Least Privilege (PoLP)**:

1.  **Workload Identity Federation (WIF)**: Configures passwordless, keyless authentication between GitHub Actions and Google Cloud, restricting access explicitly to the repository: `ikogan/autonomous-game-assist-cli`.
2.  **Sandboxed GKE Node Pool**: Creates an isolated node pool with **gVisor** enabled (`sandbox_type = "gvisor"`) to protect host kernels from potentially compromised workloads.
3.  **Least Privilege Node Identity**: Configures GKE nodes to run using a dedicated service account (`ga-node-sa`) granted only necessary logging, monitoring, and Artifact Registry read permissions, avoiding GKE node privilege escalation.
4.  **Hardened GCS Bucket**: Enforces bucket-level public access prevention and enables Object Versioning for reliable disaster recovery.

---

## Prerequisites

1.  **Google Cloud SDK (`gcloud`)** installed and authenticated.
2.  **Terraform CLI** (version `>= 1.0.0`) installed.
3.  An existing **GKE Cluster** to attach the hardened node pool to.

---

## Local Deployment Instructions

### 1. Set Environment Variables
Copy the sample variables file and populate it with your specific GCP details:
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
Ensure that configuration is syntactically correct and structurally valid:
```bash
terraform validate
```

### 4. Plan Deployment
Preview the changes that will be applied to your GCP environment:
```bash
terraform plan
```

### 5. Apply Changes
Deploy the hardened infrastructure:
```bash
terraform apply
```

---

## CI/CD Pipeline Validation

The repository is bundled with a `cloudbuild.yaml` configuration to automate:
1.  **Syntax Validation**: Runs `terraform validate`.
2.  **IaC Security Scanning**: Utilizes **Aquasecurity Trivy** to scan configuration manifests for potential misconfigurations or high/critical vulnerabilities before applying changes.

To run the validation pipeline locally or trigger in Cloud Build:
```bash
gcloud builds submit --config=cloudbuild.yaml .
```
