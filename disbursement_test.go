package dokugo

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestScenario08_AccountInquiry_Success(t *testing.T) {
	var gotReq AccountInquiryRequest
	gw, _ := mockGateway(t, pathAccountInquiry,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &gotReq); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(AccountInquiryResponse{
				ErrorResponse:            ErrorResponse{ResponseCode: "2001700", ResponseMessage: "Successful"},
				BeneficiaryAccountNumber: gotReq.BeneficiaryAccountNumber,
				BeneficiaryAccountName:   "John Doe",
				BeneficiaryBankCode:      gotReq.AdditionalInfo.BeneficiaryBankCode,
				Amount:                   gotReq.Amount,
				SessionID:                "session-123",
			})
		},
	)

	resp, err := gw.AccountInquiry(&AccountInquiryRequest{
		PartnerReferenceNo:       "REF-001",
		CustomerNumber:           "0812345678",
		BeneficiaryAccountNumber: "1234567890",
		Amount:                   Amount{Value: "100000.00", Currency: "IDR"},
		AdditionalInfo:           AccountInquiryAdditionalInfo{BeneficiaryBankCode: "014"},
	})
	if err != nil {
		t.Fatalf("AccountInquiry error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus, response: %+v", resp)
	}
	if resp.SessionID != "session-123" {
		t.Fatalf("SessionID = %q, want session-123", resp.SessionID)
	}
	if resp.BeneficiaryAccountNumber != "1234567890" {
		t.Fatalf("BeneficiaryAccountNumber = %q, want 1234567890", resp.BeneficiaryAccountNumber)
	}
}

func TestScenario09_TransferBank_Success(t *testing.T) {
	var gotReq TransferBankRequest
	gw, _ := mockGateway(t, pathTransferBank,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &gotReq); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(TransferBankResponse{
				ErrorResponse:      ErrorResponse{ResponseCode: "2002500", ResponseMessage: "Successful"},
				ReferenceNo:        "DOKU-REF-001",
				PartnerReferenceNo: gotReq.PartnerReferenceNo,
				TransactionDate:    "2026-07-08T10:00:00+07:00",
			})
		},
	)

	resp, err := gw.TransferBank(&TransferBankRequest{
		PartnerReferenceNo:       "PAYOUT-001",
		CustomerNumber:           "0812345678",
		BeneficiaryAccountNumber: "1234567890",
		BeneficiaryBankCode:      "014",
		Amount:                   Amount{Value: "100000.00", Currency: "IDR"},
		SessionID:                "session-123",
		AdditionalInfo: TransferBankAdditionalInfo{
			BeneficiaryFirstName:   "John",
			BeneficiaryLastName:    "Doe",
			BeneficiaryPhoneNumber: "0812345678",
			BeneficiaryAccountName: "John Doe",
			SenderCountryCode:      "ID",
			SenderFirstName:        "Grosenia",
			SenderLastName:         "Niaga",
			SenderPersonalID:       "1234567890123456",
			SenderPersonalIDType:   "KTP",
		},
	})
	if err != nil {
		t.Fatalf("TransferBank error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus, response: %+v", resp)
	}
	if resp.PartnerReferenceNo != "PAYOUT-001" {
		t.Fatalf("PartnerReferenceNo = %q, want PAYOUT-001", resp.PartnerReferenceNo)
	}
	if gotReq.AdditionalInfo.SenderPersonalIDType != "KTP" {
		t.Fatalf("SenderPersonalIDType not sent correctly on the wire")
	}
}

func TestScenario10_TransferBank_Fail_DokuError(t *testing.T) {
	gw, _ := mockGateway(t, pathTransferBank,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(TransferBankResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "4032701", ResponseMessage: "Insufficient Balance"},
			})
		},
	)

	resp, err := gw.TransferBank(&TransferBankRequest{PartnerReferenceNo: "PAYOUT-002", SessionID: "session-x"})
	if err != nil {
		t.Fatalf("transport-level error not expected for an HTTP 400 with a body: %v", err)
	}
	if !resp.ErrorStatus {
		t.Fatal("expected ErrorStatus=true for a 400 response")
	}
	if resp.ResponseCode != "4032701" {
		t.Fatalf("ResponseCode = %s, want 4032701", resp.ResponseCode)
	}
}

func TestScenario11_BalanceInquiry_Success(t *testing.T) {
	gw, _ := mockGateway(t, pathBalanceInquiry,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(BalanceInquiryResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "2001100", ResponseMessage: "Successful"},
				AccountNo:     "A47438",
				Name:          "Grosenia",
				AccountInfos: []BalanceInquiryResponseAccountInfos{
					{AvailableBalance: Amount{Value: "5000000.00", Currency: "IDR"}},
				},
			})
		},
	)

	// AccountNo is the KirimDOKU agentKey, not a bank account number —
	// confirmed against real sandbox 2026-08-20 (see doc comment on
	// BalanceInquiryRequest).
	resp, err := gw.BalanceInquiry(&BalanceInquiryRequest{AccountNo: "A47438"})
	if err != nil {
		t.Fatalf("BalanceInquiry error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus, response: %+v", resp)
	}
	if len(resp.AccountInfos) != 1 || resp.AccountInfos[0].AvailableBalance.Value != "5000000.00" {
		t.Fatalf("AccountInfos = %+v, want one entry with AvailableBalance.Value 5000000.00", resp.AccountInfos)
	}
}

// TestScenario12_CheckDisbursementStatus_Success uses the exact field shape
// captured from a real response against ASPI Devsite's sandbox 2026-08-14
// (see pathDisbursementCheckStatus in urls.go) — regression guard against
// this endpoint's request/response shape drifting from what's confirmed
// real, not just DOKU's buggy docs.
func TestScenario12_CheckDisbursementStatus_Success(t *testing.T) {
	var gotReq DisbursementCheckStatusRequest
	gw, _ := mockGateway(t, pathDisbursementCheckStatus,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &gotReq); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(DisbursementCheckStatusResponse{
				ErrorResponse:              ErrorResponse{ResponseCode: "2003600", ResponseMessage: "Successful"},
				OriginalReferenceNo:        gotReq.OriginalReferenceNo,
				OriginalPartnerReferenceNo: gotReq.OriginalPartnerReferenceNo,
				OriginalExternalID:         gotReq.OriginalExternalID,
				ServiceCode:                gotReq.ServiceCode,
				TransactionDate:            "2026-08-14T11:19:36+07:00",
				Amount:                     Amount{Value: "100000", Currency: "IDR"},
				BeneficiaryAccountNo:       "888801000157508",
				BeneficiaryBankCode:        "8888",
				SourceAccountNo:            "888801000157508",
				PreviousResponseCode:       "2005300",
				ReferenceNumber:            gotReq.OriginalReferenceNo,
				TransactionID:              "5620574226073548",
				LatestTransactionStatus:    DisbursementStatusSuccess,
				TransactionStatusDesc:      "success",
			})
		},
	)

	resp, err := gw.CheckDisbursementStatus(&DisbursementCheckStatusRequest{
		OriginalPartnerReferenceNo: "PAYOUT-001",
		OriginalReferenceNo:        "DOKU-REF-001",
		ServiceCode:                DisbursementCheckStatusServiceCode,
		TransactionDate:            "2026-08-14T11:15:54+07:00",
		Amount:                     Amount{Value: "100000", Currency: "IDR"},
	})
	if err != nil {
		t.Fatalf("CheckDisbursementStatus error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus, response: %+v", resp)
	}
	if resp.LatestTransactionStatus != DisbursementStatusSuccess {
		t.Fatalf("LatestTransactionStatus = %q, want %q", resp.LatestTransactionStatus, DisbursementStatusSuccess)
	}
	if resp.TransactionStatusDesc != "success" {
		t.Fatalf("TransactionStatusDesc = %q, want success", resp.TransactionStatusDesc)
	}
	if gotReq.ServiceCode != DisbursementCheckStatusServiceCode {
		t.Fatalf("ServiceCode not sent correctly on the wire, got %q", gotReq.ServiceCode)
	}
}
