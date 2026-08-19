//go:build integration

package dokugo

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// integrationClient builds a Client from real sandbox credentials, skipping
// the test entirely if they aren't configured — so `go test ./...` (without
// -tags=integration) never touches the network, and CI without secrets
// doesn't fail.
func integrationClient(t *testing.T) *Gateway {
	t.Helper()
	clientID := os.Getenv("DOKU_CLIENT_ID")
	secretKey := os.Getenv("DOKU_SECRET_KEY")
	keyPath := os.Getenv("DOKU_PRIVATE_KEY_PATH")
	if clientID == "" || secretKey == "" || keyPath == "" {
		t.Skip("DOKU_CLIENT_ID / DOKU_SECRET_KEY / DOKU_PRIVATE_KEY_PATH not set, skipping integration test")
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("read private key: %v", err)
	}
	client, err := NewClient(clientID, secretKey, keyPEM, true)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return NewGateway(&client)
}

func TestIntegration_CreateStaticVA_CheckStatus_DeleteVA_RoundTrip(t *testing.T) {
	gw := integrationClient(t)
	partnerServiceID := envOr("DOKU_PARTNER_SERVICE_ID", "   19008")
	bank := envOr("DOKU_BANK", "bca")
	// DGPC: customerNo is just the merchant-assigned prefix digit (DOKU
	// generates the rest) — NOT a self-built long number, see CLAUDE.md.
	customerNo := envOr("DOKU_CUSTOMER_NO_PREFIX", "9")
	trxID := fmt.Sprintf("IT-%d", time.Now().UnixNano())

	createResp, err := gw.CreateVA(NewStaticVARequest(bank, partnerServiceID, customerNo, "Integration Test", trxID, Amount{Value: "10000.00", Currency: "IDR"}))
	if err != nil {
		t.Fatalf("CreateVA: %v", err)
	}
	if createResp.ErrorStatus {
		t.Fatalf("CreateVA failed: %s %s", createResp.ResponseCode, createResp.ResponseMessage)
	}
	// Confirmed against real sandbox 2026-07-11: for follow-up calls
	// (CheckStatus/DeleteVA/UpdateVA), virtualAccountNo must be the literal
	// partnerServiceId+customerNo WE sent — NOT the DOKU-generated 16-digit
	// number in createResp.VirtualAccountData.VirtualAccountNo (that's only
	// for display/payment purposes). Using the generated one here fails with
	// "4033115 Transaction Not Permitted [virtualAccountNo should be equal
	// to partnerServiceId + customerNo]".
	requestVirtualAccountNo := partnerServiceID + customerNo

	statusResp, err := gw.CheckStatus(&CheckStatusRequest{
		PartnerServiceID: partnerServiceID,
		CustomerNo:       customerNo,
		VirtualAccountNo: requestVirtualAccountNo,
		AdditionalInfo:   map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	// An unpaid VA is expected to report an error/not-found status here —
	// this call is exercised for its own sake (confirms auth/signing works
	// against the real sandbox), not to assert a specific payment state.
	t.Logf("CheckStatus response: %+v", statusResp)

	// DeleteVA (and UpdateVA) need a DIFFERENT CustomerNo/VirtualAccountNo
	// convention than CheckStatus — confirmed against real sandbox 2026-08-19,
	// see CLAUDE.md: use the DOKU-GENERATED CustomerNo/VirtualAccountNo from
	// createResp (not the short customerNo/requestVirtualAccountNo above,
	// those are CheckStatus-only), AND DeleteVA's TrxID must match the
	// ORIGINAL CreateVA call's TrxID exactly (a fresh TrxID fails with
	// "4043119 Invalid Bill/Virtual Account" even with the right
	// CustomerNo/VirtualAccountNo/Channel).
	generatedCustomerNo := createResp.VirtualAccountData.CustomerNo
	generatedVirtualAccountNo := createResp.VirtualAccountData.VirtualAccountNo

	deleteResp, err := gw.DeleteVA(&DeleteVARequest{
		PartnerServiceID: partnerServiceID,
		CustomerNo:       generatedCustomerNo,
		VirtualAccountNo: generatedVirtualAccountNo,
		TrxID:            trxID,
		AdditionalInfo:   vaAdditionalInfoRef{Channel: BankChannels[bank]},
	})
	if err != nil {
		t.Fatalf("DeleteVA: %v", err)
	}
	if deleteResp.ErrorStatus {
		t.Fatalf("DeleteVA failed: %s %s", deleteResp.ResponseCode, deleteResp.ResponseMessage)
	}
	t.Logf("DeleteVA response: %s %s", deleteResp.ResponseCode, deleteResp.ResponseMessage)
}

func TestIntegrationFail_CreateVA_InvalidPartnerServiceID(t *testing.T) {
	gw := integrationClient(t)
	resp, err := gw.CreateVA(NewSingleUseVARequest("bca", "INVALID!", "1", "Integration Test", "IT-FAIL", Amount{Value: "10000.00", Currency: "IDR"}))
	if err != nil {
		t.Fatalf("transport-level error not expected: %v", err)
	}
	if !resp.ErrorStatus {
		t.Fatal("expected DOKU to reject an invalid partnerServiceId")
	}
	t.Logf("expected failure response: %s %s", resp.ResponseCode, resp.ResponseMessage)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
