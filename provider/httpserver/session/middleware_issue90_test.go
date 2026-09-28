package session

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/crypt/secure"
	"github.com/oddbit-project/blueprint/log"
	"github.com/oddbit-project/blueprint/provider/kv"
)

// failingGetKV is a kv.KV whose Get always fails; everything else is a memory store.
type failingGetKV struct {
	kv.KV
	err error
}

func (f *failingGetKV) Get(string) ([]byte, error) { return nil, f.err }

type issue90Env struct {
	backend kv.KV
	store   *Store
	config  *Config
	router  *gin.Engine
	logs    *bytes.Buffer
}

func newIssue90Env(t *testing.T, backend kv.KV, encrypted bool) *issue90Env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var buf bytes.Buffer
	logger := log.New("session-test").WithOutput(&buf)

	config := NewConfig()
	if encrypted {
		config.EncryptionKey = secure.DefaultCredentialConfig{Password: "0123456789abcdef0123456789abcdef"}
	}
	store, err := NewStore(config, backend, logger)
	require.NoError(t, err)
	manager, err := NewManager(config, ManagerWithStore(store), ManagerWithLogger(logger))
	require.NoError(t, err)
	t.Cleanup(store.StopCleanup)

	router := gin.New()
	router.Use(manager.Middleware())
	router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })

	// positive control: a warning written through this logger must reach the buffer,
	// so the "not logged" guards cannot pass because warnings are suppressed
	logger.Warn("probe")
	require.Contains(t, buf.String(), `"level":"warn"`, "warnings must be captured")
	buf.Reset()
	return &issue90Env{backend: backend, store: store, config: config, router: router, logs: &buf}
}

// request sends a request with the given session cookie (none if empty) and returns the
// session cookie the response sets, if any.
func (e *issue90Env) request(t *testing.T, cookie string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: e.config.CookieName, Value: cookie})
	}
	w := httptest.NewRecorder()
	e.router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	for _, c := range w.Result().Cookies() {
		if c.Name == e.config.CookieName {
			return c.Value
		}
	}
	return ""
}

func (e *issue90Env) warningLogged() bool {
	return strings.Contains(e.logs.String(), `"level":"warn"`)
}

// Issue #90 claim 1: a stored session that fails to decrypt is logged, then replaced.
func TestMiddleware_Issue90_DecryptFailureIsLogged(t *testing.T) {
	env := newIssue90Env(t, kv.NewMemoryKV(), true)
	sess, id := env.store.Generate()
	require.NoError(t, env.store.Set(id, sess))
	data, err := env.backend.Get(id)
	require.NoError(t, err)
	tampered := append([]byte{}, data...)
	tampered[len(tampered)-1] ^= 0xff
	require.NoError(t, env.backend.SetTTL(id, tampered, time.Minute))

	newID := env.request(t, id)

	assert.True(t, env.warningLogged(), "logs: %s", env.logs.String())
	assert.Contains(t, env.logs.String(), secure.ErrAuthenticationFailed.Error())
	assert.NotEmpty(t, newID, "a new session is still issued")
	assert.NotEqual(t, id, newID)
	assert.NotContains(t, env.logs.String(), id, "the session ID must not be logged")
}

// Issue #90 claim 2: a backend read failure is logged.
func TestMiddleware_Issue90_BackendReadFailureIsLogged(t *testing.T) {
	backendErr := errors.New("kv get failed: backend unavailable")
	env := newIssue90Env(t, &failingGetKV{KV: kv.NewMemoryKV(), err: backendErr}, false)
	const id = "cookie-value-for-backend-failure"

	newID := env.request(t, id)

	assert.True(t, env.warningLogged(), "logs: %s", env.logs.String())
	assert.Contains(t, env.logs.String(), backendErr.Error())
	assert.NotEmpty(t, newID)
	assert.NotContains(t, env.logs.String(), id, "the session ID must not be logged")
}

// Issue #90 guard: an unknown session ID (ErrSessionNotFound) is normal and not logged.
func TestMiddleware_Issue90_UnknownSessionNotLogged(t *testing.T) {
	env := newIssue90Env(t, kv.NewMemoryKV(), true)
	newID := env.request(t, "unknown-session-id")
	assert.Empty(t, env.logs.String(), "nothing is logged")
	assert.NotEmpty(t, newID)
}

// Issue #90 guard: an expired session (ErrSessionExpired) is normal and not logged.
func TestMiddleware_Issue90_ExpiredSessionNotLogged(t *testing.T) {
	env := newIssue90Env(t, kv.NewMemoryKV(), true)
	sess, id := env.store.Generate()
	sess.Created = time.Now().Add(-2 * time.Duration(env.config.ExpirationSeconds) * time.Second)
	require.NoError(t, env.store.Set(id, sess))
	_, err := env.store.Get(id)
	require.ErrorIs(t, err, ErrSessionExpired, "fixture must produce an expired session")
	require.NoError(t, env.store.Set(id, sess))

	newID := env.request(t, id)
	assert.Empty(t, env.logs.String(), "nothing is logged")
	assert.NotEmpty(t, newID, "a new session is issued")
	assert.NotEqual(t, id, newID)
}

// Issue #90 guard: no cookie at all is not logged.
func TestMiddleware_Issue90_NoCookieNotLogged(t *testing.T) {
	env := newIssue90Env(t, kv.NewMemoryKV(), true)
	assert.NotEmpty(t, env.request(t, ""))
	assert.Empty(t, env.logs.String(), "nothing is logged")
}

// Issue #90 guard: a valid session is kept and not logged.
func TestMiddleware_Issue90_ValidSessionNotLogged(t *testing.T) {
	env := newIssue90Env(t, kv.NewMemoryKV(), true)
	sess, id := env.store.Generate()
	require.NoError(t, env.store.Set(id, sess))
	assert.Empty(t, env.request(t, id), "a valid session sets no new cookie")
	assert.Empty(t, env.logs.String(), "nothing is logged")
}

// Issue #90 claim 1, second shape: an undecodable record in an unencrypted store is logged.
func TestMiddleware_Issue90_UndecodableSessionIsLogged(t *testing.T) {
	env := newIssue90Env(t, kv.NewMemoryKV(), false)
	const id = "cookie-value-for-undecodable-record"
	require.NoError(t, env.backend.SetTTL(id, []byte("not a gob-encoded session"), time.Minute))

	newID := env.request(t, id)
	assert.True(t, env.warningLogged(), "logs: %s", env.logs.String())
	assert.NotEmpty(t, newID)
	assert.NotEqual(t, id, newID)
	assert.NotContains(t, env.logs.String(), id, "the session ID must not be logged")
}
