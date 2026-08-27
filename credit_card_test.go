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

func TestCreditCardClient_CreatePaymentPage_ErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errors":{"code":"INVALID_PARAMETER","message":"Invalid Total Amount And Line Items","type":"Invalid Parameter"}}`))
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
