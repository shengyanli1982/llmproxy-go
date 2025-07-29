package auth

import (
	"net/http"
	"testing"

	"github.com/shengyanli1982/llmproxy-go/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBearerAuthenticator_Apply(t *testing.T) {
	tests := []struct {
		name     string
		token    string
		wantAuth string
	}{
		{
			name:     "valid token",
			token:    "test-token-123",
			wantAuth: "Bearer test-token-123",
		},
		{
			name:     "empty token",
			token:    "",
			wantAuth: "Bearer ",
		},
		{
			name:     "token with spaces",
			token:    "test token with spaces",
			wantAuth: "Bearer test token with spaces",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &bearerAuthenticator{token: tt.token}
			req, err := http.NewRequest("GET", "http://example.com", nil)
			require.NoError(t, err)

			err = auth.Apply(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantAuth, req.Header.Get("Authorization"))
		})
	}
}

func TestBasicAuthenticator_Apply(t *testing.T) {
	tests := []struct {
		name         string
		username     string
		password     string
		expectedAuth string
	}{
		{
			name:         "valid credentials",
			username:     "user",
			password:     "pass",
			expectedAuth: "Basic dXNlcjpwYXNz", // base64 encoding of "user:pass"
		},
		{
			name:         "empty username",
			username:     "",
			password:     "pass",
			expectedAuth: "Basic OnBhc3M=", // base64 encoding of ":pass"
		},
		{
			name:         "empty password",
			username:     "user",
			password:     "",
			expectedAuth: "Basic dXNlcjo=", // base64 encoding of "user:"
		},
		{
			name:         "both empty",
			username:     "",
			password:     "",
			expectedAuth: "Basic Og==", // base64 encoding of ":"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &basicAuthenticator{
				username: tt.username,
				password: tt.password,
			}
			req, err := http.NewRequest("GET", "http://example.com", nil)
			require.NoError(t, err)

			err = auth.Apply(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedAuth, req.Header.Get("Authorization"))
		})
	}
}

func TestNoneAuthenticator_Apply(t *testing.T) {
	auth := &noneAuthenticator{}
	req, err := http.NewRequest("GET", "http://example.com", nil)
	require.NoError(t, err)

	// Set an existing Authorization header to ensure it's not modified
	req.Header.Set("Authorization", "existing-auth")

	err = auth.Apply(req)
	assert.NoError(t, err)
	assert.Equal(t, "existing-auth", req.Header.Get("Authorization"))
}

func TestFactory_Create(t *testing.T) {
	factory := NewFactory()

	tests := []struct {
		name       string
		config     *config.AuthConfig
		wantType   string
		wantError  bool
	}{
		{
			name: "bearer auth",
			config: &config.AuthConfig{
				Type:  "bearer",
				Token: "test-token",
			},
			wantType:  "bearer",
			wantError: false,
		},
		{
			name: "basic auth",
			config: &config.AuthConfig{
				Type:     "basic",
				Username: "user",
				Password: "pass",
			},
			wantType:  "basic",
			wantError: false,
		},
		{
			name: "none auth",
			config: &config.AuthConfig{
				Type: "none",
			},
			wantType:  "none",
			wantError: false,
		},
		{
			name:       "nil config",
			config:     nil,
			wantType:   "",
			wantError:  true,
		},
		{
			name: "unknown type",
			config: &config.AuthConfig{
				Type: "unknown",
			},
			wantType:  "",
			wantError: true,
		},
		{
			name: "empty type defaults to none",
			config: &config.AuthConfig{
				Type: "",
			},
			wantType:  "none",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth, err := factory.Create(tt.config)
			
			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, auth)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, auth)
				assert.Equal(t, tt.wantType, auth.Type())
			}
		})
	}
}

func TestAuthenticatorTypes(t *testing.T) {
	tests := []struct {
		name         string
		authenticator Authenticator
		expectedType string
	}{
		{
			name:         "bearer authenticator",
			authenticator: &bearerAuthenticator{},
			expectedType: "bearer",
		},
		{
			name:         "basic authenticator",
			authenticator: &basicAuthenticator{},
			expectedType: "basic",
		},
		{
			name:         "none authenticator",
			authenticator: &noneAuthenticator{},
			expectedType: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expectedType, tt.authenticator.Type())
		})
	}
}

// Benchmark tests
func BenchmarkBearerAuthenticator_Apply(b *testing.B) {
	auth := &bearerAuthenticator{token: "test-token"}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth.Apply(req)
	}
}

func BenchmarkBasicAuthenticator_Apply(b *testing.B) {
	auth := &basicAuthenticator{username: "user", password: "pass"}
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		auth.Apply(req)
	}
}