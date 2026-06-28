package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadDefaultsIncludeLocalhost5173CORSOrigin pins the dev-mode
// CORS default. The React UI runs on Vite at http://localhost:5173
// and fetches the API on http://127.0.0.1:8080. Without this
// default the browser blocks every API request with a CORS error
// and the dashboard renders nothing. The dev convenience defaults
// include three common local origins (localhost:5173 for the Vite
// dev server, 127.0.0.1:5173 as an alias, and 127.0.0.1:3000 for
// alternate dev ports); production deployments override them via
// config.yaml's server.cors_origins.
func TestLoadDefaultsIncludeLocalhost5173CORSOrigin(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"http://localhost:5173", "http://127.0.0.1:5173", "http://127.0.0.1:3000"},
		cfg.Server.CORSOrigins,
		"default CORSOrigins must permit the Vite dev origin")
}

// TestLoadCORSOriginsFromYAML verifies the YAML key
// `server.cors_origins` overrides the dev default. Production
// deployments use this to allowlist their real frontend domain.
func TestLoadCORSOriginsFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	yaml := `
server:
  bind_address: 0.0.0.0
  port: 9000
  cors_origins:
    - https://viewred.example.com
    - https://ops.viewred.example.com
`
	require.NoError(t, writeFile(path, yaml))

	cfg, err := Load(path)
	require.NoError(t, err)
	assert.Equal(t,
		[]string{"https://viewred.example.com", "https://ops.viewred.example.com"},
		cfg.Server.CORSOrigins,
		"cors_origins from YAML must replace the dev default")
}
