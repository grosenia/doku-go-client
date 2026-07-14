package dokugo

import (
	"bytes"
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// asymmetricTimestampLayout is UTC with NO offset (e.g. "2020-09-22T01:51:00Z"),
// required specifically for the access-token endpoint's string-to-sign. This is
// a different format than every transactional call, which uses local time
// WITH an offset — mixing these up produces a signature DOKU will silently
// reject as "Unauthorized" with no other clue, so keep the two formatters
// distinct rather than trying to share one.
const asymmetricTimestampLayout = "2006-01-02T15:04:05Z"

// symmetricTimestampLayout is local time WITH offset (e.g. "2024-03-19T14:39:01+07:00"),
// required for every transactional call and for inbound webhook signature verification.
const symmetricTimestampLayout = "2006-01-02T15:04:05Z07:00"

func formatAsymmetricTimestamp(t time.Time) string {
	return t.UTC().Format(asymmetricTimestampLayout)
}

func formatSymmetricTimestamp(t time.Time) string {
	return t.Format(symmetricTimestampLayout)
}

// signAsymmetric signs `clientID + "|" + xTimestampUTC` with the merchant's
// RSA private key (SHA256withRSA / PKCS1v15), used ONLY for the access-token
// (Get Token B2B) endpoint. Returns the base64-encoded signature.
func signAsymmetric(privateKey *rsa.PrivateKey, clientID, xTimestampUTC string) (string, error) {
	stringToSign := clientID + "|" + xTimestampUTC
	hashed := sha256.Sum256([]byte(stringToSign))
	sig, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, hashed[:])
	if err != nil {
		return "", fmt.Errorf("doku: sign asymmetric: %w", err)
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// verifyAsymmetric checks a base64 signature over `clientID + "|" + xTimestampUTC`
// against the given RSA public key (SHA256withRSA / PKCS1v15) — the inverse of
// signAsymmetric. Used to verify a request DOKU sends TO us (e.g. the Token
// URL call for VA Direct Inquiry), where DOKU signs with its private key and
// we verify with "DOKU Public Key" from the dashboard.
func verifyAsymmetric(publicKey *rsa.PublicKey, clientID, xTimestampUTC, signatureB64 string) bool {
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return false
	}
	stringToSign := clientID + "|" + xTimestampUTC
	hashed := sha256.Sum256([]byte(stringToSign))
	return rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hashed[:], sig) == nil
}

// minifyJSON removes insignificant whitespace without reordering keys or
// altering values (unlike round-tripping through a map), matching DOKU's
// "minify the request body" step before hashing it.
func minifyJSON(body []byte) ([]byte, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, body); err != nil {
		return nil, fmt.Errorf("doku: minify json: %w", err)
	}
	return buf.Bytes(), nil
}

// signSymmetric implements DOKU's SNAP symmetric signature:
//
//	stringToSign = HTTPMethod + ":" + EndpointUrl + ":" + AccessToken + ":" +
//	               lowercase(hex(sha256(minify(body)))) + ":" + TimeStamp
//	signature    = base64(HMAC_SHA512(clientSecret, stringToSign))
//
// endpointPath is DOKU's path when WE call DOKU. When verifying an INBOUND
// webhook instead, pass the merchant's OWN notification path — per DOKU's
// docs, the signed "Request-Target" is always the path of whichever side is
// being called, not always DOKU's.
//
// NOTE: DOKU's docs describe this formula for outbound calls; the "AccessToken"
// segment's exact role for an inbound webhook notification (where DOKU is
// calling us, so no bearer token was issued for that specific call) is not
// spelled out in public docs. VerifyWebhookSignature currently threads through
// whatever value the caller supplies for accessToken — confirm the correct
// value (likely empty string, or omitted from the string-to-sign entirely)
// against a real DOKU-signed notification before relying on this in production.
func signSymmetric(secretKey, httpMethod, endpointPath, accessToken string, body []byte, xTimestamp string) (string, error) {
	minified, err := minifyJSON(body)
	if err != nil {
		return "", err
	}
	bodyHash := sha256.Sum256(minified)
	bodyHashHex := strings.ToLower(hex.EncodeToString(bodyHash[:]))

	stringToSign := httpMethod + ":" + endpointPath + ":" + accessToken + ":" + bodyHashHex + ":" + xTimestamp

	mac := hmac.New(sha512.New, []byte(secretKey))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil)), nil
}

// VerifyWebhookSignature re-derives the symmetric signature over the
// merchant's own notification path and compares it against the inbound
// X-SIGNATURE header using a constant-time comparison. See the accessToken
// caveat on signSymmetric above.
func VerifyWebhookSignature(secretKey, httpMethod, notificationPath, accessToken string, rawBody []byte, xTimestamp, signatureHeader string) bool {
	expected, err := signSymmetric(secretKey, httpMethod, notificationPath, accessToken, rawBody, xTimestamp)
	if err != nil {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(signatureHeader))
}
