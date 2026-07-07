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
	customerNo := fmt.Sprintf("%020d", time.Now().UnixNano()%1e18)

	createResp, err := gw.CreateVA(NewStaticVARequest(bank, partnerServiceID, customerNo, "Integration Test", "IT-"+customerNo, Amount{Value: "10000.00", Currency: "IDR"}))
	if err != nil {
		t.Fatalf("CreateVA: %v", err)
	}
	if createResp.ErrorStatus {
		t.Fatalf("CreateVA failed: %s %s", createResp.ResponseCode, createResp.ResponseMessage)
	}

	statusResp, err := gw.CheckStatus(&CheckStatusRequest{
		PartnerServiceID: partnerServiceID,
		CustomerNo:       customerNo,
		VirtualAccountNo: createResp.VirtualAccountData.VirtualAccountNo,
		AdditionalInfo:   map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("CheckStatus: %v", err)
	}
	// An unpaid VA is expected to report an error/not-found status here —
	// this call is exercised for its own sake (confirms auth/signing works
	// against the real sandbox), not to assert a specific payment state.
	t.Logf("CheckStatus response: %+v", statusResp)

	deleteResp, err := gw.DeleteVA(&DeleteVARequest{
		PartnerServiceID: partnerServiceID,
		CustomerNo:       customerNo,
		VirtualAccountNo: createResp.VirtualAccountData.VirtualAccountNo,
		TrxID:            "IT-DEL-" + customerNo,
		AdditionalInfo:   vaAdditionalInfoRef{Channel: BankChannels[bank]},
	})
	if err != nil {
		t.Fatalf("DeleteVA: %v", err)
	}
	if deleteResp.ErrorStatus {
		t.Fatalf("DeleteVA failed: %s %s", deleteResp.ResponseCode, deleteResp.ResponseMessage)
	}
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
