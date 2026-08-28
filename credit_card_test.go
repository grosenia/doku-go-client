package dokugo

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignNonSNAP_MatchesHandComputedExample(t *testing.T) {
	// Hand-computed against the exact algorithm in signNonSNAP's doc comment,
	// not against DOKU's own worked example (their example's secret key is
	// undisclosed) — this is a stability/regression guard on our own formula.
	secretKey := "test-secret"
	stringToSign := "Client-Id:MCH-0001-10791114622547\n" +
		"Request-Id:cc682442-6c22-493e-8121-b9ef6b3fa728\n" +
		"Request-Timestamp:2020-08-11T08:45:42Z\n" +
		"Request-Target:/doku-virtual-account/v2/payment-code\n" +
		"Digest:5WIYK2TJg6iiZ0d5v4IXSR0EkYEkYOezJIma3Ufli5s="
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(stringToSign))
	want := "HMACSHA256=" + base64.StdEncoding.EncodeToString(mac.Sum(nil))

	got := signNonSNAP(secretKey, "MCH-0001-10791114622547", "cc682442-6c22-493e-8121-b9ef6b3fa728",
		"2020-08-11T08:45:42Z", "/doku-virtual-account/v2/payment-code",
		"5WIYK2TJg6iiZ0d5v4IXSR0EkYEkYOezJIma3Ufli5s=")

	if got != want {
		t.Fatalf("signNonSNAP() = %q, want %q", got, want)
	}
}

func TestSignNonSNAP_GETOmitsDigestLine(t *testing.T) {
	// GET requests (e.g. Check Status) omit Digest entirely per DOKU's docs
	// — passing digest="" must produce a different signature than passing
	// digest="" was somehow still appended.
	withDigest := signNonSNAP("secret", "MCH-1", "req-1", "2020-01-01T00:00:00Z", "/some/path", "abc=")
	withoutDigest := signNonSNAP("secret", "MCH-1", "req-1", "2020-01-01T00:00:00Z", "/some/path", "")
	if withDigest == withoutDigest {
		t.Fatal("signature should differ when Digest is present vs omitted")
	}
}

func TestCreditCardClient_CreatePaymentPage_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != pathCreatePaymentPage {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		for _, h := range []string{"Client-Id", "Request-Id", "Request-Timestamp", "Signature", "Digest"} {
			if r.Header.Get(h) == "" {
				t.Fatalf("missing header %s", h)
			}
		}
		if r.Header.Get("Client-Id") != "MCH-0001-test" {
			t.Fatalf("Client-Id = %q", r.Header.Get("Client-Id"))
		}

		var req CreatePaymentPageRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Order.InvoiceNumber != "INV-TEST-0001" {
			t.Fatalf("invoice_number = %q", req.Order.InvoiceNumber)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(CreatePaymentPageResponse{
			CreditCardPaymentPage: creditCardPaymentPage{URL: "https://sandbox.doku.com/wt-frontend-transaction/dynamic-payment-page?x=y"},
		})
	}))
	defer server.Close()

	client := NewCreditCardClient("MCH-0001-test", "test-secret", true)
	client.BaseURL = server.URL

	resp, err := client.CreatePaymentPage(&CreatePaymentPageRequest{
		Order: PaymentPageOrder{
			InvoiceNumber: "INV-TEST-0001",
			Amount:        90000,
			AutoRedirect:  false,
		},
		Customer: PaymentPageCustomer{Email: "buyer@example.com"},
		Payment:  PaymentPagePayment{Type: PaymentTypeSale},
	})
	if err != nil {
		t.Fatalf("CreatePaymentPage error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus, response: %+v", resp)
	}
	if resp.CreditCardPaymentPage.URL == "" {
		t.Fatal("expected a non-empty payment page URL")
	}
}

func TestVerifyCardNotificationSignature_RealCapturedNotification(t *testing.T) {
	// Verbatim data captured 2026-08-27 from a real DOKU-signed sandbox
	// notification (headers + body), confirming VerifyCardNotificationSignature
	// actually verifies real DOKU signatures, not just our own signNonSNAP output.
	rawBody := []byte(`{"order":{"invoice_number":"FUNCTEST-CARD-1787826055","amount":90000},"customer":{"name":"tujuh","email":"buyer@example.com"},"transaction":{"type":"AUTHORIZE","status":"PENDING","date":"2026-08-27T10:23:47Z","original_request_id":"d074028e2246fd758e4172bb1b59031c"},"service":{"id":"CREDIT_CARD"},"acquirer":{"id":"BANK_MANDIRI"},"channel":{"id":"CREDIT_CARD","name":"Credit Card"},"additional_info":{"override_notification_url":"https://intra-api.grosenia.co.id/v1/payments/doku/card/notification-debug"},"card_payment":{"masked_card_number":"557338******1101","approval_code":"081981","response_code":"00","response_message":"PAYMENT APPROVED","issuer":"PT BANK MANDIRI (PERSERO) Tbk","identifier":[{"name":"MID","value":"323052789944165"},{"name":"Acquirer","value":"BANK_MANDIRI"},{"name":"TID","value":"11783983"}],"authorize_id":"17878262273617612","brand":"MASTER","authentication_id":"640657752b1bd9b6898453bfd71723d0fa4fd19bda7788760a47fbbdada88fdd","three_d_secure_status":"TRUE"},"verification":{"status":"APPROVE","reason":"Decision No Rules Triggered"}}`)

	ok := VerifyCardNotificationSignature(
		"SK-QbTL192wDWAGYLydPZsE",
		"BRN-0268-1783481131809",
		"CREDIT_CARD4605139620740696042707905868166483795498650276372334601842093866",
		"2026-08-27T10:24:12Z",
		"/v1/payments/doku/card/notification-debug", // the FULL external path DOKU actually called, matching override_notification_url
		rawBody,
		"HMACSHA256=+3NsEDZwtxVDlVKkYUNBsSZB41fUeftS/2aKjinNo54=",
	)
	if !ok {
		t.Fatal("expected the real captured notification signature to verify")
	}

	// A tampered body must fail verification.
	tampered := append([]byte(nil), rawBody...)
	tampered[10] = 'X'
	if VerifyCardNotificationSignature("SK-QbTL192wDWAGYLydPZsE", "BRN-0268-1783481131809",
		"CREDIT_CARD4605139620740696042707905868166483795498650276372334601842093866",
		"2026-08-27T10:24:12Z", "/v1/payments/doku/card/notification-debug", tampered,
		"HMACSHA256=+3NsEDZwtxVDlVKkYUNBsSZB41fUeftS/2aKjinNo54=") {
		t.Fatal("expected a tampered body to fail verification")
	}
}

func TestCardNotification_UnmarshalsRealCapturedPayload(t *testing.T) {
	// Verbatim payload captured 2026-08-27 from a real sandbox AUTHORIZE
	// transaction's notification (delivered via
	// additional_info.override_notification_url).
	raw := []byte(`{"order":{"invoice_number":"FUNCTEST-CARD-1787826055","amount":90000},"customer":{"name":"tujuh","email":"buyer@example.com"},"transaction":{"type":"AUTHORIZE","status":"PENDING","date":"2026-08-27T10:23:47Z","original_request_id":"d074028e2246fd758e4172bb1b59031c"},"service":{"id":"CREDIT_CARD"},"acquirer":{"id":"BANK_MANDIRI"},"channel":{"id":"CREDIT_CARD","name":"Credit Card"},"additional_info":{"override_notification_url":"https://intra-api.grosenia.co.id/v1/payments/doku/card/notification-debug"},"card_payment":{"masked_card_number":"557338******1101","approval_code":"081981","response_code":"00","response_message":"PAYMENT APPROVED","issuer":"PT BANK MANDIRI (PERSERO) Tbk","identifier":[{"name":"MID","value":"323052789944165"},{"name":"Acquirer","value":"BANK_MANDIRI"},{"name":"TID","value":"11783983"}],"authorize_id":"17878262273617612","brand":"MASTER","authentication_id":"640657752b1bd9b6898453bfd71723d0fa4fd19bda7788760a47fbbdada88fdd","three_d_secure_status":"TRUE"},"verification":{"status":"APPROVE","reason":"Decision No Rules Triggered"}}`)

	var n CardNotification
	if err := json.Unmarshal(raw, &n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if n.Order.InvoiceNumber != "FUNCTEST-CARD-1787826055" {
		t.Errorf("Order.InvoiceNumber = %q", n.Order.InvoiceNumber)
	}
	if n.Transaction.Type != "AUTHORIZE" {
		t.Errorf("Transaction.Type = %q", n.Transaction.Type)
	}
	if n.CardPayment.AuthorizeID != "17878262273617612" {
		t.Errorf("CardPayment.AuthorizeID = %q, want 17878262273617612", n.CardPayment.AuthorizeID)
	}
	if n.CardPayment.ResponseCode != "00" {
		t.Errorf("CardPayment.ResponseCode = %q", n.CardPayment.ResponseCode)
	}
	if len(n.CardPayment.Identifier) != 3 {
		t.Fatalf("len(Identifier) = %d, want 3", len(n.CardPayment.Identifier))
	}
	if n.CardPayment.Identifier[2].Name != "TID" || n.CardPayment.Identifier[2].Value != "11783983" {
		t.Errorf("Identifier[2] = %+v", n.CardPayment.Identifier[2])
	}
	if n.Verification.Status != "APPROVE" {
		t.Errorf("Verification.Status = %q", n.Verification.Status)
	}
}

func TestCreditCardClient_CreatePaymentPage_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"code":"INVALID_PARAMETER","message":"Invalid Total Amount And Line Items","type":"Invalid Parameter"}}`))
	}))
	defer server.Close()

	client := NewCreditCardClient("MCH-0001-test", "test-secret", true)
	client.BaseURL = server.URL

	resp, err := client.CreatePaymentPage(&CreatePaymentPageRequest{
		Order: PaymentPageOrder{InvoiceNumber: "INV-TEST-0002", Amount: 1000},
	})
	if err != nil {
		t.Fatalf("CreatePaymentPage transport error: %v", err)
	}
	if !resp.ErrorStatus {
		t.Fatal("expected ErrorStatus=true for a 400 response")
	}
	if resp.Errors.Code != "INVALID_PARAMETER" {
		t.Fatalf("Errors.Code = %q", resp.Errors.Code)
	}
}
