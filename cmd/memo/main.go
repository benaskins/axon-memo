package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:     "memo",
	Short:   "CLI for axon-memo — store and recall agent memories",
	Version: version,
}

func init() {
	rootCmd.PersistentFlags().String("url", "", "axon-memo service URL (default: MEMO_URL env or http://localhost:8086)")
}

func baseURL(cmd *cobra.Command) string {
	if u, _ := cmd.Flags().GetString("url"); u != "" {
		return u
	}
	if u := os.Getenv("MEMO_URL"); u != "" {
		return u
	}
	return "http://localhost:8086"
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
