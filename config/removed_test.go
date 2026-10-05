package config_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/oddbit-project/blueprint/config"
	"github.com/oddbit-project/blueprint/config/provider"
)

func jsonProvider(t *testing.T, src string) config.ConfigProvider {
	t.Helper()
	p, err := provider.NewJsonProvider([]byte(src))
	require.NoError(t, err)
	return p
}

var severalRemoved = map[string]string{"c.x": "hc", "a": "ha", "b.y": "hb"}

const severalConfig = `{"c":{"x":1},"a":1,"b":{"y":2}}`
const severalMsg = `Removed config key present: "a": ha; "b.y": hb; "c.x": hc`

func TestCheckRemovedKeys(t *testing.T) {
	nats := map[string]string{"nats.maxDeliver": "use events.policies"}
	natsH := map[string]string{"nats.maxDeliver": "h"}
	natsEmpty := map[string]string{"nats.maxDeliver": ""}
	aclGone := map[string]string{"modules.auth.aclCacheTtl": "gone"}
	row1 := `Removed config key present: "nats.maxDeliver": use events.policies`

	tests := []struct {
		name    string
		config  string
		removed map[string]string
		wantErr error
		wantMsg string
	}{
		{"removed key", `{"nats":{"maxDeliver":9}}`, nats, config.ErrRemovedKey, row1},
		{"case-insensitive every segment", `{"Nats":{"MaxDeliver":1}}`, nats, config.ErrRemovedKey,
			`Removed config key present: "Nats.MaxDeliver" (registered as "nats.maxDeliver"): use events.policies`},
		{"mixed case per segment", `{"nATS":{"maxdeliver":1}}`, nats, config.ErrRemovedKey,
			`Removed config key present: "nATS.maxdeliver" (registered as "nats.maxDeliver"): use events.policies`},
		{"empty hint", `{"nats":{"maxDeliver":1}}`, natsEmpty, config.ErrRemovedKey,
			`Removed config key present: "nats.maxDeliver"`},
		{"nested path", `{"modules":{"auth":{"aclCacheTtl":60}}}`, aclGone, config.ErrRemovedKey,
			`Removed config key present: "modules.auth.aclCacheTtl": gone`},
		{"missing intermediate", `{"modules":{}}`, aclGone, nil, ""},
		{"empty root", `{}`, aclGone, nil, ""},
		{"null intermediate", `{"modules":null}`, aclGone, nil, ""},
		{"array intermediate", `{"modules":[1]}`, aclGone, nil, ""},
		{"scalar intermediate", `{"modules":{"auth":5}}`, aclGone, nil, ""},
		{"null leaf counts", `{"nats":{"maxDeliver":null}}`, nats, config.ErrRemovedKey, row1},
		{"unregistered keys ignored", `{"nats":{"other":1},"x":2}`, nats, nil, ""},
		{"both case variants in one object", `{"nats":{"maxDeliver":1,"MaxDeliver":2}}`, natsH, config.ErrRemovedKey,
			`Removed config key present: "nats.MaxDeliver" (registered as "nats.maxDeliver"): h; "nats.maxDeliver": h`},
		{"case variants at intermediate level", `{"nats":{"maxDeliver":1},"NATS":{"maxDeliver":2}}`, natsH, config.ErrRemovedKey,
			`Removed config key present: "NATS.maxDeliver" (registered as "nats.maxDeliver"): h; "nats.maxDeliver": h`},
		{"several removed keys sorted", severalConfig, severalRemoved, config.ErrRemovedKey, severalMsg},
		{"invalid path empty", `{}`, map[string]string{"": "h"}, config.ErrInvalidKeyPath, `Invalid config key path: ""`},
		{"invalid path leading dot", `{}`, map[string]string{".a": "h"}, config.ErrInvalidKeyPath, `Invalid config key path: ".a"`},
		{"invalid path trailing dot", `{}`, map[string]string{"a.": "h"}, config.ErrInvalidKeyPath, `Invalid config key path: "a."`},
		{"invalid path double dot", `{}`, map[string]string{"a..b": "h"}, config.ErrInvalidKeyPath, `Invalid config key path: "a..b"`},
		{"invalid path checked before hits", `{"a":1}`, map[string]string{"a": "h", "b..c": "h"}, config.ErrInvalidKeyPath,
			`Invalid config key path: "b..c"`},
		{"registrations differ only in case", `{}`, map[string]string{"nats.maxDeliver": "", "Nats.MaxDeliver": ""}, config.ErrInvalidKeyPath,
			`Invalid config key path: "Nats.MaxDeliver" and "nats.maxDeliver" differ only in case`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := config.CheckRemovedKeys(jsonProvider(t, tc.config), tc.removed)
			if tc.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.True(t, errors.Is(err, tc.wantErr))
			assert.Equal(t, tc.wantMsg, err.Error())
		})
	}
}

func TestCheckRemovedKeysDeterministic(t *testing.T) {
	p := jsonProvider(t, severalConfig)
	for i := 0; i < 50; i++ {
		err := config.CheckRemovedKeys(p, severalRemoved)
		require.Error(t, err)
		require.Equal(t, severalMsg, err.Error())
	}
}

func TestCheckRemovedKeysEmptyMapSkipsProvider(t *testing.T) {
	p := provider.NewEnvProvider("TESTRMK_", false)
	assert.NoError(t, config.CheckRemovedKeys(p, nil))
	assert.NoError(t, config.CheckRemovedKeys(p, map[string]string{}))
}

func TestCheckRemovedKeysEnvProvider(t *testing.T) {
	p := provider.NewEnvProvider("TESTRMK_", false)
	err := config.CheckRemovedKeys(p, map[string]string{"a": "h"})
	require.Error(t, err)
	assert.True(t, errors.Is(err, config.ErrNotImplemented))
	assert.True(t, errors.Is(err, config.ErrNoKey))
	assert.False(t, errors.Is(err, config.ErrRemovedKey))
}

func TestCheckRemovedKeysSubtree(t *testing.T) {
	n, err := jsonProvider(t, `{"nats":{"maxDeliver":1}}`).GetConfigNode("nats")
	require.NoError(t, err)

	err = config.CheckRemovedKeys(n, map[string]string{"maxDeliver": "h"})
	require.Error(t, err)
	assert.Equal(t, `Removed config key present: "maxDeliver": h`, err.Error())

	assert.NoError(t, config.CheckRemovedKeys(n, map[string]string{"nats.maxDeliver": "h"}))
}
