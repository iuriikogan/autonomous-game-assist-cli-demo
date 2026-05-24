package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "game-assist",
	Short: "game-assist is a local CLI for the Autonomous Game Assist Agent",
	Long:  `A CLI to dispatch game assistance agents to GKE and download generated assets.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
