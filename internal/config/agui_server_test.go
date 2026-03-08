package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAguiServerOptions_Validate(t *testing.T) {
	t.Run("nil options returns nil", func(t *testing.T) {
		var opts *AguiServerOptions
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("valid empty options", func(t *testing.T) {
		opts := &AguiServerOptions{}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("valid port", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: 8080,
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("port zero is valid", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: 0,
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("port 65535 is valid", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: 65535,
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("negative port returns error", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: -1,
		}
		err := opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("port greater than 65535 returns error", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: 65536,
		}
		err := opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid port")
	})

	t.Run("valid base_path with leading slash", func(t *testing.T) {
		opts := &AguiServerOptions{
			BasePath: "/agui",
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("valid base_path with nested path", func(t *testing.T) {
		opts := &AguiServerOptions{
			BasePath: "/api/agui",
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("empty base_path is valid", func(t *testing.T) {
		opts := &AguiServerOptions{
			BasePath: "",
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("base_path without leading slash returns error", func(t *testing.T) {
		opts := &AguiServerOptions{
			BasePath: "agui",
		}
		err := opts.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid base_path")
	})

	t.Run("valid CORS origins", func(t *testing.T) {
		opts := &AguiServerOptions{
			CORSOrigins: []string{"http://localhost:3000", "https://example.com"},
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("empty CORS origins is valid", func(t *testing.T) {
		opts := &AguiServerOptions{
			CORSOrigins: []string{},
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("nil CORS origins is valid", func(t *testing.T) {
		opts := &AguiServerOptions{
			CORSOrigins: nil,
		}
		err := opts.Validate()
		require.NoError(t, err)
	})

	t.Run("combined valid options", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port:        9000,
			BasePath:    "/agui",
			CORSOrigins: []string{"http://localhost:3000"},
		}
		err := opts.Validate()
		require.NoError(t, err)
	})
}

func TestAguiServerOptions_IsEnabled(t *testing.T) {
	t.Run("nil options returns false", func(t *testing.T) {
		var opts *AguiServerOptions
		enabled := opts.IsEnabled()
		assert.False(t, enabled)
	})

	t.Run("nil Enabled field returns false", func(t *testing.T) {
		opts := &AguiServerOptions{}
		enabled := opts.IsEnabled()
		assert.False(t, enabled)
	})

	t.Run("Enabled set to true returns true", func(t *testing.T) {
		enabled := true
		opts := &AguiServerOptions{
			Enabled: &enabled,
		}
		assert.True(t, opts.IsEnabled())
	})

	t.Run("Enabled set to false returns false", func(t *testing.T) {
		enabled := false
		opts := &AguiServerOptions{
			Enabled: &enabled,
		}
		assert.False(t, opts.IsEnabled())
	})
}

func TestAguiServerOptions_GetPort(t *testing.T) {
	t.Run("nil options returns default 8080", func(t *testing.T) {
		var opts *AguiServerOptions
		port := opts.GetPort()
		assert.Equal(t, 8080, port)
	})

	t.Run("zero port returns default 8080", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: 0,
		}
		port := opts.GetPort()
		assert.Equal(t, 8080, port)
	})

	t.Run("configured port is returned", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: 9000,
		}
		port := opts.GetPort()
		assert.Equal(t, 9000, port)
	})

	t.Run("max port is returned", func(t *testing.T) {
		opts := &AguiServerOptions{
			Port: 65535,
		}
		port := opts.GetPort()
		assert.Equal(t, 65535, port)
	})
}

func TestAguiServerOptions_GetBasePath(t *testing.T) {
	t.Run("nil options returns default /agui", func(t *testing.T) {
		var opts *AguiServerOptions
		basePath := opts.GetBasePath()
		assert.Equal(t, "/agui", basePath)
	})

	t.Run("empty base_path returns default /agui", func(t *testing.T) {
		opts := &AguiServerOptions{
			BasePath: "",
		}
		basePath := opts.GetBasePath()
		assert.Equal(t, "/agui", basePath)
	})

	t.Run("configured base_path is returned", func(t *testing.T) {
		opts := &AguiServerOptions{
			BasePath: "/api/agui",
		}
		basePath := opts.GetBasePath()
		assert.Equal(t, "/api/agui", basePath)
	})

	t.Run("root path is returned", func(t *testing.T) {
		opts := &AguiServerOptions{
			BasePath: "/",
		}
		basePath := opts.GetBasePath()
		assert.Equal(t, "/", basePath)
	})
}

func TestAguiServerOptions_GetCORSOrigins(t *testing.T) {
	t.Run("nil options returns nil", func(t *testing.T) {
		var opts *AguiServerOptions
		origins := opts.GetCORSOrigins()
		assert.Nil(t, origins)
	})

	t.Run("nil CORS origins returns nil", func(t *testing.T) {
		opts := &AguiServerOptions{
			CORSOrigins: nil,
		}
		origins := opts.GetCORSOrigins()
		assert.Nil(t, origins)
	})

	t.Run("empty CORS origins returns empty slice", func(t *testing.T) {
		opts := &AguiServerOptions{
			CORSOrigins: []string{},
		}
		origins := opts.GetCORSOrigins()
		assert.NotNil(t, origins)
		assert.Empty(t, origins)
	})

	t.Run("configured CORS origins are returned", func(t *testing.T) {
		opts := &AguiServerOptions{
			CORSOrigins: []string{"http://localhost:3000", "https://example.com"},
		}
		origins := opts.GetCORSOrigins()
		assert.Len(t, origins, 2)
		assert.Equal(t, "http://localhost:3000", origins[0])
		assert.Equal(t, "https://example.com", origins[1])
	})
}

func TestAguiServerOptions_ValidateWithConfig(t *testing.T) {
	t.Run("valid configuration in Config struct", func(t *testing.T) {
		enabled := true
		cfg := &Config{
			Options: &Options{
				AguiServer: &AguiServerOptions{
					Enabled:     &enabled,
					Port:        8080,
					BasePath:    "/agui",
					CORSOrigins: []string{"http://localhost:3000"},
				},
			},
		}
		err := cfg.Options.AguiServer.Validate()
		require.NoError(t, err)
		assert.True(t, cfg.Options.AguiServer.IsEnabled())
		assert.Equal(t, 8080, cfg.Options.AguiServer.GetPort())
		assert.Equal(t, "/agui", cfg.Options.AguiServer.GetBasePath())
	})

	t.Run("disabled server configuration", func(t *testing.T) {
		enabled := false
		cfg := &Config{
			Options: &Options{
				AguiServer: &AguiServerOptions{
					Enabled: &enabled,
					Port:    9000,
				},
			},
		}
		err := cfg.Options.AguiServer.Validate()
		require.NoError(t, err)
		assert.False(t, cfg.Options.AguiServer.IsEnabled())
		assert.Equal(t, 9000, cfg.Options.AguiServer.GetPort())
	})

	t.Run("invalid port in Config struct", func(t *testing.T) {
		cfg := &Config{
			Options: &Options{
				AguiServer: &AguiServerOptions{
					Port: 70000,
				},
			},
		}
		err := cfg.Options.AguiServer.Validate()
		require.Error(t, err)
	})
}
