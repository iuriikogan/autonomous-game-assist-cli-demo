package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/iuriikogan/autonomous-game-assist-cli/pkg/k8s"
	"github.com/spf13/cobra"
)

var (
	generateImage   string
	generateUser    string
	generateSession string

	newDispatcher = k8s.NewDispatcher
)

var generateCmd = &cobra.Command{
	Use:   "generate [prompt]",
	Short: "Submit a new game assist task to the GKE sandbox",
	Long:  `Submits a secure Kubernetes Job running the sandboxed agent-runner with the specified prompt, and streams logs to standard output.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		prompt := args[0]
		ctx := cmd.Context()

		if generateSession == "" {
			generateSession = fmt.Sprintf("session-%d", time.Now().Unix())
		}

		// Initialize the K8s dispatcher
		dispatcher, err := newDispatcher()
		if err != nil {
			return fmt.Errorf("failed to initialize Kubernetes client: %w", err)
		}

		jobName := fmt.Sprintf("game-assist-%s", generateSession)
		runnerArgs := []string{
			"--prompt", prompt,
			"--user", generateUser,
			"--session", generateSession,
		}

		fmt.Printf("Submitting job %s with session %s...\n", jobName, generateSession)
		_, err = dispatcher.DispatchJob(ctx, jobName, generateImage, runnerArgs)
		if err != nil {
			return fmt.Errorf("failed to dispatch job: %w", err)
		}
		fmt.Println("Job submitted successfully! Waiting for execution...")

		// Start streaming logs in the background
		logCtx, logCancel := context.WithCancel(ctx)
		defer logCancel()
		logErrChan := make(chan error, 1)

		go func() {
			stream, err := dispatcher.StreamJobLogs(logCtx, jobName)
			if err != nil {
				logErrChan <- err
				return
			}
			if stream != nil {
				defer stream.Close()
				_, err = io.Copy(os.Stdout, stream)
				logErrChan <- err
			} else {
				logErrChan <- nil
			}
		}()

		// Wait for the job to finish
		waitErr := dispatcher.WaitForJob(ctx, jobName)

		// Ensure logs finished streaming or timed out
		select {
		case logErr := <-logErrChan:
			if logErr != nil && logErr != io.EOF {
				fmt.Fprintf(os.Stderr, "Warning: log streaming error: %v\n", logErr)
			}
		case <-time.After(2 * time.Second):
			// Don't block too long if logs are stuck
		}

		if waitErr != nil {
			return fmt.Errorf("job execution failed: %w", waitErr)
		}

		fmt.Println("Job completed successfully!")
		return nil
	},
}

func init() {
	generateCmd.Flags().StringVar(&generateImage, "image", "gcr.io/gaming-assist-ai/agent-runner:latest", "The container image for the agent-runner pod")
	generateCmd.Flags().StringVar(&generateUser, "user", "game_dev_1", "The user ID starting the request")
	generateCmd.Flags().StringVar(&generateSession, "session", "", "The session ID (auto-generated if empty)")
	rootCmd.AddCommand(generateCmd)
}
