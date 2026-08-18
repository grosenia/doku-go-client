package dokugo

import (
	"crypto/aes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testKirimDokuLegacyEncryptionKey = "pl6jn16fvkb64fit" // 16 chars, docs' own sample key

// decryptTestSignature reverses sign() so tests can assert on plaintext
// (agentKey+requestId) instead of duplicating the AES call to compare
// ciphertext, which would just be testing the test.
func decryptTestSignature(t *testing.T, signature, key string) string {
	t.Helper()
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	ciphertext, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		t.Fatalf("base64 decode signature: %v", err)
	}
	plaintext := make([]byte, len(ciphertext))
	for i := 0; i < len(ciphertext); i += block.BlockSize() {
		block.Decrypt(plaintext[i:i+block.BlockSize()], ciphertext[i:i+block.BlockSize()])
	}
	padLen := int(plaintext[len(plaintext)-1])
	return string(plaintext[:len(plaintext)-padLen])
}

func TestKirimDokuLegacySign_RoundTrips(t *testing.T) {
	c := NewKirimDokuLegacyClient("A47438", testKirimDokuLegacyEncryptionKey, true)
	requestID := "4201243b-c0a3-495f-8464-bad5f4a2e9d4"

	signature, err := c.sign(requestID)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	got := decryptTestSignature(t, signature, testKirimDokuLegacyEncryptionKey)
	want := c.AgentKey + requestID
	if got != want {
		t.Errorf("decrypted signature = %q, want %q", got, want)
	}
}

func TestKirimDokuLegacyPing_BodyAuth(t *testing.T) {
	var captured authFields
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(KirimDokuLegacyPingResponse{Status: 0, Message: "Ok"})
	}))
	defer srv.Close()

	c := NewKirimDokuLegacyClient("A47438", testKirimDokuLegacyEncryptionKey, true)
	c.BaseURL = srv.URL

	resp, err := c.Ping()
	if err != nil {
		t.Fatalf("Ping: %v", err)
	}
	if resp.Status != 0 || resp.Message != "Ok" {
		t.Errorf("Ping response = %+v, want status 0 Ok", resp)
	}
	if captured.AgentKey != "A47438" {
		t.Errorf("body agentKey = %q, want A47438", captured.AgentKey)
	}
	plaintext := decryptTestSignature(t, captured.Signature, testKirimDokuLegacyEncryptionKey)
	if plaintext != captured.AgentKey+captured.RequestID {
		t.Errorf("signature does not match agentKey+requestId: got plaintext %q", plaintext)
	}
}

func TestKirimDokuLegacyCashInInquiry_HeaderAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cashin/inquiry" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		agentKey := r.Header.Get("agentKey")
		requestID := r.Header.Get("requestId")
		signature := r.Header.Get("signature")
		if agentKey == "" || requestID == "" || signature == "" {
			t.Fatalf("missing auth headers: agentKey=%q requestId=%q signature=%q", agentKey, requestID, signature)
		}
		plaintext := decryptTestSignature(t, signature, testKirimDokuLegacyEncryptionKey)
		if plaintext != agentKey+requestID {
			t.Errorf("header signature mismatch: got plaintext %q, want %q", plaintext, agentKey+requestID)
		}

		var body KirimDokuLegacyInquiryRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		if body.BeneficiaryAccount == nil || body.BeneficiaryAccount.Number != "0803944123" {
			t.Errorf("beneficiaryAccount not round-tripped correctly: %+v", body.BeneficiaryAccount)
		}

		_ = json.NewEncoder(w).Encode(KirimDokuLegacyInquiryResponse{
			Status:  0,
			Inquiry: KirimDokuLegacyInquiryData{IDToken: "I0828364575432248"},
		})
	}))
	defer srv.Close()

	c := NewKirimDokuLegacyClient("A47438", testKirimDokuLegacyEncryptionKey, true)
	c.BaseURL = srv.URL

	resp, err := c.CashInInquiry(&KirimDokuLegacyInquiryRequest{
		Channel:             KirimDokuLegacyChannel{Code: KirimDokuLegacyChannelBank},
		SenderCountry:       KirimDokuLegacyCountry{Code: "ID"},
		SenderCurrency:      KirimDokuLegacyCurrency{Code: "IDR"},
		SenderAmount:        "50000",
		BeneficiaryCountry:  KirimDokuLegacyCountry{Code: "ID"},
		BeneficiaryCurrency: KirimDokuLegacyCurrency{Code: "IDR"},
		BeneficiaryAccount: &KirimDokuLegacyBeneficiaryAccount{
			Number: "0803944123",
			Bank:   KirimDokuLegacyBank{ID: "014"},
		},
	})
	if err != nil {
		t.Fatalf("CashInInquiry: %v", err)
	}
	if resp.Inquiry.IDToken != "I0828364575432248" {
		t.Errorf("idToken = %q, want I0828364575432248", resp.Inquiry.IDToken)
	}
}

func TestNewKirimDokuLegacyRefundNotificationAck(t *testing.T) {
	req := KirimDokuLegacyRefundNotification{TransactionID: "DK0826666"}
	resp := NewKirimDokuLegacyRefundNotificationAck(req)
	if resp.ResponseCode != "00" || resp.TransactionID != "DK0826666" || !strings.EqualFold(resp.Status, "true") {
		t.Errorf("ack response = %+v", resp)
	}
}
