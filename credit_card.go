package dokugo

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	pathCreatePaymentPage = "/credit-card/v1/payment-page"
	pathCreditCardCapture = "/credit-card/capture"
)

// CreditCardClient talks to DOKU's Card product (Payment Page / DOKU JS,
// non-PCI-DSS path) — a separate credential set and signature scheme from
// the SNAP Client/Gateway used elsewhere in this package (see
// types_credit_card.go's package doc comment). ClientID here is this
// product's own Client-Id (e.g. "MCH-0001-..."), not SNAP's "BRN-..." one —
// don't assume they're interchangeable without confirming against DOKU's
// dashboard.
type CreditCardClient struct {
	BaseURL    string
	ClientID   string
	SecretKey  string
	HTTPClient *http.Client
}

// NewCreditCardClient constructs a client for DOKU's Card product.
func NewCreditCardClient(clientID, secretKey string, sandbox bool) *CreditCardClient {
	baseURL := "https://api.doku.com"
	if sandbox {
		baseURL = "https://api-sandbox.doku.com"
	}
	return &CreditCardClient{
		BaseURL:    baseURL,
		ClientID:   clientID,
		SecretKey:  secretKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *CreditCardClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

// nonSNAPTimestampLayout is UTC with no offset (e.g. "2020-08-11T08:45:42Z")
// — same format as SNAP's asymmetric endpoint, confirmed against DOKU's own
// worked examples for this product 2026-08-24.
const nonSNAPTimestampLayout = "2006-01-02T15:04:05Z"

func formatNonSNAPTimestamp(t time.Time) string {
	return t.UTC().Format(nonSNAPTimestampLayout)
}

// generateRequestID returns a random 32-hex-char string — DOKU only
// requires "unique random string, max 128 chars", not literally a UUID
// (their own docs examples happen to show UUID-shaped values, but a plain
// random hex string satisfies the stated requirement without pulling in a
// UUID dependency).
func generateRequestID() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("doku: generate request id: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// digestBody returns base64(sha256(body)) — DOKU calls this the "Digest"
// component, required for POST requests only.
func digestBody(body []byte) string {
	sum := sha256.Sum256(body)
	return base64.StdEncoding.EncodeToString(sum[:])
}

// signNonSNAP implements DOKU's non-SNAP HMAC-SHA256 signature (confirmed
// 2026-08-24 against developers.doku.com's "Signature Component from
// Request Header" page, verbatim worked example):
//
//	stringToSign = "Client-Id:" + clientID + "\n" +
//	               "Request-Id:" + requestID + "\n" +
//	               "Request-Timestamp:" + timestamp + "\n" +
//	               "Request-Target:" + requestTarget +
//	               ["\n" + "Digest:" + digest]   // only for POST (digest != "")
//	signature    = "HMACSHA256=" + base64(HMAC_SHA256(secretKey, stringToSign))
//
// No trailing newline. GET requests (e.g. Check Status) omit the Digest
// line entirely — pass digest="" for those.
func signNonSNAP(secretKey, clientID, requestID, timestamp, requestTarget, digest string) string {
	stringToSign := "Client-Id:" + clientID + "\n" +
		"Request-Id:" + requestID + "\n" +
		"Request-Timestamp:" + timestamp + "\n" +
		"Request-Target:" + requestTarget
	if digest != "" {
		stringToSign += "\n" + "Digest:" + digest
	}
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(stringToSign))
	return "HMACSHA256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// doRequest marshals reqBody, signs per signNonSNAP, executes the request,
// and unmarshals the response. Mirrors gateway.go's doRequest but for this
// product's distinct header set (Client-Id/Request-Id/Request-Timestamp/
// Signature, no Bearer token — this product doesn't use the SNAP access-token
// flow at all).
func (c *CreditCardClient) doRequest(method, path string, reqBody, respBody any) (int, error) {
	var bodyBytes []byte
	var err error
	if reqBody != nil {
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("doku: marshal request: %w", err)
		}
	}

	requestID, err := generateRequestID()
	if err != nil {
		return 0, err
	}
	timestamp := formatNonSNAPTimestamp(time.Now())
	digest := ""
	if method == http.MethodPost {
		digest = digestBody(bodyBytes)
	}
	signature := signNonSNAP(c.SecretKey, c.ClientID, requestID, timestamp, path, digest)

	var httpReq *http.Request
	if bodyBytes != nil {
		httpReq, err = http.NewRequest(method, c.BaseURL+path, bytes.NewReader(bodyBytes))
	} else {
		httpReq, err = http.NewRequest(method, c.BaseURL+path, nil)
	}
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Client-Id", c.ClientID)
	httpReq.Header.Set("Request-Id", requestID)
	httpReq.Header.Set("Request-Timestamp", timestamp)
	httpReq.Header.Set("Signature", signature)
	if digest != "" {
		httpReq.Header.Set("Digest", digest)
	}

	res, err := c.httpClient().Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("doku: execute request: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, fmt.Errorf("doku: read response: %w", err)
	}

	if len(respBytes) > 0 && respBody != nil {
		if err := json.Unmarshal(respBytes, respBody); err != nil {
			return res.StatusCode, fmt.Errorf("doku: unmarshal response: %w", err)
		}
	}

	if marker, ok := respBody.(errorMarker); ok {
		marker.markHTTPError(res.StatusCode)
	}

	return res.StatusCode, nil
}

// CreatePaymentPage generates a hosted Card payment page URL (or a DOKU JS
// session) for the given order — see CreatePaymentPageRequest's doc comment.
func (c *CreditCardClient) CreatePaymentPage(req *CreatePaymentPageRequest) (*CreatePaymentPageResponse, error) {
	resp := &CreatePaymentPageResponse{}
	_, err := c.doRequest(http.MethodPost, pathCreatePaymentPage, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Capture completes an AUTHORIZE transaction (must be called within 7 days
// of authorization, per DOKU's docs, or the hold auto-releases).
func (c *CreditCardClient) Capture(req *CaptureRequest) (*CaptureResponse, error) {
	resp := &CaptureResponse{}
	_, err := c.doRequest(http.MethodPost, pathCreditCardCapture, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
