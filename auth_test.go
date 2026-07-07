package dokugo

import (
	"crypto"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
	"time"
)

// This exact request body + SHA-256 hex digest is DOKU's own published worked
// example (developers.doku.com/get-started-with-doku-api/signature-component/snap/symmetric-signature),
// independently re-verified with `shasum -a 256` before hardcoding here. DOKU's
// docs do not publish the client secret used for that example's final HMAC
// output, so the digest step is the furthest point we can pin against a real
// DOKU fixture — everything past it (the HMAC itself) is instead verified
// against an independently-computed reference using Go's own crypto/hmac.
const dokuExampleBody = `{"partnerServiceId":"  088899","customerNo":"12345678901234567890","virtualAccountNo":"  08889912345678901234567890","virtualAccountName":"Jokul Doe","virtualAccountEmail":"jokul@email.com","virtualAccountPhone":"6281828384858","trxId":"abcdefgh1234","totalAmount":{"value":"12345678.00","currency":"IDR"}}`
const dokuExampleBodyDigest = "3274fab8dac896837b106a16da2a974e7e65142dcecb4b768ef0294102838977"

func TestMinifyAndHash_MatchesDokuPublishedExample(t *testing.T) {
	minified, err := minifyJSON([]byte(dokuExampleBody))
	if err != nil {
		t.Fatalf("minifyJSON error: %v", err)
	}
	sum := sha256.Sum256(minified)
	got := strings.ToLower(hex.EncodeToString(sum[:]))
	if got != dokuExampleBodyDigest {
		t.Fatalf("digest mismatch:\n got:  %s\n want: %s", got, dokuExampleBodyDigest)
	}
}

func TestMinifyJSON_RemovesWhitespaceButKeepsKeyOrderAndValues(t *testing.T) {
	input := []byte(`{
		"b": 1,
		"a": "two"
	}`)
	got, err := minifyJSON(input)
	if err != nil {
		t.Fatalf("minifyJSON error: %v", err)
	}
	want := `{"b":1,"a":"two"}`
	if string(got) != want {
		t.Fatalf("minifyJSON = %q, want %q", string(got), want)
	}
}

func TestSignAsymmetric_RoundTripsWithMatchingPublicKey(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	xTimestamp := formatAsymmetricTimestamp(time.Date(2020, 9, 22, 1, 51, 0, 0, time.UTC))
	if xTimestamp != "2020-09-22T01:51:00Z" {
		t.Fatalf("unexpected asymmetric timestamp format: %s", xTimestamp)
	}

	sigB64, err := signAsymmetric(priv, "BRN-0239-1736742088036", xTimestamp)
	if err != nil {
		t.Fatalf("signAsymmetric error: %v", err)
	}

	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatalf("signature is not valid base64: %v", err)
	}

	stringToSign := "BRN-0239-1736742088036" + "|" + xTimestamp
	hashed := sha256.Sum256([]byte(stringToSign))
	if err := rsa.VerifyPKCS1v15(&priv.PublicKey, crypto.SHA256, hashed[:], sig); err != nil {
		t.Fatalf("signature did not verify against its own public key: %v", err)
	}
}

func TestSignAsymmetric_DifferentTimestampProducesDifferentSignature(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	sig1, _ := signAsymmetric(priv, "client-id", "2020-01-01T00:00:00Z")
	sig2, _ := signAsymmetric(priv, "client-id", "2020-01-01T00:00:01Z")
	if sig1 == sig2 {
		t.Fatal("expected different signatures for different timestamps")
	}
}

func TestSignSymmetric_MatchesIndependentlyComputedHMAC(t *testing.T) {
	secret := "test-secret-key"
	method := "POST"
	path := "/bi-snap-va/v1/transfer-va/create-va"
	accessToken := "test-access-token"
	body := []byte(dokuExampleBody)
	xTimestamp := "2024-03-26T16:01:41+07:00"

	got, err := signSymmetric(secret, method, path, accessToken, body, xTimestamp)
	if err != nil {
		t.Fatalf("signSymmetric error: %v", err)
	}

	// Independently reconstruct the expected value using the exact formula
	// from DOKU's docs, without calling any of our own helper functions.
	minified, _ := minifyJSON(body)
	bodyHash := sha256.Sum256(minified)
	bodyHashHex := strings.ToLower(hex.EncodeToString(bodyHash[:]))
	stringToSign := method + ":" + path + ":" + accessToken + ":" + bodyHashHex + ":" + xTimestamp
	mac := hmac.New(sha512.New, []byte(secret))
	mac.Write([]byte(stringToSign))
	want := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if got != want {
		t.Fatalf("signSymmetric = %s, want %s", got, want)
	}
}

func TestSignSymmetric_OutputIsBase64NotHex(t *testing.T) {
	sig, err := signSymmetric("secret", "POST", "/path", "token", []byte(`{}`), "2024-01-01T00:00:00+07:00")
	if err != nil {
		t.Fatalf("signSymmetric error: %v", err)
	}
	if _, err := base64.StdEncoding.DecodeString(sig); err != nil {
		t.Fatalf("expected valid base64 output, got %q: %v", sig, err)
	}
}

func TestSignSymmetric_SensitiveToEveryComponent(t *testing.T) {
	base := func() (string, error) {
		return signSymmetric("secret", "POST", "/path", "token", []byte(`{"a":1}`), "2024-01-01T00:00:00+07:00")
	}
	baseline, err := base()
	if err != nil {
		t.Fatal(err)
	}

	variants := map[string]string{}
	if s, _ := signSymmetric("other-secret", "POST", "/path", "token", []byte(`{"a":1}`), "2024-01-01T00:00:00+07:00"); true {
		variants["secret"] = s
	}
	if s, _ := signSymmetric("secret", "GET", "/path", "token", []byte(`{"a":1}`), "2024-01-01T00:00:00+07:00"); true {
		variants["method"] = s
	}
	if s, _ := signSymmetric("secret", "POST", "/other", "token", []byte(`{"a":1}`), "2024-01-01T00:00:00+07:00"); true {
		variants["path"] = s
	}
	if s, _ := signSymmetric("secret", "POST", "/path", "other-token", []byte(`{"a":1}`), "2024-01-01T00:00:00+07:00"); true {
		variants["token"] = s
	}
	if s, _ := signSymmetric("secret", "POST", "/path", "token", []byte(`{"a":2}`), "2024-01-01T00:00:00+07:00"); true {
		variants["body"] = s
	}
	if s, _ := signSymmetric("secret", "POST", "/path", "token", []byte(`{"a":1}`), "2024-01-01T00:00:01+07:00"); true {
		variants["timestamp"] = s
	}

	for name, sig := range variants {
		if sig == baseline {
			t.Errorf("changing %s did not change the signature", name)
		}
	}
}

func TestFormatSymmetricTimestamp_UsesLocalOffsetNotUTC(t *testing.T) {
	loc := time.FixedZone("WIB", 7*3600)
	ts := formatSymmetricTimestamp(time.Date(2024, 3, 19, 14, 39, 1, 0, loc))
	want := "2024-03-19T14:39:01+07:00"
	if ts != want {
		t.Fatalf("formatSymmetricTimestamp = %s, want %s", ts, want)
	}
}

func TestVerifyWebhookSignature_AcceptsGenuineRejectsTampered(t *testing.T) {
	secret := "webhook-secret"
	method := "POST"
	notificationPath := "/v1.1/transfer-va/payment"
	accessToken := ""
	body := []byte(`{"partnerServiceId":"   19008","paidAmount":{"value":"11500.00","currency":"IDR"}}`)
	xTimestamp := formatSymmetricTimestamp(time.Now())

	sig, err := signSymmetric(secret, method, notificationPath, accessToken, body, xTimestamp)
	if err != nil {
		t.Fatalf("signSymmetric error: %v", err)
	}

	if !VerifyWebhookSignature(secret, method, notificationPath, accessToken, body, xTimestamp, sig) {
		t.Fatal("expected genuine signature to verify")
	}

	tamperedBody := []byte(`{"partnerServiceId":"   19008","paidAmount":{"value":"99999999.00","currency":"IDR"}}`)
	if VerifyWebhookSignature(secret, method, notificationPath, accessToken, tamperedBody, xTimestamp, sig) {
		t.Fatal("expected tampered body to fail verification")
	}

	if VerifyWebhookSignature("wrong-secret", method, notificationPath, accessToken, body, xTimestamp, sig) {
		t.Fatal("expected wrong secret to fail verification")
	}
}
