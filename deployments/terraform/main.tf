# Terraform Configuration for Autonomous Game Assist Agent CLI Infrastructure

terraform {
  required_version = ">= 1.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    google-beta = {
      source  = "hashicorp/google-beta"
      version = "~> 5.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

provider "google-beta" {
  project = var.project_id
  region  = var.region
}

variable "project_id" {
  type        = string
  description = "The GCP Project ID where resources will be provisioned"
}

variable "env" {
  type        = string
  description = "The environment (dev, staging, prod)"
  default     = "dev"
}

variable "region" {
  type        = string
  description = "The GCP region for resources"
  default     = "us-central1"
}

locals {
  name_prefix  = "${var.project_id}-${var.env}-${var.region}-gameassist"
  cluster_name = "${var.env}-${var.region}-ga-cluster"
  subnet_name  = var.region == "us-central1" ? "compliance-subnet-usc1" : "compliance-subnet"
  mandatory_labels = {
    environment = var.env
    owner       = "ikogan"
    cost-center = "gaming-assist-ai"
    managed-by  = "terraform"
  }
}

# GCS Bucket for final deliverables
resource "google_storage_bucket" "deliverables" {
  name                        = "${local.name_prefix}-bucket"
  location                    = var.region
  force_destroy               = true
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"

  versioning {
    enabled = true
  }

  labels = local.mandatory_labels
}

# Secret Manager secret for API keys
resource "google_secret_manager_secret" "api_keys" {
  secret_id = "${local.name_prefix}-secret"

  labels = local.mandatory_labels

  replication {
    auto {}
  }
}

# Enable Kubernetes Engine API
resource "google_project_service" "container" {
  service            = "container.googleapis.com"
  disable_on_destroy = false
}

# Dedicated service account for GKE Node Pool (Principle of Least Privilege)
resource "google_service_account" "gke_nodes" {
  account_id   = "${var.env}-${var.region}-ga-node-sa"
  display_name = "GKE Node Pool Service Account"
  description  = "Minimally privileged service account for GKE nodes in the gameassist environment"
}

# IAM Bindings to support minimal telemetry and node operational capability
resource "google_project_iam_member" "node_logging" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_project_iam_member" "node_monitoring_metric" {
  project = var.project_id
  role    = "roles/monitoring.metricWriter"
  member  = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_project_iam_member" "node_monitoring_viewer" {
  project = var.project_id
  role    = "roles/monitoring.viewer"
  member  = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_project_iam_member" "node_metadata_writer" {
  project = var.project_id
  role    = "roles/stackdriver.resourceMetadata.writer"
  member  = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_project_iam_member" "node_artifact_registry" {
  project = var.project_id
  role    = "roles/artifactregistry.reader"
  member  = "serviceAccount:${google_service_account.gke_nodes.email}"
}

resource "google_compute_subnetwork" "usc1_subnet" {
  count         = var.region == "us-central1" ? 1 : 0
  name          = "compliance-subnet-usc1"
  ip_cidr_range = "10.128.0.0/20"
  network       = "compliance-vpc"
  region        = "us-central1"
  project       = var.project_id
  private_ip_google_access = true

  secondary_ip_range {
    range_name    = "gke-pods"
    ip_cidr_range = "10.244.0.0/16"
  }
  secondary_ip_range {
    range_name    = "gke-services"
    ip_cidr_range = "10.245.0.0/20"
  }
}

# Provision a secure GKE Cluster (Standard Regional Cluster)
#tfsec:ignore:google-gke-enable-master-networks
resource "google_container_cluster" "primary" {
  name     = local.cluster_name
  location = var.region

  # We delete the default node pool and configure our custom sandboxed pool instead
  remove_default_node_pool = true
  initial_node_count       = 1

  # Enable Workload Identity on the Cluster
  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  # Configure Private Cluster to comply with vmExternalIpAccess Org Policy
  private_cluster_config {
    enable_private_nodes    = true
    enable_private_endpoint = false # Allow public access to control plane
    master_ipv4_cidr_block  = var.region == "us-central1" ? "172.16.0.16/28" : "172.16.0.0/28"
  }

  # Default to standard VPC networking
  network    = "compliance-vpc"
  subnetwork = var.region == "us-central1" ? google_compute_subnetwork.usc1_subnet[0].name : "compliance-subnet"

  ip_allocation_policy {
    cluster_secondary_range_name  = var.region == "us-central1" ? "gke-pods" : null
    services_secondary_range_name = var.region == "us-central1" ? "gke-services" : null
  }

  # Configure default node pool to comply with requireShieldedVm Org Policy
  node_config {
    service_account = google_service_account.gke_nodes.email
    preemptible     = true
    
    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }
  }

  resource_labels = local.mandatory_labels
  deletion_protection = false
}

# GKE Node Pool with gVisor enabled
resource "google_container_node_pool" "gvisor_nodes" {
  provider   = google-beta
  name       = "${var.env}-${var.region}-ga-np"
  location   = var.region
  cluster    = google_container_cluster.primary.name
  node_count = 1

  depends_on = [
    google_project_service.container,
    google_container_cluster.primary,
    google_project_iam_member.node_logging,
    google_project_iam_member.node_monitoring_metric,
    google_project_iam_member.node_monitoring_viewer,
    google_project_iam_member.node_metadata_writer,
    google_project_iam_member.node_artifact_registry
  ]

  node_config {
    preemptible     = true
    machine_type    = "e2-standard-4"
    service_account = google_service_account.gke_nodes.email

    metadata = {
      disable-legacy-endpoints = "true"
    }

    # Enable Workload Identity on the node pool
    workload_metadata_config {
      mode = "GKE_METADATA"
    }

    # Enable Shielded VM options to comply with requireShieldedVm Org Policy
    shielded_instance_config {
      enable_secure_boot          = true
      enable_integrity_monitoring = true
    }

    # Enable gVisor sandbox
    sandbox_config {
      sandbox_type = "gvisor"
    }

    labels = local.mandatory_labels

    tags = ["gameassist-runner", var.env]
  }
}

output "gcs_bucket_name" {
  value       = google_storage_bucket.deliverables.name
  description = "The name of the provisioned GCS bucket"
}

output "secret_id" {
  value       = google_secret_manager_secret.api_keys.secret_id
  description = "The ID of the provisioned Secret Manager secret"
}

output "node_pool_id" {
  value       = google_container_node_pool.gvisor_nodes.id
  description = "The ID of the GKE Node Pool"
}

# ==============================================================================
# Cloud NAT Infrastructure for Secure GKE Egress
# ==============================================================================

# Cloud Router required to host the NAT gateway
resource "google_compute_router" "router" {
  name    = "${local.name_prefix}-router"
  region  = var.region
  network = "compliance-vpc"
}

# Cloud NAT Gateway restricted to the GKE subnet
resource "google_compute_router_nat" "nat" {
  name                               = "${local.name_prefix}-nat"
  router                             = google_compute_router.router.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"

  subnetwork {
    name                    = var.region == "us-central1" ? google_compute_subnetwork.usc1_subnet[0].id : "compliance-subnet"
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }

  log_config {
    enable = true
    filter = "ERRORS_ONLY"
  }
}

