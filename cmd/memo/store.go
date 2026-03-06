package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	memo "github.com/benaskins/axon-memo"
	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "store [content]",
		Short: "Store a memory",
		Long: `Store a memory directly. Use --durable for facts, principles, and procedures
that should not decay over time.

Examples:
  memo store "axon-memo uses Mayer's ABI trust model"
  memo store --durable --type semantic "pull-based coordination, not assignment"
  memo store --type episodic "refactored relationship dimensions to ABI model"`,
		Args: cobra.ExactArgs(1),
		RunE: runStore,
	}

	cmd.Flags().String("agent", "", "agent slug (default: MEMO_AGENT env or 'claude-code')")
	cmd.Flags().String("user", "", "user ID (default: MEMO_USER env or 'default')")
	cmd.Flags().String("type", "semantic", "memory type: episodic, semantic, emotional")
	cmd.Flags().Float64("importance", 0.7, "importance score 0.0-1.0")
	cmd.Flags().Bool("durable", false, "mark as durable (no recency decay)")

	rootCmd.AddCommand(cmd)
}

func runStore(cmd *cobra.Command, args []string) error {
	agent, _ := cmd.Flags().GetString("agent")
	if agent == "" {
		agent = envOrDefault("MEMO_AGENT", "claude-code")
	}

	user, _ := cmd.Flags().GetString("user")
	if user == "" {
		user = envOrDefault("MEMO_USER", "default")
	}

	memType, _ := cmd.Flags().GetString("type")
	importance, _ := cmd.Flags().GetFloat64("importance")
	durable, _ := cmd.Flags().GetBool("durable")

	req := memo.StoreRequest{
		AgentSlug:  agent,
		UserID:     user,
		MemoryType: memType,
		Content:    args[0],
		Importance: importance,
		Durable:    durable,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := baseURL(cmd) + "/api/memory/store"
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("store failed (%d): %s", resp.StatusCode, respBody)
	}

	var result memo.StoreResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}

	durableLabel := ""
	if result.Durable {
		durableLabel = " (durable)"
	}
	fmt.Printf("stored %s%s: %s\n", result.ID, durableLabel, args[0])

	return nil
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
