package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

// longcatToolCallRegex matches <longcat_tool_call>...</longcat_tool_call> or
// incomplete <longcat_tool_call>... at end of stream.
// Format: <longcat_tool_call>{"name":"tool_name","arguments":{...}}</longcat_tool_call>
var longcatToolCallRegex = regexp.MustCompile(`<longcat_tool_call>(.*?)</longcat_tool_call>|<longcat_tool_call>([\s\S]*)$`)

// LongcatToolCall represents a parsed tool call from longcat format.
type LongcatToolCall struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

// ParseLongcatToolCalls extracts tool calls from text that may contain
// <longcat_tool_call>...</longcat_tool_call> tags.
// Returns the tool calls found and the remaining text (with tags removed).
func ParseLongcatToolCalls(text string) (calls []LongcatToolCall, remaining string) {
	remaining = text
	matches := longcatToolCallRegex.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, text
	}

	for _, m := range matches {
		var content string
		if len(m) > 1 && m[1] != "" {
			content = strings.TrimSpace(m[1])
		} else if len(m) > 2 && m[2] != "" {
			content = strings.TrimSpace(m[2])
		}
		if content == "" {
			continue
		}

		var tc LongcatToolCall
		if err := json.Unmarshal([]byte(content), &tc); err != nil {
			continue
		}
		if tc.Name != "" {
			calls = append(calls, tc)
		}
	}

	// Remove matched tool call blocks from remaining text
	remaining = longcatToolCallRegex.ReplaceAllString(text, "")
	remaining = strings.TrimSpace(remaining)
	return calls, remaining
}

// HasIncompleteLongcatToolCall returns true if text ends with an unclosed
// <longcat_tool_call> tag (content still streaming).
func HasIncompleteLongcatToolCall(text string) bool {
	open := strings.LastIndex(text, "<longcat_tool_call>")
	close := strings.LastIndex(text, "</longcat_tool_call>")
	return open > close
}
