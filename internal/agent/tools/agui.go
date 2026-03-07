package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"charm.land/fantasy"
	aguisse "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/client/sse"
	aguievents "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/events"
	aguitypes "github.com/ag-ui-protocol/ag-ui/sdks/community/go/pkg/core/types"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/charmbracelet/crush/internal/permission"
)

const AguiToolName = "agui_sse"

type AguiParams struct {
	Endpoint       string `json:"endpoint,omitempty" jsonschema:"description=Optional AG-UI endpoint override."`
	APIKey         string `json:"api_key,omitempty" jsonschema:"description=Optional AG-UI API key override."`
	ThreadID       string `json:"thread_id,omitempty" jsonschema:"description=Optional AG-UI thread ID."`
	Message        string `json:"message" jsonschema:"required,description=Message sent to AG-UI agent."`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty" jsonschema:"description=Optional per-request timeout in seconds."`
}

func NewAguiTool(cfg *config.Config, permissions permission.Service) fantasy.AgentTool {
	return fantasy.NewParallelAgentTool(
		AguiToolName,
		"Call AG-UI over SSE and return streamed text/tool events.",
		func(ctx context.Context, params AguiParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Message) == "" {
				return fantasy.NewTextErrorResponse("Message parameter is required"), nil
			}
			if cfg == nil || cfg.Options == nil {
				return fantasy.NewTextErrorResponse("AG-UI configuration is unavailable"), nil
			}
			if cfg.Options.Agui.Enabled != nil && !*cfg.Options.Agui.Enabled {
				return fantasy.NewTextErrorResponse("AG-UI tool is disabled"), nil
			}

			aguiCfg := cfg.Options.Agui
			endpoint := params.Endpoint
			if endpoint == "" {
				endpoint = aguiCfg.Endpoint
			}
			if endpoint == "" {
				return fantasy.NewTextErrorResponse("AG-UI endpoint is required"), nil
			}
			apiKey := params.APIKey
			if apiKey == "" {
				apiKey = aguiCfg.APIKey
			}

			sessionID := GetSessionFromContext(ctx)
			allowed, err := permissions.Request(ctx, permission.CreatePermissionRequest{
				SessionID:   sessionID,
				Path:        cfg.WorkingDir(),
				ToolCallID:  call.ID,
				ToolName:    AguiToolName,
				Action:      "stream",
				Description: fmt.Sprintf("Stream AG-UI events from %s", endpoint),
				Params:      params,
			})
			if err != nil {
				return fantasy.ToolResponse{}, err
			}
			if !allowed {
				return fantasy.ToolResponse{}, permission.ErrorPermissionDenied
			}

			streamCtx := ctx
			if params.TimeoutSeconds > 0 {
				var cancel context.CancelFunc
				streamCtx, cancel = context.WithTimeout(ctx, time.Duration(params.TimeoutSeconds)*time.Second)
				defer cancel()
			}

			client := aguisse.NewClient(aguisse.Config{
				Endpoint:       endpoint,
				APIKey:         apiKey,
				ConnectTimeout: aguiCfg.ConnectTimeout,
				ReadTimeout:    aguiCfg.ReadTimeout,
			})
			defer client.Close()

			threadID := params.ThreadID
			if threadID == "" {
				threadID = fmt.Sprintf("crush_%d", time.Now().UnixNano())
			}
			input := aguitypes.RunAgentInput{
				ThreadID: threadID,
				RunID:    fmt.Sprintf("run_%d", time.Now().UnixNano()),
				State:    map[string]any{},
				Messages: []aguitypes.Message{{
					ID:      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
					Role:    aguitypes.RoleUser,
					Content: params.Message,
				}},
				Tools:          []aguitypes.Tool{},
				Context:        []aguitypes.Context{},
				ForwardedProps: map[string]any{},
			}

			frames, errs, err := client.Stream(aguisse.StreamOptions{
				Context: streamCtx,
				Payload: input,
			})
			if err != nil {
				return fantasy.NewTextErrorResponse(fmt.Sprintf("AG-UI stream failed: %v", err)), nil
			}

			decoder := aguievents.NewEventDecoder(nil)
			var out strings.Builder

			for {
				select {
				case frame, ok := <-frames:
					if !ok {
						text := strings.TrimSpace(out.String())
						if text == "" {
							text = "AG-UI stream completed with no text output"
						}
						return fantasy.NewTextResponse(text), nil
					}

					var envelope struct {
						Type string `json:"type"`
					}
					if err := json.Unmarshal(frame.Data, &envelope); err != nil {
						out.WriteString("\n[agui decode error] ")
						out.WriteString(err.Error())
						continue
					}
					event, err := decoder.DecodeEvent(envelope.Type, frame.Data)
					if err != nil {
						out.WriteString("\n[agui event error] ")
						out.WriteString(err.Error())
						continue
					}
					switch e := event.(type) {
					case *aguievents.TextMessageStartEvent:
						out.WriteString("\n[start ")
						out.WriteString(e.MessageID)
						out.WriteString("]\n")
					case *aguievents.TextMessageContentEvent:
						out.WriteString(e.Delta)
					case *aguievents.TextMessageEndEvent:
						out.WriteString("\n[end]\n")
					case *aguievents.ToolCallStartEvent:
						out.WriteString("\n[tool ")
						out.WriteString(e.ToolCallName)
						out.WriteString("]\n")
					}

				case err, ok := <-errs:
					if !ok || err == nil {
						continue
					}
					return fantasy.NewTextErrorResponse(fmt.Sprintf("AG-UI stream error: %v", err)), nil

				case <-streamCtx.Done():
					if out.Len() > 0 {
						return fantasy.NewTextResponse(strings.TrimSpace(out.String())), nil
					}
					return fantasy.NewTextErrorResponse(streamCtx.Err().Error()), nil
				}
			}
		},
	)
}
