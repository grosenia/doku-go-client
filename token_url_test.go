package dokugo

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func generateTestRSAKeyPair(t *testing.T) (privPEM, pubPEM []byte, priv *rsa.PrivateKey) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	privDER := x509.MarshalPKCS1PrivateKey(priv)
	privPEM = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privDER})

	pubDER, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	pubPEM = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubDER})
	return
}

func TestVerifyTokenURLRequestSignature_Valid(t *testing.T) {
	_, pubPEM, priv := generateTestRSAKeyPair(t)

	xTimestamp := formatAsymmetricTimestamp(time.Now())
	clientID := "test-client-id"
	sig, err := signAsymmetric(priv, clientID, xTimestamp)
	if err != nil {
		t.Fatalf("signAsymmetric: %v", err)
	}

	ok, err := VerifyTokenURLRequestSignature(pubPEM, clientID, xTimestamp, sig)
	if err != nil {
		t.Fatalf("VerifyTokenURLRequestSignature error: %v", err)
	}
	if !ok {
		t.Fatal("expected a genuine signature to verify")
	}
}

func TestVerifyTokenURLRequestSignature_TamperedSignature(t *testing.T) {
	_, pubPEM, priv := generateTestRSAKeyPair(t)
	xTimestamp := formatAsymmetricTimestamp(time.Now())
	sig, _ := signAsymmetric(priv, "test-client-id", xTimestamp)

	ok, err := VerifyTokenURLRequestSignature(pubPEM, "a-different-client-id", xTimestamp, sig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected verification to fail when clientID doesn't match what was signed")
	}
}

func TestVerifyTokenURLRequestSignature_WrongKey(t *testing.T) {
	_, _, priv := generateTestRSAKeyPair(t)
	_, otherPubPEM, _ := generateTestRSAKeyPair(t)

	xTimestamp := formatAsymmetricTimestamp(time.Now())
	sig, _ := signAsymmetric(priv, "test-client-id", xTimestamp)

	ok, err := VerifyTokenURLRequestSignature(otherPubPEM, "test-client-id", xTimestamp, sig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected verification to fail against the wrong public key")
	}
}

func TestVerifyTokenURLRequestSignature_StaleTimestamp(t *testing.T) {
	_, pubPEM, priv := generateTestRSAKeyPair(t)
	staleTimestamp := formatAsymmetricTimestamp(time.Now().Add(-1 * time.Hour))
	sig, _ := signAsymmetric(priv, "test-client-id", staleTimestamp)

	ok, err := VerifyTokenURLRequestSignature(pubPEM, "test-client-id", staleTimestamp, sig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected verification to reject a stale (replay-window-exceeded) timestamp")
	}
}

func TestNewTokenURLResponse(t *testing.T) {
	resp := NewTokenURLResponse("abc123", 900)
	if resp.AccessToken != "abc123" || resp.TokenType != "Bearer" || resp.ExpiresIn != 900 {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
