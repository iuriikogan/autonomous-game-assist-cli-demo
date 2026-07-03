# Multi-Agent Architecture on Google Cloud Platform

This document provides a comprehensive overview of the enterprise-grade architecture of the Autonomous Game Assist platform, detailing how sequential agent pipelines cooperate, integrate with GCP-native resources, and leverage distributed tracing via OpenTelemetry and Google Cloud Trace.

---

## 1. Component Architecture

The platform consists of a structured CLI suite that populates semantic search collections, dispatches execution jobs to isolated GKE sandboxes, and coordinates high-fidelity Foley audio selection, Blueprint retrieval, sandboxed Python code validation, and pull request delivery.

```mermaid
graph TB
    subgraph Developer Environment
        CLI[Game Assist CLI: cmd/game-assist]
        Indexer[Vector Indexer: cmd/vector-indexer]
    end

    subgraph GCP Project [Autonomous Game Assist Platform]
        subgraph Orchestration & Execution
            Runner[Agent Runner Process: GKE Job]
            ADK[ADK Orchestrator]
        end

        subgraph GenAI & Search Services
            Gemini[Gemini 3.1 Pro: gemini-3.1-pro-preview]
            VectorSearch[Vertex AI Vector Search 2.0 Collection]
        end

        subgraph Storage & Security
            GCS[Cloud Storage Buckets]
            SecretManager[Secret Manager]
        end

        subgraph Observability
            CloudTrace[Google Cloud Trace]
            CloudLogging[Cloud Logging]
        end
    end

    subgraph External Services
        GitHub[GitHub VCS Server]
    end

    %% Ingestion
    Indexer -->|Dense Vector Embeddings| VectorSearch

    %% Inputs
    CLI -->|Dispatch GKE Job| Runner
    CLI -.->|Download Assets & Scripts| GCS
    
    %% Orchestration
    Runner -->|Bootstrap Session & Audio State| ADK
    ADK -->|1. Query Code/Blueprint Context & Gen UE5 Script| UnrealAgent[Unreal Agent]
    ADK -->|2. Sandboxed Dry-Run & Verification| ValidationAgent[Validation Agent]
    ADK -->|3. Upload Deliverables| GCSUploader[GCS Uploader Agent]
    ADK -->|4. Commit & Create PR| PRAgent[Pull Request Agent]

    %% Services
    UnrealAgent -->|Multi-Modal Search| VectorSearch
    UnrealAgent -->|Python Code Generation| Gemini
    ValidationAgent -->|Python Subprocess Dry-Run Sandbox| LocalSandbox[Local Sandbox Subprocess]
    GCSUploader -->|Upload WAV & Python Script| GCS
    PRAgent -->|Commit Script & Create PR| GitHub

    %% Security
    Runner -->|Resolve API Keys & Tokens| SecretManager
    
    %% Telemetry
    ADK -.->|Trace Context Propagation| CloudTrace
    Runner -.->|Trace Context Propagation| CloudTrace
    VectorSearch -.->|Telemetry Metadata| CloudTrace
```

---

## 2. Core System Components & GCP Integration

### 2.1 Central Orchestration
* **Agent Development Kit (ADK)**: Framework managing sequential execution flow across 4 core sub-agents (`unreal_agent`, `validation_agent`, `gcs_uploader`, `pull_request_agent`), intermediate state variables, and context propagation.
* **Agent Runner (`cmd/agent-runner`)**: Bootstrapped inside containerized GKE gVisor runtimes, accepting optional `-audio` file path flags, pre-populating session state with `audio_binary`, resolving secrets from Secret Manager, and executing orchestration workflows.

### 2.2 Large Language Models & Multimodal AI
* **Gemini 3.1 Pro (`gemini-3.1-pro-preview`)**: Core reasoning engine driving Unreal Engine Python script generation, vector query synthesis, and sandbox dry-run auto-correction.
* **Gemini 3.1 Flash**: Deployed for lightweight, latency-critical operations.
* **Vector Search 2.0**: Used as a multi-modal semantic search layer over target C++ source and Blueprint asset collections.

### 2.3 Sandboxed Local Subprocesses
* **Subprocess Sandbox**: Dry-runs generated Unreal Engine 5 Python automation scripts in isolated sub-processes, capturing execution errors, exit codes, and diagnostics for iterative auto-correction.

### 2.4 Delivery & Storage (Cloud Storage)
* **Google Cloud Storage (GCS)**: Stores selected Foley sound assets and validated Python automation scripts under session-keyed paths (`audio/foley_{session_id}.wav` and `scripts/unreal_assist_{session_id}.py`).

### 2.5 Pull Request Agent (Human-in-the-Loop Review)
* **GitHub Pull Request Integration**: Authenticates via GitHub Personal Access Tokens stored in Secret Manager, clones repository branches, commits integration scripts, and opens PRs containing direct asset download links.

---

## 3. Observability & Distributed Tracing

* **OpenTelemetry Context Propagation**: Propagates `TraceContext` and `Baggage` across agent steps under root span `game-assist-workflow`.
* **Google Cloud Trace**: Exports telemetry spans for agent latencies, vector retrieval queries, and sandbox validation runs.

---

## 4. Resource Governance & FinOps Compliance

All provisioned infrastructure and vector collections enforce standardized labels:

| Label | Description | Allowed Values / Pattern |
| :--- | :--- | :--- |
| `environment` | Deployment stage | `dev`, `staging`, `prod` |
| `owner` | Responsible user or team handle | e.g. `ikogan` |
| `cost-center` | Accounting budget allocation | e.g. `gaming-assist-ai` |
| `managed-by` | IaC or automation tool | `terraform`, `vector-indexer` |
