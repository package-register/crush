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
	defaultEndpoint    = "http://127.0.0.1:8080/agui/sse"
	requestTimeout     = 2 * time.Minute
	readTimeout        = 5 * time.Minute
	stdinBufferInitial = 64 * 1024
	stdinBufferMax     = 1 << 20
)

func main() {
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

	client := &http.Client{
		Timeout: readTimeout,
	}

	// Prepare the request payload
	payload := map[string]any{
		"threadId": "demo-thread",
		"runId":    fmt.Sprintf("run-%d", time.Now().UnixNano()),
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	// Create POST request
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	// Set query parameters
	q := req.URL.Query()
	q.Set("threadId", payload["threadId"].(string))
	q.Set("runId", payload["runId"].(string))
	req.URL.RawQuery = q.Encode()

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	// Send request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("connect to server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server error: %s - %s", resp.Status, string(body))
	}

	// Read SSE stream
	return readSSEStream(resp.Body)
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

	return nil
}

func formatEvent(eventType string, event map[string]any) []string {
	label := fmt.Sprintf("[%s]", eventType)

	switch eventType {
	case "RUN_STARTED":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "RUN_FINISHED":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "RUN_ERROR":
		msg, _ := event["error"].(string)
		return []string{fmt.Sprintf("Agent> %s: %s", label, msg)}

	case "TEXT_MESSAGE_START":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "TEXT_MESSAGE_CONTENT":
		content, _ := event["content"].(string)
		if strings.TrimSpace(content) == "" {
			return nil
		}
		return []string{fmt.Sprintf("Agent> %s %s", label, content)}

	case "TEXT_MESSAGE_END":
		return []string{fmt.Sprintf("Agent> %s", label)}

	case "TOOL_CALL_START":
		name, _ := event["name"].(string)
		toolCallID, _ := event["toolCallId"].(string)
		return []string{fmt.Sprintf("Agent> %s tool '%s' started, id: %s", label, name, toolCallID)}

	case "TOOL_CALL_ARGS":
		args, _ := event["args"].(string)
		return []string{fmt.Sprintf("Agent> %s args: %s", label, args)}

	case "TOOL_CALL_END":
		toolCallID, _ := event["toolCallId"].(string)
		return []string{fmt.Sprintf("Agent> %s tool call completed, id: %s", label, toolCallID)}

	case "TOOL_CALL_RESULT":
		result, _ := event["result"].(string)
		return []string{fmt.Sprintf("Agent> %s result: %s", label, result)}

	case "STEP_STARTED":
		stepType, _ := event["type"].(string)
		return []string{fmt.Sprintf("Agent> %s step '%s' started", label, stepType)}

	case "STEP_FINISHED":
		stepType, _ := event["type"].(string)
		return []string{fmt.Sprintf("Agent> %s step '%s' finished", label, stepType)}

	case "STATE_DELTA":
		delta, _ := event["delta"].(map[string]any)
		data, _ := json.Marshal(delta)
		return []string{fmt.Sprintf("Agent> %s %s", label, string(data))}

	case "ACTIVITY_START":
		name, _ := event["name"].(string)
		return []string{fmt.Sprintf("Agent> %s activity '%s' started", label, name)}

	case "ACTIVITY_END":
		return []string{fmt.Sprintf("Agent> %s activity completed", label)}

	case "ACTIVITY_UPDATE":
		status, _ := event["status"].(string)
		progress, _ := event["progress"].(int)
		if progress > 0 {
			return []string{fmt.Sprintf("Agent> %s %s (%d%%)", label, status, progress)}
		}
		return []string{fmt.Sprintf("Agent> %s %s", label, status)}

	case "CUSTOM":
		name, _ := event["name"].(string)
		value, _ := event["value"].(map[string]any)
		data, _ := json.Marshal(value)
		return []string{fmt.Sprintf("Agent> %s '%s': %s", label, name, string(data))}

	default:
		data, _ := json.Marshal(event["data"])
		return []string{fmt.Sprintf("Agent> %s %s", label, string(data))}
	}
}
