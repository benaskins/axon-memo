package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	memo "github.com/benaskins/axon-memo"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "recall [query]",
		Short: "Recall memories by semantic search",
		Long: `Retrieve memories relevant to a query using semantic similarity.

Examples:
  memo recall "axon-memo architecture"
  memo recall "trust model" --limit 10
  memo recall "deployment process" --agent aurelia-bot`,
		Args: cobra.ExactArgs(1),
		RunE: runRecall,
	}

	cmd.Flags().String("agent", "", "agent slug (default: MEMO_AGENT env or 'claude-code')")
	cmd.Flags().String("user", "", "user ID (default: MEMO_USER env or 'default')")
	cmd.Flags().Int("limit", 5, "max memories to return")
	cmd.Flags().Bool("json", false, "output as JSON")

	rootCmd.AddCommand(cmd)
}

func runRecall(cmd *cobra.Command, args []string) error {
	agent, _ := cmd.Flags().GetString("agent")
	if agent == "" {
		agent = envOrDefault("MEMO_AGENT", "claude-code")
	}

	user, _ := cmd.Flags().GetString("user")
	if user == "" {
		user = envOrDefault("MEMO_USER", "default")
	}

	limit, _ := cmd.Flags().GetInt("limit")
	jsonOutput, _ := cmd.Flags().GetBool("json")

	params := url.Values{
		"agent": {agent},
		"user":  {user},
		"query": {args[0]},
		"limit": {strconv.Itoa(limit)},
	}

	reqURL := baseURL(cmd) + "/api/memory/recall?" + params.Encode()
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(reqURL)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("recall failed (%d): %s", resp.StatusCode, body)
	}

	var result memo.RecallResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	if len(result.Memories) == 0 {
		fmt.Println("no memories found")
		return nil
	}

	for i, m := range result.Memories {
		fmt.Printf("%d. [%s] (%.2f) %s\n", i+1, m.Type, m.RelevanceScore, m.Content)
		if m.EmotionalContext != "" {
			fmt.Printf("   emotions: %s\n", m.EmotionalContext)
		}
	}

	return nil
}
