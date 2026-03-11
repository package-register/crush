package agent

import (
	"testing"

	"charm.land/catwalk/pkg/catwalk"
	"github.com/charmbracelet/crush/internal/config"
	"github.com/stretchr/testify/require"
)

// TestCoordinator_buildProviderWithWireAPI verifies that an openai-compat provider
// with wire_api="responses" can be built successfully (used for codex, packyapi, etc.).
func TestCoordinator_buildProviderWithWireAPI(t *testing.T) {
	env := testEnv(t)

	cfg, err := config.Init(env.workingDir, "", false)
	require.NoError(t, err)

	// Override with codex provider that uses wire_api=responses.
	cfg.Providers.Set("codex", config.ProviderConfig{
		ID:      "codex",
		Type:    catwalk.TypeOpenAICompat,
		APIKey:  "sk-test-key",
		BaseURL: "http://127.0.0.1:9999/v1",
		WireAPI: "responses",
		Models: []catwalk.Model{
			{ID: "gpt-4", Name: "GPT-4"},
		},
	})
	cfg.Models = map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: "codex", Model: "gpt-4", MaxTokens: 4096},
		config.SelectedModelTypeSmall: {Provider: "codex", Model: "gpt-4", MaxTokens: 2048},
	}

	coord := &coordinator{
		cfg:      cfg,
		sessions: env.sessions,
	}

	largeModelCfg := config.SelectedModel{Provider: "codex", Model: "gpt-4", MaxTokens: 4096}
	largeProviderCfg, ok := cfg.Providers.Get("codex")
	require.True(t, ok, "codex provider must be in config")
	require.Equal(t, "responses", largeProviderCfg.WireAPI, "wire_api must be set")

	// buildProvider should succeed; the fantasy provider will use /v1/responses.
	_, err = coord.buildProvider(largeProviderCfg, largeModelCfg, false)
	require.NoError(t, err)
}
