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

		// Exact shape captured from a real staging response 2026-08-14 —
		// fund/beneficiaryAccount nest under "inquiry", not top-level.
		_, _ = w.Write([]byte(`{"status":0,"message":"Transfer Inquiry Approve","inquiry":{"idToken":"I0828364575432248","fund":{"origin":{"amount":50000.0,"currency":"IDR"},"fees":{"total":4500.0,"currency":"IDR","components":[{"description":"Default Fee","amount":4500.0}],"additionalFee":0.0,"fixedFee":0.0},"destination":{"amount":50000.0,"currency":"IDR"}},"beneficiaryAccount":{"number":"0803944123","name":"FHILEA HERMANUS","bank":{"id":"014","code":"CENAIDJA","name":"Bank Central Asia BCA","city":"Jakarta","countryCode":"ID"}}}}`))
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
	if resp.Inquiry.Fund.Fees.Total != 4500 {
		t.Errorf("fund.fees.total = %v, want 4500", resp.Inquiry.Fund.Fees.Total)
	}
	if resp.Inquiry.BeneficiaryAccount.Name != "FHILEA HERMANUS" {
		t.Errorf("beneficiaryAccount.name = %q, want FHILEA HERMANUS", resp.Inquiry.BeneficiaryAccount.Name)
	}
}

func TestNewKirimDokuLegacyRefundNotificationAck(t *testing.T) {
	req := KirimDokuLegacyRefundNotification{TransactionID: "DK0826666"}
	resp := NewKirimDokuLegacyRefundNotificationAck(req)
	if resp.ResponseCode != "00" || resp.TransactionID != "DK0826666" || !strings.EqualFold(resp.Status, "true") {
		t.Errorf("ack response = %+v", resp)
	}
}

// TestKirimDokuLegacyCheckBalance_BodyAuth uses the exact response shape
// captured from a real staging call 2026-08-14 — balance fields are numeric
// on the wire, not strings, which this test guards against regressing.
func TestKirimDokuLegacyCheckBalance_BodyAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/checkbalance" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		var captured authFields
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		plaintext := decryptTestSignature(t, captured.Signature, testKirimDokuLegacyEncryptionKey)
		if plaintext != captured.AgentKey+captured.RequestID {
			t.Errorf("signature mismatch: got plaintext %q", plaintext)
		}
		_, _ = w.Write([]byte(`{"status":0,"message":"Check balance success","balance":{"corporateName":"Grosenia","creditLimit":0.000000,"creditAlertLimit":0.000000,"creditLastBalance":1000000000000.000000}}`))
	}))
	defer srv.Close()

	c := NewKirimDokuLegacyClient("A47438", testKirimDokuLegacyEncryptionKey, true)
	c.BaseURL = srv.URL

	resp, err := c.CheckBalance()
	if err != nil {
		t.Fatalf("CheckBalance: %v", err)
	}
	if resp.Balance.CorporateName != "Grosenia" || resp.Balance.CreditLastBalance != 1_000_000_000_000 {
		t.Errorf("CheckBalance response = %+v", resp.Balance)
	}
}
