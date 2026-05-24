# Terraform resources for Workload Identity Federation (WIF) and GitHub Actions integration

data "google_project" "current" {}

# Workload Identity Pool for GitHub Actions
resource "google_iam_workload_identity_pool" "github_pool" {
  workload_identity_pool_id = "github-actions-pool"
  display_name              = "GitHub Actions Pool"
  description               = "Identity pool for GitHub Actions CI/CD"
}

# Workload Identity Provider for GitHub Actions
resource "google_iam_workload_identity_pool_provider" "github_provider" {
  workload_identity_pool_id          = google_iam_workload_identity_pool.github_pool.workload_identity_pool_id
  workload_identity_pool_provider_id = "github-provider"
  display_name                        = "GitHub Provider"
  description                         = "OIDC provider for GitHub Actions"

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.actor"      = "assertion.actor"
    "attribute.repository" = "assertion.repository"
  }

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
  }

  # Restrict to specific repository for security
  attribute_condition = "assertion.repository == 'ikogan/autonomous-game-assist-cli'"
}

# Service Account for GitHub Actions
resource "google_service_account" "github_actions" {
  account_id   = "github-actions-cicd"
  display_name = "GitHub Actions CI/CD Service Account"
  description  = "Service Account used by GitHub Actions to trigger builds"
}

# Allow GitHub Actions to impersonate the Service Account
resource "google_service_account_iam_member" "wif_impersonation" {
  service_account_id = google_service_account.github_actions.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github_pool.name}/attribute.repository/ikogan/autonomous-game-assist-cli"
}

# Grant Cloud Build Editor role to the GitHub Actions Service Account
resource "google_project_iam_member" "github_actions_cloudbuild" {
  project = var.project_id
  role    = "roles/cloudbuild.builds.editor"
  member  = "serviceAccount:${google_service_account.github_actions.email}"
}

# Output the WIF Provider Name and Service Account Email for GitHub Actions configuration
output "wif_provider_name" {
  value       = google_iam_workload_identity_pool_provider.github_provider.name
  description = "The full resource name of the Workload Identity Provider"
}

output "github_actions_service_account_email" {
  value       = google_service_account.github_actions.email
  description = "The email of the GitHub Actions Service Account"
}
