// AG-UI Client Example
//
// This example demonstrates how to connect to an AG-UI server using SSE streaming.
// It shows how to send messages and receive events from the agent.
//
// Usage:
//
//	go run . [--endpoint http://localhost:8080/agui/sse]
//
// The client reads prompts from stdin and streams agent responses via SSE.
// Type your message and press Enter. Use Ctrl+D to exit or type 'quit'.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	defaultEndpoint    = "http://127.0.0.1:9999/agui/sse"
	requestTimeout     = 5 * time.Minute
	readTimeout        = 5 * time.Minute
	stdinBufferInitial = 64 * 1024
	stdinBufferMax     = 1 << 20
)

// errStreamComplete is returned when RUN_FINISHED or RUN_ERROR is received so we
// stop reading immediately instead of blocking until context timeout.
var errStreamComplete = errors.New("stream complete")

func main() {
	// Custom usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: agui-client [flags]\n\n")
		fmt.Fprintf(os.Stderr, "A client for testing AG-UI server SSE streaming functionality.\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Connect to local AG-UI server\n")
		fmt.Fprintf(os.Stderr, "  agui-client -endpoint http://localhost:9999/agui/sse\n\n")
		fmt.Fprintf(os.Stderr, "  # Connect to remote AG-UI server\n")
		fmt.Fprintf(os.Stderr, "  agui-client -endpoint http://your-server:9999/agui/sse\n")
	}

	endpoint := flag.String("endpoint", defaultEndpoint, "AG-UI SSE endpoint")
	flag.Parse()

	if err := runInteractive(*endpoint); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runInteractive(endpoint string) error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, stdinBufferInitial), stdinBufferMax)

	fmt.Printf("AG-UI Client Example\n")
	fmt.Printf("Endpoint: %s\n", endpoint)
	fmt.Println("Type your prompt and press Enter (Ctrl+D to exit, or type 'quit').")
	fmt.Println()

	for {
		fmt.Print("You> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil && !errors.Is(err, context.Canceled) {
				return fmt.Errorf("read input: %w", err)
			}
			fmt.Println()
			return nil
		}
		prompt := strings.TrimSpace(scanner.Text())
		if prompt == "" {
			continue
		}
		if strings.EqualFold(prompt, "quit") || strings.EqualFold(prompt, "exit") {
			return nil
		}
		if err := streamConversation(endpoint, prompt); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
		}
		fmt.Println()
	}
}

func streamConversation(endpoint, prompt string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	// Crush uses dual endpoints: GET /agui/sse for streaming, POST /agui/run to trigger
	runURL := getRunURL(endpoint)
	// Use unique thread per run to avoid accumulated context exceeding API limits (~20MB)
	threadID := fmt.Sprintf("demo-thread-%d", time.Now().Unix())
	runID := fmt.Sprintf("run-%d", time.Now().UnixNano())

	// Prepare the run payload
	payload := map[string]any{
		"threadId": threadID,
		"runId":    runID,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	bodyBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// 1. Connect to SSE stream (GET) - must be established before triggering run
	sseURL := endpoint
	if idx := strings.Index(endpoint, "?"); idx == -1 {
		sseURL = endpoint + "?threadId=" + threadID + "&runId=" + runID
	} else {
		sseURL = endpoint + "&threadId=" + threadID + "&runId=" + runID
	}

	req, err := http.NewRequestWithContext(ctx, "GET", sseURL, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	client := &http.Client{Timeout: readTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to SSE: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("SSE error: %s - %s", resp.Status, string(body))
	}

	// 2. Trigger run (POST) in goroutine - events will flow through SSE
	go func() {
		runReq, _ := http.NewRequestWithContext(ctx, "POST", runURL, bytes.NewReader(bodyBytes))
		runReq.Header.Set("Content-Type", "application/json")
		if _, err := client.Do(runReq); err != nil {
			fmt.Fprintf(os.Stderr, "trigger run: %v\n", err)
		}
	}()

	// 3. Read SSE stream
	return readSSEStream(resp.Body)
}

// getRunURL derives the run endpoint from SSE endpoint. e.g. .../agui/sse -> .../agui/run
func getRunURL(sseEndpoint string) string {
	sseEndpoint = strings.Split(sseEndpoint, "?")[0]
	sseEndpoint = strings.TrimSuffix(sseEndpoint, "/")
	if strings.HasSuffix(sseEndpoint, "/sse") {
		return strings.TrimSuffix(sseEndpoint, "sse") + "run"
	}
	return sseEndpoint + "/run"
}

func readSSEStream(r io.Reader) error {
	reader := bufio.NewReader(r)
	var buffer strings.Builder

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("read stream: %w", err)
		}

		line = strings.TrimSuffix(line, "\n")
		line = strings.TrimSuffix(line, "\r")

		if line == "" {
			// Empty line marks end of SSE frame
			if buffer.Len() > 0 {
				if err := processSSEFrame(buffer.String()); err != nil {
					if errors.Is(err, errStreamComplete) {
						return nil
					}
					fmt.Fprintf(os.Stderr, "parse event: %v\n", err)
				}
				buffer.Reset()
			}
			continue
		}

		if strings.HasPrefix(line, "data: ") {
			buffer.WriteString(strings.TrimPrefix(line, "data: "))
		}
	}

	// Process any remaining data
	if buffer.Len() > 0 {
		if err := processSSEFrame(buffer.String()); err != nil {
			fmt.Fprintf(os.Stderr, "parse event: %v\n", err)
		}
	}

	return nil
}

func processSSEFrame(data string) error {
	if strings.TrimSpace(data) == "" {
		return nil
	}

	// Parse JSON event
	var event map[string]any
	if err := json.Unmarshal([]byte(data), &event); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	// Extract event type
	eventType, _ := event["type"].(string)
	if eventType == "" {
		return nil
	}

	// Format and print the event
	lines := formatEvent(eventType, event)
	for _, line := range lines {
		fmt.Println(line)
	}

	// Stop reading when run ends; avoids blocking until context timeout
	if eventType == "RUN_FINISHED" || eventType == "RUN_ERROR" {
		return errStreamComplete
	}
	return nil
}

func getEventData(event map[string]any) map[string]any {
	if data, ok := event["data"].(map[string]any); ok {
		return data
	}
	return event
}

func formatEvent(eventType string, event map[string]any) []string {
	label := fmt.Sprintf("[%s]", eventType)
	data := getEventData(event)

	switch eventType {
	case "RUN_STARTED":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "RUN_FINISHED":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "RUN_ERROR":
		msg, _ := data["error"].(string)
		if msg == "" {
			msg, _ = event["error"].(string)
		}
		return []string{fmt.Sprintf("Agent> %s: %s", label, msg)}

	case "TEXT_MESSAGE_START":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "TEXT_MESSAGE_CONTENT":
		content, _ := data["content"].(string)
		if content == "" {
			content, _ = event["content"].(string)
		}
		if strings.TrimSpace(content) == "" {
			return nil
		}
		return []string{fmt.Sprintf("Agent> %s %s", label, content)}

	case "TEXT_MESSAGE_END":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "TOOL_CALL_START":
		name, _ := data["name"].(string)
		toolCallID, _ := data["toolCallId"].(string)
		return []string{fmt.Sprintf("Agent> %s tool '%s' started, id: %s", label, name, toolCallID)}

	case "TOOL_CALL_ARGS":
		args, _ := data["args"].(string)
		return []string{fmt.Sprintf("Agent> %s args: %s", label, args)}

	case "TOOL_CALL_END":
		toolCallID, _ := data["toolCallId"].(string)
		return []string{fmt.Sprintf("Agent> %s tool call completed, id: %s", label, toolCallID)}

	case "TOOL_CALL_RESULT":
		result, _ := data["result"].(string)
		if result == "" {
			r, _ := json.Marshal(data["result"])
			result = string(r)
		}
		return []string{fmt.Sprintf("Agent> %s result: %s", label, result)}

	case "STEP_STARTED":
		stepType, _ := data["type"].(string)
		return []string{fmt.Sprintf("Agent> %s step '%s' started", label, stepType)}

	case "STEP_FINISHED":
		stepType, _ := data["type"].(string)
		return []string{fmt.Sprintf("Agent> %s step '%s' finished", label, stepType)}

	case "STATE_DELTA":
		delta, _ := data["delta"].(map[string]any)
		deltaJSON, _ := json.Marshal(delta)
		return []string{fmt.Sprintf("Agent> %s %s", label, string(deltaJSON))}

	case "ACTIVITY_START":
		name, _ := data["name"].(string)
		return []string{fmt.Sprintf("Agent> %s activity '%s' started", label, name)}

	case "ACTIVITY_END":
		return []string{fmt.Sprintf("Agent> %s activity completed", label)}

	case "ACTIVITY_UPDATE":
		status, _ := data["status"].(string)
		switch p := data["progress"].(type) {
		case float64:
			if p > 0 {
				return []string{fmt.Sprintf("Agent> %s %s (%.0f%%)", label, status, p)}
			}
		case int:
			if p > 0 {
				return []string{fmt.Sprintf("Agent> %s %s (%d%%)", label, status, p)}
			}
		}
		return []string{fmt.Sprintf("Agent> %s %s", label, status)}

	case "CUSTOM":
		name, _ := data["type"].(string)
		value, _ := data["payload"].(map[string]any)
		valueJSON, _ := json.Marshal(value)
		return []string{fmt.Sprintf("Agent> %s '%s': %s", label, name, string(valueJSON))}

	default:
		dataJSON, _ := json.Marshal(event["data"])
		return []string{fmt.Sprintf("Agent> %s %s", label, string(dataJSON))}
	}
}
