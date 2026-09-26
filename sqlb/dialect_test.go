package sqlb

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDialectFeatures(t *testing.T) {
	tests := []struct {
		name     string
		d        Dialect
		wantName string
		has      []Feature
		hasNot   []Feature
		maxArgs  int
	}{
		{
			name:     "postgres",
			d:        Postgres(),
			wantName: "postgres",
			has:      []Feature{FeatureReturning, FeatureUpsert, FeatureILike, FeatureUpdate},
			hasNot:   []Feature{FeatureClickHouse},
			maxArgs:  65535,
		},
		{
			name:     "sqlite",
			d:        SQLite(),
			wantName: "sqlite",
			has:      []Feature{FeatureReturning, FeatureUpsert, FeatureUpdate},
			hasNot:   []Feature{FeatureILike, FeatureClickHouse},
			maxArgs:  32766,
		},
		{
			name:     "clickhouse",
			d:        ClickHouse(),
			wantName: "clickhouse",
			has:      []Feature{FeatureILike, FeatureClickHouse},
			hasNot:   []Feature{FeatureReturning, FeatureUpsert, FeatureUpdate},
			maxArgs:  0,
		},
		{
			name:     "generic",
			d:        Generic(),
			wantName: "generic",
			has:      []Feature{FeatureUpdate},
			hasNot:   []Feature{FeatureReturning, FeatureUpsert, FeatureILike, FeatureClickHouse},
			maxArgs:  999,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantName, tt.d.Name())
			for _, f := range tt.has {
				assert.True(t, tt.d.Has(f), "expected feature %d", f)
			}
			for _, f := range tt.hasNot {
				assert.False(t, tt.d.Has(f), "unexpected feature %d", f)
			}
			assert.Equal(t, tt.maxArgs, tt.d.MaxArgs())
		})
	}
}

func TestDialectForPreRegistered(t *testing.T) {
	tests := []struct {
		driver   string
		wantName string
	}{
		{"pgx", "postgres"},
		{"pgx/v5", "postgres"},
		{"postgres", "postgres"},
		{"sqlite", "sqlite"},
		{"sqlite3", "sqlite"},
		{"clickhouse", "clickhouse"},
	}
	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			d, err := DialectFor(tt.driver)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantName, d.Name())
		})
	}
}

func TestDialectForUnknown(t *testing.T) {
	_, err := DialectFor("nope")
	assert.True(t, errors.Is(err, ErrUnknownDialect))
}

func TestRegisterOverrides(t *testing.T) {
	Register("sqlb-test-driver", SQLite())
	d, err := DialectFor("sqlb-test-driver")
	assert.NoError(t, err)
	assert.Equal(t, "sqlite", d.Name())

	Register("sqlb-test-driver", Postgres())
	d, err = DialectFor("sqlb-test-driver")
	assert.NoError(t, err)
	assert.Equal(t, "postgres", d.Name())
}
