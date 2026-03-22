package main

import (
	"log"
	"os"

	"github.com/dpanel-dev/installer/internal/ui/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "dpanel-installer",
	Short: "DPanel Installer in Go",
	Run: func(cmd *cobra.Command, args []string) {
		// Default to TUI if no flags provided
		if err := tui.StartTUI(); err != nil {
			log.Fatalf("TUI Error: %v", err)
		}
	},
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
