# Multi-Agent Sequential Workflow

This document outlines the step-by-step sequential processing workflow of the Autonomous Game Assist CLI system on Google Cloud Platform.

---

## 1. Core Execution Flow

When a game developer triggers the CLI with a high-level request (e.g., `./game-assist generate "Integrate existing metal footstep sound effect on trigger overlap"`), the Central Coordinator orchestrates six cooperative agent steps in sequence to expand metadata, select audio assets, query Blueprint indexes, validate integration scripts, and deliver PRs.

```mermaid
sequenceDiagram
    autonumber
    actor Dev as Game Developer
    participant CLI as Local CLI (game-assist)
    participant Runner as Agent Runner (GKE Job)
    participant Coordinator as Central Coordinator Agent
    participant PC as Prompt Crafter Agent
    participant UA as Unreal C++ Agent
    participant VA as Validation Agent
    participant VS as Multi-Modal Vector Search Tool
    participant SB as Sandbox Tool (g++ & Smoke Harness)
    participant Uploader as GCS Uploader Agent
    participant GCS as Cloud Storage
    participant PR as Pull Request Agent
    participant GH as GitHub API

    Dev->>CLI: game-assist generate --prompt "[WAV integration request]"
    activate CLI
    CLI->>Runner: Deploy GKE Sandbox Job
    activate Runner
    Runner->>Coordinator: Start execution session
    activate Coordinator
    Note over Coordinator: Start Trace: "game-assist-workflow"

    %% Step 1: Prompt Expansion & Acoustic Metadata
    Coordinator->>PC: Trigger Metadata Expansion
    activate PC
    Note over PC: Start Span: "prompt_crafter_run"
    PC->>PC: Extract WAV asset intent & target level context
    PC-->>Coordinator: Rich level_integration_description
    deactivate PC

    %% Step 2: C++ Code Generation with Multi-Modal Vector Context
    Coordinator->>UA: Trigger C++ Code Gen
    activate UA
    Note over UA: Start Span: "unreal_agent_run"
    UA->>VS: Query multi-modal index for base C++ level classes & WAV metadata
    activate VS
    Note over VS: Start Span: "vector_search_tool"
    VS-->>UA: Base C++ class structure & WAV asset paths
    deactivate VS
    UA->>UA: Write Unreal C++ level actor code additions
    UA-->>Coordinator: C++ code additions (unreal_script)
    deactivate UA

    %% Step 3: Sandboxed Compilation & Mock Smoke Test Validation
    Coordinator->>VA: Trigger C++ Validation
    activate VA
    Note over VA: Start Span: "validation_agent_run"
    
    loop Sandboxed C++ Compile & Smoke Test (up to 3 attempts)
        VA->>SB: Execute g++ compilation & mock smoke test harness
        activate SB
        Note over SB: Start Span: "sandbox_execution_tool"
        SB-->>VA: Exit code, stdout/smoke test results, stderr
        deactivate SB
        alt compile / smoke test fails
            VA->>VA: Rewrite C++ code additions based on stderr/test logs
        else success
            Note over VA: All 3/3 mock smoke tests PASSED
        end
    end
    
    VA-->>Coordinator: validated_script
    deactivate VA

    %% Step 4: Staging Deliverables to Cloud Storage
    Coordinator->>Uploader: Trigger Staging
    activate Uploader
    Note over Uploader: Start Span: "gcs_uploader_run"
    Uploader->>GCS: Stage target WAV files and metadata
    Uploader-->>Coordinator: Asset GCS URIs
    deactivate Uploader

    %% Step 5: Git Diff Application & Detailed PR Comment Creation
    Coordinator->>PR: Trigger Pull Request
    activate PR
    Note over PR: Start Span: "pull_request_agent_run"
    PR->>PR: Apply C++ diff to target class, compute git diff
    PR->>GH: POST /repos/{owner}/{repo}/pulls (with detailed C++ PR comment & git diff)
```    activate GH
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

## 2. Sub-Agent Operations and State Transitions

### 2.1 Central Coordinator Agent
* **Type**: Orchestrator (`sequentialagent`)
* **State Inputs**: Developer natural language prompt from `game-assist`.
* **Responsibility**: Manages session variables, coordinates sub-agent execution, and propagates OpenTelemetry trace contexts.

### 2.2 Prompt Crafter Agent
* **Type**: Reasoner (`llmagent` using Gemini 3.1 Pro)
* **State Output**: `foley_description` (string)
* **Instruction**: Expands developer request into detailed acoustic dynamics, materials (steel, sand, foliage), impact velocities, and reverberation metadata.

### 2.3 Audio Asset Selector
* **Type**: Asset Resolver (`coordinator.assetSelector`)
* **State Input**: `foley_description`
* **State Output**: `audio_asset_uri` (string)
* **Instruction**: Identifies and selects the best matching pre-existing Foley sound asset from the central sound repository.

### 2.4 Unreal Agent
* **Type**: Integration Coder (`llmagent` using Gemini 3.1 Pro)
* **Tool**: `vector_search`
* **State Output**: `unreal_script` (Python string)
* **Instruction**: Queries **Vertex AI Vector Search 2.0 Collections** to discover target Level Blueprints and actor classes, generating a Python script to link the selected Foley sound asset to UE5 trigger volumes.

### 2.5 Validation Agent & Auto-Correction Loop
* **Type**: Quality Assurance Coder (`llmagent` using Gemini 3.1 Pro)
* **Tool**: `sandbox_tool`
* **State Input**: `unreal_script`
* **State Output**: `validated_script` (Python string)
* **Loop Logic**:
  1. Executes script in isolated subprocess sandbox.
  2. Evaluates exit status and `stderr`.
  3. Rewrites script upon syntax or API errors (maximum 3 iterations).

### 2.6 GCS Uploader Agent
* **Type**: Storage System Agent (`coordinator.gcsUploader`)
* **State Inputs**: `audio_asset_uri`, `validated_script`
* **State Output**: `gs://` delivery paths
* **Instruction**: Uploads validated script and Foley audio asset under session-keyed storage locations.

### 2.7 Pull Request Agent
* **Type**: Git Integration Agent (`pr.Agent`)
* **State Inputs**: `validated_script`, `audio_gcs_uri`, `script_gcs_uri`, `prompt`
* **State Output**: GitHub Pull Request URL
* **Instruction**: Clones repository, creates feature branch `foley-assist-{session_id}`, commits the script to `content/scripts/`, and dispatches GitHub API call to open PR with direct GCS download links.
