package dokugo

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"time"
)

// tokenURLTimestampMaxSkew bounds how far off xTimestamp may be from now when
// verifying an inbound Token URL request, mirroring the "reject stale/replayed
// requests" concern any signed inbound webhook needs. DOKU's own tokens live
// for 900s; a generous 5-minute clock-skew allowance is unrelated to that and
// only guards against replay of a captured request.
const tokenURLTimestampMaxSkew = 5 * time.Minute

// ParseRSAPublicKeyPEM parses a PEM-encoded RSA public key, e.g. the "DOKU
// Public Key" value from DOKU's dashboard (Settings > API Keys > DOKU Public
// Key), used to verify inbound Token URL requests.
func ParseRSAPublicKeyPEM(pemBytes []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("doku: invalid public key PEM")
	}
	keyIface, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Some tools emit PKCS1-wrapped RSA public keys instead of PKIX.
		return x509.ParsePKCS1PublicKey(block.Bytes)
	}
	key, ok := keyIface.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("doku: public key is not RSA")
	}
	return key, nil
}

// VerifyTokenURLRequestSignature verifies DOKU's inbound "Get Token" request
// to our own Token URL (required for VA Direct Inquiry / DIPC — DOKU calls us
// first to obtain a bearer token before calling our Inquiry/Payment URLs).
// Confirmed by DOKU support 2026-07-14: verification uses the dashboard's
// "DOKU Public Key" against the same asymmetric scheme as the B2B Get Token
// endpoint (X-SIGNATURE = sign(xClientKey + "|" + xTimestamp) with DOKU's
// private key), just with the roles reversed.
func VerifyTokenURLRequestSignature(dokuPublicKeyPEM []byte, xClientKey, xTimestamp, xSignature string) (bool, error) {
	pubKey, err := ParseRSAPublicKeyPEM(dokuPublicKeyPEM)
	if err != nil {
		return false, err
	}
	ts, err := time.Parse(asymmetricTimestampLayout, xTimestamp)
	if err != nil {
		return false, errors.New("doku: invalid X-TIMESTAMP")
	}
	if skew := time.Since(ts); skew < -tokenURLTimestampMaxSkew || skew > tokenURLTimestampMaxSkew {
		return false, nil
	}
	return verifyAsymmetric(pubKey, xClientKey, xTimestamp, xSignature), nil
}

// TokenURLResponse is the response body OUR Token URL handler must return —
// confirmed by DOKU support 2026-07-14 to mirror DOKU's own B2B Get Token API
// response shape (developers.doku.com's "Get Token API (B2B)" page, "API
// Response Body" section), just issued by us instead of DOKU.
type TokenURLResponse struct {
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
	AccessToken     string `json:"accessToken"`
	TokenType       string `json:"tokenType"`
	ExpiresIn       int    `json:"expiresIn"`
}

// NewTokenURLResponse builds a successful Token URL response. responseCode
// "2007300" follows DOKU's observed "2 + 00 + serviceCode + 00" pattern
// (Get Token/73 per the B2B docs) — not a literal DOKU-confirmed example for
// this reversed direction, verify against a real DOKU request if it matters.
func NewTokenURLResponse(accessToken string, expiresInSeconds int) TokenURLResponse {
	return TokenURLResponse{
		ResponseCode:    "2007300",
		ResponseMessage: "Successful",
		AccessToken:     accessToken,
		TokenType:       "Bearer",
		ExpiresIn:       expiresInSeconds,
	}
}
