package dokugo

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"testing"
)

// generateTestRSAKeyPEM returns a freshly generated RSA private key, PEM
// (PKCS1) encoded, suitable for constructing a test Client. Real Grosenia
// keys would be provisioned once and stored securely, but tests need a fresh,
// disposable keypair each run.
func generateTestRSAKeyPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test RSA key: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	return pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: der})
}

// mockTokenHandler always returns a valid, long-lived (per the mock's own
// expiresIn) access token — used as the token-endpoint handler in scenario
// tests that aren't specifically exercising token-caching/refresh behavior.
func mockTokenHandler(accessToken string, expiresIn int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(accessTokenResponse{
			ResponseCode:    "2007300",
			ResponseMessage: "Successful",
			AccessToken:     accessToken,
			TokenType:       "Bearer",
			ExpiresIn:       expiresIn,
		})
	}
}

// mockGateway spins up an httptest server multiplexing the token endpoint
// (via tokenHandler) and the given API path (via apiHandler), and returns a
// Gateway whose Client.BaseURL points at it.
func mockGateway(t *testing.T, apiPath string, tokenHandler, apiHandler http.HandlerFunc) (*Gateway, *httptest.Server) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(tokenEndpointPath, tokenHandler)
	mux.HandleFunc(apiPath, apiHandler)

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	keyPEM := generateTestRSAKeyPEM(t)
	client, err := NewClient("test-client-id", "test-secret", keyPEM, true)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	client.BaseURL = srv.URL

	return NewGateway(&client), srv
}
