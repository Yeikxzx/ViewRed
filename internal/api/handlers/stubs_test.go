package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)



// TestVulnerabilitiesList_ReturnsEmptyArray verifies the vulnerabilities
// endpoint returns 200 with `[]`. Same UI-safety reason as
// TestAlertsList_ReturnsEmptyArray: a `null` body breaks `.map(...)`.
func TestVulnerabilitiesList_ReturnsEmptyArray(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/vulnerabilities", VulnerabilitiesList)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vulnerabilities", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())
}

// TestConfigGet_ReturnsVersionAndEmptyFeatures verifies the config
// endpoint returns the current viewred version and an empty features
// map. The React settings view reads `version` for the "About"
// footer and `features` to know which toggles to render. Even with no
// features yet, the key MUST be present (not omitted) so the UI does
// not have to special-case `undefined`.
func TestConfigGet_ReturnsVersionAndEmptyFeatures(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/config", ConfigGet)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)

	assert.Equal(t, "0.1.0", body["version"])
	assert.Contains(t, body, "features")
	assert.JSONEq(t, `{}`, asJSON(t, body["features"]))
}

// asJSON re-marshals a decoded value so we can use assert.JSONEq to
// compare it against a literal. assert.JSONEq requires string inputs;
// map[string]any round-trips through json.Marshal with consistent
// key ordering in Go's encoding/json, which is what we want here.
func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
