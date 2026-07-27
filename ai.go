package main

import (
	"bytes"
	stdctx "context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const fixPointSystemPrompt = `You are FixPoint, an expert software engineer and debugger. You are given a stack trace, local variables, and a code snippet from a breakpoint. Provide a detailed diagnosis of why execution stopped, identify the likely root cause, and propose a concrete code fix with rationale.

Respond using these sections:
1) What Happened — a concise summary of the state when execution paused.
2) Root Cause — the most likely underlying bug or logic error.
3) Evidence From Context — specific variable values, frame info, or code patterns that support your conclusion.
4) Proposed Fix — a clear, actionable code change with explanation.
5) Validation Steps — how to verify the fix works.`

func GetFixFromAI(ctx *DebugContext) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	model := getOpenRouterModel()

	if apiKey == "" {
		return "", fmt.Errorf("missing OPENROUTER_API_KEY")
	}

	userPrompt := buildUserPrompt(ctx)

	messages := []map[string]string{
		{"role": "system", "content": fixPointSystemPrompt},
		{"role": "user", "content": userPrompt + "\n\nReturn a complete answer with specific code-level recommendations. If data is missing, state assumptions clearly."},
	}

	requestBody := map[string]any{
		"model":    model,
		"messages": messages,
		"temperature": 0.2,
		"max_tokens":  1600,
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	payloadSizeKB := float64(len(payload)) / 1024.0
	infoMsg := fmt.Sprintf("Sending AI request (Payload: %.2f KB, Model: %s)...", payloadSizeKB, model)
	fmt.Println(RenderInfo(infoMsg))

	reqCtx, cancel := stdctx.WithTimeout(stdctx.Background(), 90*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("HTTP-Referer", "https://github.com/anomalyco/fixpoint")
	req.Header.Set("X-Title", "FixPoint")

	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		errText := strings.TrimSpace(string(responseBytes))
		return "", fmt.Errorf("OpenRouter API error (%d): %s", resp.StatusCode, errText)
	}

	var apiResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error,omitempty"`
	}

	if err := json.Unmarshal(responseBytes, &apiResp); err != nil {
		return "", err
	}

	if apiResp.Error != nil && apiResp.Error.Message != "" {
		return "", fmt.Errorf("OpenRouter API error: %s", apiResp.Error.Message)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("OpenRouter API returned empty choices")
	}

	content := strings.TrimSpace(apiResp.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("OpenRouter API returned empty response")
	}

	return content, nil
}

func getOpenRouterModel() string {
	model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if model == "" {
		return "google/gemini-2.5-flash"
	}
	return model
}

func buildUserPrompt(ctx *DebugContext) string {
	var b strings.Builder

	b.WriteString("Debug Context Report\n")
	b.WriteString("====================\n\n")
	b.WriteString(fmt.Sprintf("Reason: %s\n", ctx.Reason))
	b.WriteString(fmt.Sprintf("Thread ID: %d\n", ctx.ThreadID))
	b.WriteString(fmt.Sprintf("Frame ID: %d\n", ctx.FrameID))
	b.WriteString(fmt.Sprintf("Source: %s:%d\n\n", ctx.SourcePath, ctx.SourceLine))

	b.WriteString("Stack Trace:\n")
	for i, frame := range ctx.StackTrace {
		b.WriteString(fmt.Sprintf("%d. %s (%s:%d:%d) [frameId=%d]\n", i+1, frame.Name, frame.SourcePath, frame.Line, frame.Column, frame.ID))
	}
	b.WriteString("\n")

	b.WriteString("Local Variables:\n")
	for _, variable := range ctx.Variables {
		if variable.Type != "" {
			b.WriteString(fmt.Sprintf("- %s (%s) = %s\n", variable.Name, variable.Type, variable.Value))
			continue
		}
		b.WriteString(fmt.Sprintf("- %s = %s\n", variable.Name, variable.Value))
	}
	b.WriteString("\n")

	b.WriteString("Source Window:\n")
	for _, line := range ctx.SourceSnippet {
		marker := " "
		if line.LineNumber == ctx.SourceLine {
			marker = ">"
		}
		b.WriteString(fmt.Sprintf("%s %4d | %s\n", marker, line.LineNumber, line.Text))
	}

	return b.String()
}
