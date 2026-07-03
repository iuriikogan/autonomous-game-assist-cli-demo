# Multi-Agent Sequential Workflow

This document outlines the step-by-step sequential processing workflow of the Autonomous Game Assist CLI system on Google Cloud Platform.

---

## 1. Core Execution Flow

When a game developer triggers the CLI with a high-level request (e.g., `./game-assist generate "Integrate existing metal footstep sound effect on trigger overlap"`), the Central Coordinator orchestrates four cooperative agent steps in sequence to query Blueprint indexes, generate and validate Python integration scripts, stage GCS assets, and deliver PRs.

```mermaid
sequenceDiagram
    autonumber
    actor Dev as Game Developer
    participant CLI as Local CLI (game-assist)
    participant Runner as Agent Runner (GKE Job)
    participant Coordinator as Central Coordinator Agent
    participant UA as Unreal Agent
    participant VA as Validation Agent
    participant VS as Multi-Modal Vector Search Tool
    participant SB as Subprocess Sandbox Tool
    participant Uploader as GCS Uploader Agent
    participant GCS as Cloud Storage
    participant PR as Pull Request Agent
    participant GH as GitHub API

    Dev->>CLI: game-assist generate --prompt "[WAV integration request]"
    activate CLI
    CLI->>Runner: Deploy GKE Sandbox Job
    activate Runner
    Runner->>Coordinator: Start session with audio_binary state
    activate Coordinator
    Note over Coordinator: Start Trace: "game-assist-workflow"

    %% Step 1: Python Code Generation with Multi-Modal Vector Context
    Coordinator->>UA: Trigger UE5 Script Generation
    activate UA
    Note over UA: Start Span: "unreal_agent_run"
    UA->>VS: Query multi-modal index for base Blueprint classes & asset context
    activate VS
    Note over VS: Start Span: "vector_search_tool"
    VS-->>UA: Base class structures & asset paths
    deactivate VS
    UA->>UA: Generate UE5 Python automation script
    UA-->>Coordinator: Python integration script (unreal_script)
    deactivate UA

    %% Step 2: Sandboxed Dry-Run & Verification Validation
    Coordinator->>VA: Trigger Python Validation
    activate VA
    Note over VA: Start Span: "validation_agent_run"
    
    loop Sandboxed Dry-Run Execution (up to 3 attempts)
        VA->>SB: Execute Python dry-run harness in isolated subprocess
        activate SB
        Note over SB: Start Span: "sandbox_execution_tool"
        SB-->>VA: Exit code, stdout/stderr diagnostic logs
        deactivate SB
        alt dry-run fails
            VA->>VA: Rewrite Python script based on stderr/diagnostic logs
        else success
            Note over VA: Dry-run execution PASSED
        end
    end
    
    VA-->>Coordinator: validated_script
    deactivate VA

    %% Step 3: Staging Deliverables to Cloud Storage
    Coordinator->>Uploader: Trigger Staging
    activate Uploader
    Note over Uploader: Start Span: "gcs_uploader_run"
    Uploader->>GCS: Stage target WAV files and Python script
    Uploader-->>Coordinator: Asset & script GCS URIs
    deactivate Uploader

    %% Step 4: Git Branch Creation & Detailed PR Creation
    Coordinator->>PR: Trigger Pull Request
    activate PR
    Note over PR: Start Span: "pull_request_agent_run"
    PR->>PR: Commit script to content/scripts/, prepare PR body
    PR->>GH: POST /repos/{owner}/{repo}/pulls (with GCS asset download links)
    activate GH
    GH-->>PR: PR URL (e.g. github.com/.../pulls/1)
    deactivate GH
    PR-->>Coordinator: PR URL
    deactivate PR

    Coordinator-->>Runner: Workflow Complete
    deactivate Coordinator
    Runner-->>CLI: Job finished
    deactivate Runner
    CLI-->>Dev: Print PR URL & GCS Asset URIs
    deactivate CLI
```

---

## 2. Sub-Agent Operations and State Transitions

### 2.1 Central Coordinator Agent
* **Type**: Orchestrator (`sequentialagent`)
* **State Inputs**: Developer prompt from `game-assist`, `audio_binary` pre-populated in session state by `agent-runner`.
* **Responsibility**: Manages session variables, coordinates 4 sub-agent executions, and propagates OpenTelemetry trace contexts.

### 2.2 Unreal Agent
* **Type**: Integration Coder (`llmagent` using Gemini 3.1 Pro: `gemini-3.1-pro-preview`)
* **Tool**: `vector_search`
* **State Output**: `unreal_script` (Python string)
* **Instruction**: Queries **Vertex AI Vector Search 2.0 Collections** to discover target Level Blueprints and actor classes, generating a Python script to link the selected Foley sound asset to UE5 trigger volumes.

### 2.3 Validation Agent & Auto-Correction Loop
* **Type**: Quality Assurance Coder (`llmagent` using Gemini 3.1 Pro: `gemini-3.1-pro-preview`)
* **Tool**: `sandbox_tool`
* **State Input**: `unreal_script`
* **State Output**: `validated_script` (Python string)
* **Loop Logic**:
  1. Executes script in isolated Python subprocess sandbox.
  2. Evaluates exit status and `stderr`.
  3. Rewrites script upon syntax or API errors (maximum 3 iterations).

### 2.4 GCS Uploader Agent
* **Type**: Storage System Agent (`coordinator.gcsUploader`)
* **State Inputs**: `audio_binary`, `validated_script`
* **State Output**: `gs://` delivery paths (`audio/foley_{session_id}.wav`, `scripts/unreal_assist_{session_id}.py`)
* **Instruction**: Uploads validated script and Foley audio asset under session-keyed storage locations.

### 2.5 Pull Request Agent
* **Type**: Git Integration Agent (`pr.Agent`)
* **State Inputs**: `validated_script`, `audio_gcs_uri`, `script_gcs_uri`, `prompt`
* **State Output**: GitHub Pull Request URL
* **Instruction**: Clones repository, creates feature branch `foley-assist-{session_id}`, commits the script to `content/scripts/`, and dispatches GitHub API call to open PR with direct GCS download links.
