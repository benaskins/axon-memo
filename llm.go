package mem

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// extractJSONFromMarkdown strips markdown code fences from LLM responses.
func extractJSONFromMarkdown(content string) string {
	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json")
		if start >= 0 && start+7 < len(content) {
			contentAfterMarker := content[start+7:]
			end := strings.Index(contentAfterMarker, "```")
			if end > 0 {
				return strings.TrimSpace(contentAfterMarker[:end])
			}
			return strings.TrimSpace(contentAfterMarker)
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```")
		if start >= 0 && start+3 < len(content) {
			contentAfterMarker := content[start+3:]
			end := strings.Index(contentAfterMarker, "```")
			if end > 0 {
				return strings.TrimSpace(contentAfterMarker[:end])
			}
			return strings.TrimSpace(contentAfterMarker)
		}
	}
	return content
}

// ExtractMemories calls the TextGenerator with an extraction prompt and parses
// the JSON response into an ExtractionResult.
func ExtractMemories(ctx context.Context, generate TextGenerator, prompt string) (*ExtractionResult, error) {
	responseText, err := generate(ctx, prompt, 0.3, 2048)
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	var result ExtractionResult
	content := extractJSONFromMarkdown(responseText)

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse extraction result: %w (response: %s)", err, responseText)
	}

	return &result, nil
}

// AnalyzeMemories calls the TextGenerator with a consolidation prompt and
// parses the JSON response into a ConsolidationResult.
func AnalyzeMemories(ctx context.Context, generate TextGenerator, prompt string) (*ConsolidationResult, error) {
	responseText, err := generate(ctx, prompt, 0.3, 2048)
	if err != nil {
		return nil, fmt.Errorf("llm chat: %w", err)
	}

	var result ConsolidationResult
	content := extractJSONFromMarkdown(responseText)

	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return nil, fmt.Errorf("parse consolidation result: %w (response: %s)", err, responseText)
	}

	return &result, nil
}

// GeneratePersonalityContext calls the TextGenerator to produce a personality
// context string from a prompt.
func GeneratePersonalityContext(ctx context.Context, generate TextGenerator, prompt string) (string, error) {
	responseText, err := generate(ctx, prompt, 0.7, 512)
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}

	return strings.TrimSpace(responseText), nil
}
