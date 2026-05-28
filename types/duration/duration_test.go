package duration

import (
	"encoding/json"
	"testing"
	"time"
)

func TestDuration_Constructors(t *testing.T) {
	tests := []struct {
		name string
		got  Duration
		want time.Duration
	}{
		{"Seconds(30)", Seconds(30), 30 * time.Second},
		{"Minutes(15)", Minutes(15), 15 * time.Minute},
		{"Hours(24)", Hours(24), 24 * time.Hour},
		{"Days(7)", Days(7), 7 * 24 * time.Hour},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.got.Std(); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestDuration_FromStd_TruncatesSubSecond(t *testing.T) {
	d := FromStd(1500 * time.Millisecond) // 1.5s → 1s
	if d != Seconds(1) {
		t.Errorf("FromStd(1.5s) = %d, want 1", d)
	}
}

func TestDuration_JSONMarshal_PlainInteger(t *testing.T) {
	type cfg struct {
		AccessTokenTTL  Duration `json:"accessTokenTtl"`
		RefreshTokenTTL Duration `json:"refreshTokenTtl"`
	}
	c := cfg{
		AccessTokenTTL:  Minutes(15),
		RefreshTokenTTL: Days(7),
	}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"accessTokenTtl":900,"refreshTokenTtl":604800}`
	if string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}

func TestDuration_JSONUnmarshal_AcceptsInteger(t *testing.T) {
	type cfg struct {
		AccessTokenTTL Duration `json:"accessTokenTtl"`
	}
	var c cfg
	if err := json.Unmarshal([]byte(`{"accessTokenTtl":900}`), &c); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if c.AccessTokenTTL.Std() != 15*time.Minute {
		t.Errorf("got %v, want 15m", c.AccessTokenTTL.Std())
	}
}

func TestDuration_IsPositive(t *testing.T) {
	if Seconds(0).IsPositive() {
		t.Error("zero must not be positive")
	}
	if Seconds(-1).IsPositive() {
		t.Error("negative must not be positive")
	}
	if !Seconds(1).IsPositive() {
		t.Error("one must be positive")
	}
}

func TestDuration_String_UsesTimeDurationFormat(t *testing.T) {
	if got := Minutes(15).String(); got != "15m0s" {
		t.Errorf("got %q, want %q", got, "15m0s")
	}
	if got := Days(7).String(); got != "168h0m0s" {
		t.Errorf("got %q, want %q", got, "168h0m0s")
	}
}
