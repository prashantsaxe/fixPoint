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

const fixPointSystemPrompt = "You are FixPoint, an expert Go debugger. You are given a stack trace, local variables, and a code snippet from a breakpoint. Provide a detailed diagnosis of why execution stopped, identify the likely root cause, and propose a concrete code fix with rationale. Respond using sections: 1) What Happened, 2) Root Cause, 3) Evidence From Context, 4) Proposed Fix, 5) Validation Steps."
const defaultGeminiModel = "gemini-flash-latest"

func GetFixFromAI(ctx *DebugContext, apiKey string) (string, error) {
	if strings.TrimSpace(apiKey) == "" {
		return "", fmt.Errorf("missing API key")
	}

	userPrompt := buildUserPrompt(ctx)
	fullPrompt := fixPointSystemPrompt + "\n\n" + userPrompt
	model := getGeminiModel()

	requestBody := map[string]any{
		"contents": []map[string]any{
			{
				"parts": []map[string]string{
					{"text": fullPrompt + "\n\nReturn a complete answer with specific code-level recommendations. If data is missing, state assumptions clearly."},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.2,
			"maxOutputTokens": 1400,
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return "", err
	}

	reqCtx, cancel := stdctx.WithTimeout(stdctx.Background(), 30*time.Second)
	defer cancel()

	endpoint := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent", model)
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-goog-api-key", apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
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
		if resp.StatusCode == http.StatusTooManyRequests {
			return "", fmt.Errorf("Gemini free-tier quota/rate limit hit (429): %s", errText)
		}
		return "", fmt.Errorf("AI API error (%d): %s", resp.StatusCode, errText)
	}

	var apiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.Unmarshal(responseBytes, &apiResp); err != nil {
		return "", err
	}

	if len(apiResp.Candidates) == 0 || len(apiResp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("AI API returned empty response")
	}

	content := strings.TrimSpace(apiResp.Candidates[0].Content.Parts[0].Text)
	if content == "" {
		return "", fmt.Errorf("AI API returned empty response")
	}

	return content, nil
}

func getGeminiModel() string {
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		return defaultGeminiModel
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
