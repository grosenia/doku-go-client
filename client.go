package dokugo

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const tokenEndpointPath = "/authorization/v1/access-token/b2b"

// tokenSafetyMargin is subtracted from DOKU's reported expiresIn so a token
// nearing expiry is refreshed slightly early rather than failing mid-request.
const tokenSafetyMargin = 60 * time.Second

// Client holds DOKU SNAP credentials and HTTP config. BaseURL is a plain,
// overridable string (not an enum) because DOKU has genuinely different
// sandbox/production hosts, and because httptest-based tests need to point it
// at a mock server.
type Client struct {
	BaseURL    string
	ClientID   string // DOKU's "X-PARTNER-ID" / "X-CLIENT-KEY"
	SecretKey  string // used for symmetric HMAC-SHA512 signing
	PrivateKey *rsa.PrivateKey
	HTTPClient *http.Client

	tokenMu     sync.Mutex
	cachedToken string
	tokenExpiry time.Time
}

// NewClient parses privateKeyPEM once, failing fast on invalid key material
// rather than deferring the failure to the first signed request.
func NewClient(clientID, secretKey string, privateKeyPEM []byte, sandbox bool) (Client, error) {
	block, _ := pem.Decode(privateKeyPEM)
	if block == nil {
		return Client{}, errors.New("doku: invalid private key PEM")
	}
	key, err := parseRSAPrivateKey(block.Bytes)
	if err != nil {
		return Client{}, fmt.Errorf("doku: parse private key: %w", err)
	}

	baseURL := DefaultProductionBaseURL
	if sandbox {
		baseURL = DefaultSandboxBaseURL
	}

	return Client{
		BaseURL:    baseURL,
		ClientID:   clientID,
		SecretKey:  secretKey,
		PrivateKey: key,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func parseRSAPrivateKey(der []byte) (*rsa.PrivateKey, error) {
	if key, err := x509.ParsePKCS1PrivateKey(der); err == nil {
		return key, nil
	}
	keyIface, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		return nil, err
	}
	key, ok := keyIface.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("doku: private key is not RSA")
	}
	return key, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

type accessTokenResponse struct {
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
	AccessToken     string `json:"accessToken"`
	TokenType       string `json:"tokenType"`
	ExpiresIn       int    `json:"expiresIn"`
	AdditionalInfo  string `json:"additionalInfo"`
}

// getAccessToken returns a cached bearer token, requesting a new one from
// DOKU only if the cache is empty or within tokenSafetyMargin of expiry.
// DOKU tokens expire in 900s (15 min) per its docs.
func (c *Client) getAccessToken() (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()

	if c.cachedToken != "" && time.Now().Before(c.tokenExpiry) {
		return c.cachedToken, nil
	}

	token, expiresIn, err := c.requestNewAccessToken()
	if err != nil {
		return "", err
	}
	c.cachedToken = token
	c.tokenExpiry = time.Now().Add(time.Duration(expiresIn)*time.Second - tokenSafetyMargin)
	return c.cachedToken, nil
}

func (c *Client) requestNewAccessToken() (token string, expiresIn int, err error) {
	xTimestamp := formatAsymmetricTimestamp(time.Now())
	signature, err := signAsymmetric(c.PrivateKey, c.ClientID, xTimestamp)
	if err != nil {
		return "", 0, err
	}

	body := []byte(`{"grantType":"client_credentials"}`)
	req, err := http.NewRequest(http.MethodPost, c.BaseURL+tokenEndpointPath, bytes.NewReader(body))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-TIMESTAMP", xTimestamp)
	req.Header.Set("X-SIGNATURE", signature)
	req.Header.Set("X-CLIENT-KEY", c.ClientID)

	res, err := c.httpClient().Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("doku: request access token: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", 0, fmt.Errorf("doku: read access token response: %w", err)
	}

	var tokenResp accessTokenResponse
	if err := json.Unmarshal(respBytes, &tokenResp); err != nil {
		return "", 0, fmt.Errorf("doku: unmarshal access token response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 || tokenResp.AccessToken == "" {
		return "", 0, fmt.Errorf("doku: get access token failed: [%s] %s", tokenResp.ResponseCode, tokenResp.ResponseMessage)
	}
	return tokenResp.AccessToken, tokenResp.ExpiresIn, nil
}
