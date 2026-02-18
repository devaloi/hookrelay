package signature

import (
	"testing"
)

func TestSign(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		secret  string
		wantPfx string
		wantLen int
	}{
		{
			name:    "valid signature",
			payload: []byte(`{"event":"test"}`),
			secret:  "mysecret",
			wantPfx: "sha256=",
			wantLen: 71,
		},
		{
			name:    "empty secret returns empty",
			payload: []byte(`{"event":"test"}`),
			secret:  "",
			wantPfx: "",
			wantLen: 0,
		},
		{
			name:    "empty payload",
			payload: []byte{},
			secret:  "mysecret",
			wantPfx: "sha256=",
			wantLen: 71,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Sign(tt.payload, tt.secret)
			if len(got) != tt.wantLen {
				t.Errorf("Sign() len = %d, want %d", len(got), tt.wantLen)
			}
			if tt.wantPfx != "" && got[:7] != tt.wantPfx {
				t.Errorf("Sign() prefix = %q, want %q", got[:7], tt.wantPfx)
			}
		})
	}
}

func TestVerify(t *testing.T) {
	payload := []byte(`{"event":"test"}`)
	secret := "mysecret"
	validSig := Sign(payload, secret)

	tests := []struct {
		name      string
		payload   []byte
		signature string
		secret    string
		wantErr   bool
	}{
		{
			name:      "valid signature",
			payload:   payload,
			signature: validSig,
			secret:    secret,
			wantErr:   false,
		},
		{
			name:      "empty secret skips verification",
			payload:   payload,
			signature: "",
			secret:    "",
			wantErr:   false,
		},
		{
			name:      "missing signature with secret",
			payload:   payload,
			signature: "",
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "invalid signature format",
			payload:   payload,
			signature: "invalid",
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "wrong signature",
			payload:   payload,
			signature: "sha256=0000000000000000000000000000000000000000000000000000000000000000",
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "wrong secret",
			payload:   payload,
			signature: validSig,
			secret:    "wrongsecret",
			wantErr:   true,
		},
		{
			name:      "modified payload",
			payload:   []byte(`{"event":"modified"}`),
			signature: validSig,
			secret:    secret,
			wantErr:   true,
		},
		{
			name:      "invalid hex encoding",
			payload:   payload,
			signature: "sha256=notvalidhex",
			secret:    secret,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Verify(tt.payload, tt.signature, tt.secret)
			if (err != nil) != tt.wantErr {
				t.Errorf("Verify() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	payloads := [][]byte{
		[]byte(`{"event":"test"}`),
		[]byte(`simple text`),
		[]byte{0x00, 0x01, 0x02, 0x03},
		[]byte(`{"complex": {"nested": ["data", 123, true]}}`),
	}
	secrets := []string{
		"simple",
		"with spaces",
		"special!@#$%chars",
		"unicode-日本語",
	}

	for _, payload := range payloads {
		for _, secret := range secrets {
			sig := Sign(payload, secret)
			err := Verify(payload, sig, secret)
			if err != nil {
				t.Errorf("roundtrip failed for payload=%q secret=%q: %v", payload, secret, err)
			}
		}
	}
}
