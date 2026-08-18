package dokugo

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// This file implements DOKU's LEGACY (non-SNAP) KirimDOKU API — a completely
// separate system from the rest of this package (which is all SNAP). See
// docs/KIRIMDOKU_LEGACY_API_REFERENCE.md for the full captured docs and why
// this exists alongside the SNAP disbursement flow in disbursement.go.
//
// Auth is a fixed agentKey + AES-128/ECB/PKCS7 "signature" (not SNAP's
// HMAC/RSA scheme) — confirmed working live against DOKU's real staging
// sandbox 2026-08-14 (agentKey A47438 / encryptionKey pl6jn16fvkb64fit).

const (
	KirimDokuLegacySandboxBaseURL    = "https://staging.doku.com/apikirimdoku"
	KirimDokuLegacyProductionBaseURL = "https://kirimdoku.com/v2/api"
)

// KirimDokuLegacyClient holds legacy KirimDOKU credentials/HTTP config.
// Deliberately a separate type from Client — different host, different auth
// scheme, unrelated to SNAP. Construct via NewKirimDokuLegacyClient.
type KirimDokuLegacyClient struct {
	BaseURL       string
	AgentKey      string
	EncryptionKey string // 16-char AES key issued by KIRIMDOKU
	HTTPClient    *http.Client
}

func NewKirimDokuLegacyClient(agentKey, encryptionKey string, sandbox bool) *KirimDokuLegacyClient {
	baseURL := KirimDokuLegacyProductionBaseURL
	if sandbox {
		baseURL = KirimDokuLegacySandboxBaseURL
	}
	return &KirimDokuLegacyClient{
		BaseURL:       baseURL,
		AgentKey:      agentKey,
		EncryptionKey: encryptionKey,
		HTTPClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *KirimDokuLegacyClient) httpClient() *http.Client {
	if c.HTTPClient != nil {
		return c.HTTPClient
	}
	return http.DefaultClient
}

// newRequestID generates a partner-side unique request identifier — the
// docs require "max 36 alphanumeric, e.g. a UUID" and their own example is
// UUID-v4-shaped (hyphens included), so a real UUID v4 is used here.
func newRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// sign implements the "SharedKey Hash Value" scheme: AES-encrypt
// (agentKey+requestId) with EncryptionKey (AES-128/ECB/PKCS7, per the docs'
// Java sample using bare Cipher.getInstance("AES"), whose JCE default is
// ECB/PKCS5 — equivalent to PKCS7 at the 16-byte AES block size), then
// base64-encode. Go's stdlib has no ECB mode (intentionally, it's insecure
// for general use) so it's implemented here block-by-block to match DOKU's
// scheme exactly.
func (c *KirimDokuLegacyClient) sign(requestID string) (string, error) {
	block, err := aes.NewCipher([]byte(c.EncryptionKey))
	if err != nil {
		return "", fmt.Errorf("doku: kirimdoku legacy AES key: %w", err)
	}
	plaintext := []byte(c.AgentKey + requestID)
	padded := pkcs7Pad(plaintext, block.BlockSize())
	encrypted := make([]byte, len(padded))
	for i := 0; i < len(padded); i += block.BlockSize() {
		block.Encrypt(encrypted[i:i+block.BlockSize()], padded[i:i+block.BlockSize()])
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

func pkcs7Pad(data []byte, blockSize int) []byte {
	padLen := blockSize - len(data)%blockSize
	return append(data, bytes.Repeat([]byte{byte(padLen)}, padLen)...)
}

// authFields is embedded into requests whose auth triple travels in the JSON
// body (Ping, CheckBalance) — see doRequestBodyAuth.
type authFields struct {
	AgentKey  string `json:"agentKey"`
	RequestID string `json:"requestId"`
	Signature string `json:"signature"`
}

func (c *KirimDokuLegacyClient) newAuthFields() (authFields, error) {
	requestID, err := newRequestID()
	if err != nil {
		return authFields{}, err
	}
	signature, err := c.sign(requestID)
	if err != nil {
		return authFields{}, err
	}
	return authFields{AgentKey: c.AgentKey, RequestID: requestID, Signature: signature}, nil
}

// doRequestBodyAuth is for endpoints where agentKey/requestId/signature are
// JSON body fields (Ping, CheckBalance) — reqBody must embed authFields.
func (c *KirimDokuLegacyClient) doRequestBodyAuth(path string, reqBody interface{}, respBody interface{}) error {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("doku: marshal kirimdoku legacy request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return c.execute(httpReq, respBody)
}

// doRequestHeaderAuth is for endpoints where agentKey/requestId/signature
// travel as HTTP headers instead (CashInInquiry, CashInRemit, TransactionInfo
// — confirmed for the first two against real sandbox calls; TransactionInfo
// follows the same convention since its documented body has no auth fields
// at all, but this specific case isn't independently confirmed).
func (c *KirimDokuLegacyClient) doRequestHeaderAuth(path string, reqBody interface{}, respBody interface{}) error {
	auth, err := c.newAuthFields()
	if err != nil {
		return err
	}
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("doku: marshal kirimdoku legacy request: %w", err)
	}
	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("agentKey", auth.AgentKey)
	httpReq.Header.Set("requestId", auth.RequestID)
	httpReq.Header.Set("signature", auth.Signature)
	return c.execute(httpReq, respBody)
}

func (c *KirimDokuLegacyClient) execute(httpReq *http.Request, respBody interface{}) error {
	res, err := c.httpClient().Do(httpReq)
	if err != nil {
		return fmt.Errorf("doku: execute kirimdoku legacy request: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("doku: read kirimdoku legacy response: %w", err)
	}
	if len(respBytes) > 0 && respBody != nil {
		if err := json.Unmarshal(respBytes, respBody); err != nil {
			return fmt.Errorf("doku: unmarshal kirimdoku legacy response: %w", err)
		}
	}
	return nil
}

// Ping checks connection availability.
func (c *KirimDokuLegacyClient) Ping() (*KirimDokuLegacyPingResponse, error) {
	auth, err := c.newAuthFields()
	if err != nil {
		return nil, err
	}
	resp := &KirimDokuLegacyPingResponse{}
	if err := c.doRequestBodyAuth("/ping", struct {
		authFields
	}{auth}, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CheckBalance checks the partner's own KIRIMDOKU deposit balance.
func (c *KirimDokuLegacyClient) CheckBalance() (*KirimDokuLegacyCheckBalanceResponse, error) {
	auth, err := c.newAuthFields()
	if err != nil {
		return nil, err
	}
	resp := &KirimDokuLegacyCheckBalanceResponse{}
	if err := c.doRequestBodyAuth("/checkbalance", struct {
		authFields
	}{auth}, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CashInInquiry validates a beneficiary account and returns the fee/idToken
// needed for a follow-up CashInRemit call.
func (c *KirimDokuLegacyClient) CashInInquiry(req *KirimDokuLegacyInquiryRequest) (*KirimDokuLegacyInquiryResponse, error) {
	resp := &KirimDokuLegacyInquiryResponse{}
	if err := c.doRequestHeaderAuth("/cashin/inquiry", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// CashInRemit executes the actual transfer, using the idToken from a prior
// CashInInquiry call.
func (c *KirimDokuLegacyClient) CashInRemit(req *KirimDokuLegacyRemitRequest) (*KirimDokuLegacyRemitResponse, error) {
	resp := &KirimDokuLegacyRemitResponse{}
	if err := c.doRequestHeaderAuth("/cashin/remit", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// TransactionInfo gets a transaction's status — the legacy-API equivalent of
// CheckDisbursementStatus for the SNAP API. transactionID accepts either
// DOKU's own transaction ID (e.g. "DK0826666") or an inquiry's idToken.
func (c *KirimDokuLegacyClient) TransactionInfo(transactionID string) (*KirimDokuLegacyTransactionInfoResponse, error) {
	resp := &KirimDokuLegacyTransactionInfoResponse{}
	req := struct {
		TransactionID string `json:"transactionId"`
	}{transactionID}
	if err := c.doRequestHeaderAuth("/transaction/info", req, resp); err != nil {
		return nil, err
	}
	return resp, nil
}
