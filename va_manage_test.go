package dokugo

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestScenario03_DeleteVA_Success(t *testing.T) {
	var gotReq DeleteVARequest
	gw, _ := mockGateway(t, pathDeleteVA,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				t.Errorf("method = %s, want DELETE", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotReq)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(DeleteVAResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "2003100", ResponseMessage: "Successful"},
				VirtualAccountData: DeleteVAResponseData{
					PartnerServiceID: gotReq.PartnerServiceID,
					CustomerNo:       gotReq.CustomerNo,
					VirtualAccountNo: gotReq.VirtualAccountNo,
					TrxID:            gotReq.TrxID,
				},
			})
		},
	)

	resp, err := gw.DeleteVA(&DeleteVARequest{
		PartnerServiceID: "   19008",
		CustomerNo:       "00000000000000000001",
		VirtualAccountNo: "   190080" + "0000000000000001",
		TrxID:            "TRX-DEL-1",
		AdditionalInfo:   vaAdditionalInfoRef{Channel: BankChannels["bca"]},
	})
	if err != nil {
		t.Fatalf("DeleteVA error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus: %+v", resp)
	}
	if resp.VirtualAccountData.TrxID != "TRX-DEL-1" {
		t.Fatalf("TrxID = %s, want TRX-DEL-1", resp.VirtualAccountData.TrxID)
	}
}

func TestScenario03_DeleteVA_Fail_NotFound(t *testing.T) {
	gw, _ := mockGateway(t, pathDeleteVA,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(DeleteVAResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "4043118", ResponseMessage: "Invalid Virtual Account"},
			})
		},
	)

	resp, err := gw.DeleteVA(&DeleteVARequest{
		PartnerServiceID: "   19008",
		CustomerNo:       "0",
		VirtualAccountNo: "   190080",
		TrxID:            "TRX-DEL-2",
	})
	if err != nil {
		t.Fatalf("transport-level error not expected for a 404 with a body: %v", err)
	}
	if !resp.ErrorStatus {
		t.Fatal("expected ErrorStatus=true for a 404 response")
	}
	if resp.ResponseCode != "4043118" {
		t.Fatalf("ResponseCode = %s, want 4043118", resp.ResponseCode)
	}
}

func TestScenario04_UpdateVA_Success(t *testing.T) {
	var gotReq UpdateVARequest
	gw, _ := mockGateway(t, pathUpdateVA,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPut {
				t.Errorf("method = %s, want PUT", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotReq)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(UpdateVAResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "2002800", ResponseMessage: "Successful"},
				VirtualAccountData: CreateVAResponseData{
					PartnerServiceID:   gotReq.PartnerServiceID,
					CustomerNo:         gotReq.CustomerNo,
					VirtualAccountNo:   gotReq.VirtualAccountNo,
					VirtualAccountName: gotReq.VirtualAccountName,
					TrxID:              gotReq.TrxID,
				},
			})
		},
	)

	newName := "Updated Name"
	resp, err := gw.UpdateVA(&UpdateVARequest{
		PartnerServiceID:   "   19008",
		CustomerNo:         "00000000000000000001",
		VirtualAccountNo:   "   190080" + "0000000000000001",
		VirtualAccountName: newName,
		TrxID:              "TRX-UPD-1",
		AdditionalInfo:     CreateVAAdditionalInfo{Channel: BankChannels["bca"]},
	})
	if err != nil {
		t.Fatalf("UpdateVA error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus: %+v", resp)
	}
	if resp.VirtualAccountData.VirtualAccountName != newName {
		t.Fatalf("VirtualAccountName = %s, want %s", resp.VirtualAccountData.VirtualAccountName, newName)
	}
}

func TestScenario05_CheckStatus_Success(t *testing.T) {
	gw, _ := mockGateway(t, pathCheckStatus,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CheckStatusResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "2002600", ResponseMessage: "Successful"},
				VirtualAccountData: CheckStatusResponseData{
					PaymentFlagReason: PaymentFlagReason{English: "Success", Indonesia: "Sukses"},
					PartnerServiceID:  "   19008",
					CustomerNo:        "00000000000000000001",
					VirtualAccountNo:  "   19008" + "00000000000000000001",
					PaidAmount:        Amount{Value: "150000.00", Currency: "IDR"},
					BillDetails:       []BillDetail{{BillAmount: Amount{Value: "150000.00", Currency: "IDR"}}},
				},
			})
		},
	)

	resp, err := gw.CheckStatus(&CheckStatusRequest{
		PartnerServiceID: "   19008",
		CustomerNo:       "00000000000000000001",
		VirtualAccountNo: "   19008" + "00000000000000000001",
		AdditionalInfo:   map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("CheckStatus error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus: %+v", resp)
	}
	if resp.VirtualAccountData.PaidAmount.Value != "150000.00" {
		t.Fatalf("PaidAmount = %+v, want 150000.00", resp.VirtualAccountData.PaidAmount)
	}
}

func TestScenario05_CheckStatus_Fail_NotPaid(t *testing.T) {
	gw, _ := mockGateway(t, pathCheckStatus,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(CheckStatusResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "4042614", ResponseMessage: "Paid Bill Not Found"},
			})
		},
	)

	resp, err := gw.CheckStatus(&CheckStatusRequest{
		PartnerServiceID: "   19008",
		CustomerNo:       "00000000000000000001",
		VirtualAccountNo: "   19008" + "00000000000000000001",
		AdditionalInfo:   map[string]interface{}{},
	})
	if err != nil {
		t.Fatalf("transport-level error not expected for a 404 with a body: %v", err)
	}
	if !resp.ErrorStatus {
		t.Fatal("expected ErrorStatus=true when the VA hasn't been paid yet")
	}
	if resp.ResponseCode != "4042614" {
		t.Fatalf("ResponseCode = %s, want 4042614", resp.ResponseCode)
	}
}
