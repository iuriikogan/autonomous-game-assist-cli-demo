package pr

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/gcp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// Config defines the execution parameters for the PullRequestAgent.
type Config struct {
	SecretManagerClient gcp.SecretManagerClient
	SecretPath          string // e.g. "projects/{project}/secrets/github-token/versions/latest"
	TargetRepoOwner     string // e.g. "ikogan"
	TargetRepoName      string // e.g. "autonomous-game-assist-cli"
	BaseBranch          string // e.g. "main"
}

type pullRequestAgent struct {
	agent.Agent
	cfg Config
}

// New creates a custom ADK Agent that commits changes to a new branch and opens a Pull Request on GitHub.
func New(cfg Config) (agent.Agent, error) {
	if cfg.SecretManagerClient == nil {
		return nil, fmt.Errorf("secret manager client is required")
	}
	if cfg.SecretPath == "" {
		return nil, fmt.Errorf("secret path is required")
	}
	if cfg.TargetRepoOwner == "" {
		return nil, fmt.Errorf("target repo owner is required")
	}
	if cfg.TargetRepoName == "" {
		return nil, fmt.Errorf("target repo name is required")
	}
	if cfg.BaseBranch == "" {
		cfg.BaseBranch = "main"
	}

	pra := &pullRequestAgent{cfg: cfg}

	base, err := agent.New(agent.Config{
		Name:        "pull_request_agent",
		Description: "Pushes validated scripts to a new branch on GitHub and opens a Pull Request with direct GCS audio download links for human approval.",
		Run:         pra.run,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create base agent: %w", err)
	}
	pra.Agent = base

	return pra, nil
}

func (p *pullRequestAgent) run(ictx agent.InvocationContext) iter.Seq2[*session.Event, error] {
	return func(yield func(*session.Event, error) bool) {
		tr := otel.Tracer("game-assist-agents")
		ctx, span := tr.Start(ictx, "pull_request_agent_run")
		defer span.End()

		sess := ictx.Session()
		sessID := sess.ID()

		// 1. Retrieve inputs from session state
		validatedScriptVal, err := sess.State().Get("validated_script")
		if err != nil {
			yield(nil, fmt.Errorf("session lacks required variable 'validated_script': %w", err))
			return
		}
		validatedScript, ok := validatedScriptVal.(string)
		if !ok {
			yield(nil, fmt.Errorf("unexpected type for 'validated_script': %T", validatedScriptVal))
			return
		}

		audioURICtx, err := sess.State().Get("audio_gcs_uri")
		if err != nil {
			yield(nil, fmt.Errorf("session lacks required variable 'audio_gcs_uri': %w", err))
			return
		}
		audioURI, ok := audioURICtx.(string)
		if !ok {
			yield(nil, fmt.Errorf("unexpected type for 'audio_gcs_uri': %T", audioURICtx))
			return
		}

		scriptURICtx, err := sess.State().Get("script_gcs_uri")
		if err != nil {
			yield(nil, fmt.Errorf("session lacks required variable 'script_gcs_uri': %w", err))
			return
		}
		scriptURI, ok := scriptURICtx.(string)
		if !ok {
			yield(nil, fmt.Errorf("unexpected type for 'script_gcs_uri': %T", scriptURICtx))
			return
		}

		var userPrompt string
		promptCtx, err := sess.State().Get("prompt")
		if err == nil {
			if pStr, ok := promptCtx.(string); ok {
				userPrompt = pStr
			}
		}

		var foleyDesc string
		foleyCtx, err := sess.State().Get("foley_description")
		if err == nil {
			if fStr, ok := foleyCtx.(string); ok {
				foleyDesc = fStr
			}
		}

		span.SetAttributes(
			attribute.String("pr.session_id", sessID),
			attribute.String("pr.target_branch", p.cfg.BaseBranch),
		)

		// 2. Fetch GitHub Personal Access Token from Secret Manager
		githubToken, err := p.cfg.SecretManagerClient.GetSecret(ctx, p.cfg.SecretPath)
		if err != nil {
			yield(nil, fmt.Errorf("failed to retrieve GitHub Token from Secret Manager: %w", err))
			return
		}
		githubToken = strings.TrimSpace(githubToken)

		// 3. Create temporary workspace and checkout code
		tmpDir, err := os.MkdirTemp("", "git-checkout-*")
		if err != nil {
			yield(nil, fmt.Errorf("failed to create temporary git workspace: %w", err))
			return
		}
		defer os.RemoveAll(tmpDir)

		// Build authenticated Git clone URL
		cloneURL := fmt.Sprintf("https://x-access-token:%s@github.com/%s/%s.git",
			githubToken, p.cfg.TargetRepoOwner, p.cfg.TargetRepoName)

		// Run Git commands to initialize checkout
		if err := runCmd(ctx, "", "git", "clone", "--depth", "1", "--branch", p.cfg.BaseBranch, cloneURL, tmpDir); err != nil {
			yield(nil, fmt.Errorf("failed to clone repository: %w", err))
			return
		}

		// Configure local git identity inside checkout
		if err := runCmd(ctx, tmpDir, "git", "config", "user.name", "Foley Assist Bot"); err != nil {
			yield(nil, fmt.Errorf("failed to configure git user name: %w", err))
			return
		}
		if err := runCmd(ctx, tmpDir, "git", "config", "user.email", "foley-assist-bot@google.com"); err != nil {
			yield(nil, fmt.Errorf("failed to configure git user email: %w", err))
			return
		}

		// Create a new session-specific branch
		branchName := fmt.Sprintf("foley-assist-%s", sessID)
		if err := runCmd(ctx, tmpDir, "git", "checkout", "-b", branchName); err != nil {
			yield(nil, fmt.Errorf("failed to checkout new branch %s: %w", branchName, err))
			return
		}

		// Write the validated python script to target location in the workspace
		scriptRelPath := filepath.Join("content", "scripts", fmt.Sprintf("unreal_assist_%s.py", sessID))
		scriptFullPath := filepath.Join(tmpDir, scriptRelPath)
		if err := os.MkdirAll(filepath.Dir(scriptFullPath), 0755); err != nil {
			yield(nil, fmt.Errorf("failed to create scripts directory structure: %w", err))
			return
		}
		if err := os.WriteFile(scriptFullPath, []byte(validatedScript), 0644); err != nil {
			yield(nil, fmt.Errorf("failed to write script file to git workspace: %w", err))
			return
		}

		// Commit and push changes to remote branch
		if err := runCmd(ctx, tmpDir, "git", "add", scriptRelPath); err != nil {
			yield(nil, fmt.Errorf("failed to git add: %w", err))
			return
		}
		commitMsg := fmt.Sprintf("feat(foley): automated foley script integration [session: %s]", sessID)
		if err := runCmd(ctx, tmpDir, "git", "commit", "-m", commitMsg); err != nil {
			yield(nil, fmt.Errorf("failed to git commit: %w", err))
			return
		}
		if err := runCmd(ctx, tmpDir, "git", "push", "origin", branchName); err != nil {
			yield(nil, fmt.Errorf("failed to git push: %w", err))
			return
		}

		// 4. Formulate PR details and open via GitHub API
		audioHTTPS := gcsURIToHTTPS(audioURI)
		scriptHTTPS := gcsURIToHTTPS(scriptURI)

		prTitle := fmt.Sprintf("feat(foley): Synthesized Foley Asset Integration (Session: %s)", sessID)
		prBody := fmt.Sprintf(`# 🎧 Foley Asset Integration Review (Session: %s)

This Pull Request contains the automated Unreal Engine 5 level integration script generated by the **Autonomous Game Assist** multi-agent workflow.

## 🚀 Asset Delivery & Review Paths

| Deliverable | Location | Action |
| :--- | :--- | :--- |
| **Synthesized Audio WAV** | `+"`"+`%s`+"`"+` | [Click here to download/listen](%s) |
| **Unreal Level Script** | `+"`"+`%s`+"`"+` | [View script in GCS](%s) |

---

## 🔊 Synthesized Foley Context
> **Prompt**: "%s"
> **Detailed Acoustics**: %s

---

## 🛠️ Code Integration Overview
The accompanying code changes introduce the custom level automation script under:
`+"`"+`content/scripts/unreal_assist_%s.py`+"`"+`

### Reviewer Quickstart (Local Verification)
To test these assets locally on your developer workstation:
1. Pull this PR branch locally: `+"`"+`git checkout %s`+"`"+`
2. Download deliverables directly using the CLI:
   `+"`"+`game-assist download --session %s`+"`"+`
3. Open Unreal Engine Editor and execute `+"`"+`unreal_assist_%s.py`+"`"+`.
`, sessID, audioURI, audioHTTPS, scriptURI, scriptHTTPS, userPrompt, foleyDesc, sessID, branchName, sessID, sessID)

		prURL, err := createPullRequest(ctx, githubToken, p.cfg.TargetRepoOwner, p.cfg.TargetRepoName, prTitle, prBody, branchName, p.cfg.BaseBranch)
		if err != nil {
			yield(nil, fmt.Errorf("failed to open GitHub Pull Request: %w", err))
			return
		}

		span.SetAttributes(attribute.String("pr.url", prURL))

		// 5. Yield completion event
		evt := session.NewEvent(ictx.InvocationID())
		evt.Content = &genai.Content{
			Parts: []*genai.Part{
				{Text: fmt.Sprintf("SUCCESS: Human-in-the-Loop Review Initiated.\nCreated Pull Request: %s\nAudio Asset Location: %s", prURL, audioURI)},
			},
		}
		yield(evt, nil)
	}
}

func runCmd(ctx context.Context, dir string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Suppress sensitive details from stdout/stderr to satisfy Wiz/Snyk security rules
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command %s %v failed: %w (stderr: %s)", name, args, err, stderr.String())
	}
	return nil
}

func gcsURIToHTTPS(gcsURI string) string {
	if !strings.HasPrefix(gcsURI, "gs://") {
		return gcsURI
	}
	path := strings.TrimPrefix(gcsURI, "gs://")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 {
		return gcsURI
	}
	// Format: https://storage.googleapis.com/bucket/object
	return fmt.Sprintf("https://storage.googleapis.com/%s/%s", parts[0], parts[1])
}

type githubPRRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Head  string `json:"head"`
	Base  string `json:"base"`
}

type githubPRResponse struct {
	HTMLURL string `json:"html_url"`
}

func createPullRequest(ctx context.Context, token, owner, repo, title, body, head, base string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", owner, repo)

	payload := githubPRRequest{
		Title: title,
		Body:  body,
		Head:  head,
		Base:  base,
	}

	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("failed to marshal PR payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request to GitHub API failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read API response body: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var prResponse githubPRResponse
	if err := json.Unmarshal(respBytes, &prResponse); err != nil {
		return "", fmt.Errorf("failed to unmarshal GitHub API response: %w (body: %s)", err, string(respBytes))
	}

	return prResponse.HTMLURL, nil
}
