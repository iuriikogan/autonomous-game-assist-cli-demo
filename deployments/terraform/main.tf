# Terraform Configuration for Autonomous Game Assist Agent CLI Infrastructure

terraform {
  required_version = ">= 1.0.0"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
  }
}

provider "google" {
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

variable "gke_cluster_name" {
  type        = string
  description = "The name of the existing GKE cluster to attach the node pool to"
}

locals {
  name_prefix = "${var.project_id}-${var.env}-${var.region}-gameassist"
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

# GKE Node Pool with gVisor enabled
resource "google_container_node_pool" "gvisor_nodes" {
  name       = "${local.name_prefix}-nodepool"
  location   = var.region
  cluster    = var.gke_cluster_name
  node_count = 1

  node_config {
    preemptible  = true
    machine_type = "e2-standard-4"

    # Enable Workload Identity on the node pool
    workload_metadata_config {
      mode = "GKE_METADATA"
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
