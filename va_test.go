package dokugo

import (
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"testing"
)

func TestScenario01_CreateVA_Static_Success(t *testing.T) {
	var gotReq CreateVARequest
	gw, _ := mockGateway(t, pathCreateVA,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("method = %s, want POST", r.Method)
			}
			body, _ := io.ReadAll(r.Body)
			if err := json.Unmarshal(body, &gotReq); err != nil {
				t.Fatalf("unmarshal request: %v", err)
			}
			// Auth headers required on every transactional call.
			for _, h := range []string{"X-TIMESTAMP", "X-SIGNATURE", "X-PARTNER-ID", "X-EXTERNAL-ID", "CHANNEL-ID", "Authorization"} {
				if r.Header.Get(h) == "" {
					t.Errorf("missing required header %s", h)
				}
			}
			if r.Header.Get("CHANNEL-ID") != ChannelIDHostToHost {
				t.Errorf("CHANNEL-ID = %s, want %s", r.Header.Get("CHANNEL-ID"), ChannelIDHostToHost)
			}
			if r.Header.Get("Authorization") != "Bearer mock-token" {
				t.Errorf("Authorization = %s, want Bearer mock-token", r.Header.Get("Authorization"))
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CreateVAResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "2002700", ResponseMessage: "Successful"},
				VirtualAccountData: CreateVAResponseData{
					PartnerServiceID:   gotReq.PartnerServiceID,
					CustomerNo:         gotReq.CustomerNo,
					VirtualAccountNo:   gotReq.VirtualAccountNo,
					VirtualAccountName: gotReq.VirtualAccountName,
					TrxID:              gotReq.TrxID,
					TotalAmount:        gotReq.TotalAmount,
				},
			})
		},
	)

	resp, err := gw.CreateVA(NewStaticVARequest("bca", "   19008", "00000000000000000001", "Customer Name", "TRX-001", Amount{Value: "11500.00", Currency: "IDR"}))
	if err != nil {
		t.Fatalf("CreateVA error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus, response: %+v", resp)
	}
	if resp.VirtualAccountData.VirtualAccountNo != gotReq.VirtualAccountNo {
		t.Fatalf("VirtualAccountNo = %q, want %q", resp.VirtualAccountData.VirtualAccountNo, gotReq.VirtualAccountNo)
	}

	// Assert the request that was actually sent had reusableStatus = true.
	if gotReq.AdditionalInfo.VirtualAccountConfig == nil || gotReq.AdditionalInfo.VirtualAccountConfig.ReusableStatus == nil || !*gotReq.AdditionalInfo.VirtualAccountConfig.ReusableStatus {
		t.Fatalf("expected reusableStatus=true on the wire for a static VA request, got %+v", gotReq.AdditionalInfo.VirtualAccountConfig)
	}
	if gotReq.AdditionalInfo.Channel != "VIRTUAL_ACCOUNT_BCA" {
		t.Fatalf("channel = %s, want VIRTUAL_ACCOUNT_BCA", gotReq.AdditionalInfo.Channel)
	}
}

func TestScenario02_CreateVA_SingleUse_Success(t *testing.T) {
	var gotReq CreateVARequest
	gw, _ := mockGateway(t, pathCreateVA,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotReq)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CreateVAResponse{
				ErrorResponse:      ErrorResponse{ResponseCode: "2002700", ResponseMessage: "Successful"},
				VirtualAccountData: CreateVAResponseData{VirtualAccountNo: gotReq.VirtualAccountNo},
			})
		},
	)

	resp, err := gw.CreateVA(NewSingleUseVARequest("bca", "   19008", "00000000000000000002", "Customer Name", "TRX-002", Amount{Value: "50000.00", Currency: "IDR"}))
	if err != nil {
		t.Fatalf("CreateVA error: %v", err)
	}
	if resp.ErrorStatus {
		t.Fatalf("unexpected ErrorStatus")
	}

	// The whole point of "single-use": reusableStatus must be omitted from
	// the wire request, not sent as false — DOKU's default-to-false behavior
	// only kicks in when the field is absent entirely.
	if gotReq.AdditionalInfo.VirtualAccountConfig != nil {
		t.Fatalf("expected virtualAccountConfig to be omitted entirely for single-use VA, got %+v", gotReq.AdditionalInfo.VirtualAccountConfig)
	}
}

func TestScenario01_CreateVA_Fail_DokuError(t *testing.T) {
	gw, _ := mockGateway(t, pathCreateVA,
		mockTokenHandler("mock-token", 900),
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(CreateVAResponse{
				ErrorResponse: ErrorResponse{ResponseCode: "4002701", ResponseMessage: "Invalid Field Format"},
			})
		},
	)

	resp, err := gw.CreateVA(NewStaticVARequest("bca", "   19008", "1", "Customer Name", "TRX-003", Amount{Value: "1.00", Currency: "IDR"}))
	if err != nil {
		t.Fatalf("transport-level error not expected for an HTTP 400 with a body: %v", err)
	}
	if !resp.ErrorStatus {
		t.Fatal("expected ErrorStatus=true for a 400 response")
	}
	if resp.ResponseCode != "4002701" {
		t.Fatalf("ResponseCode = %s, want 4002701", resp.ResponseCode)
	}
}

func TestScenario07_TokenCaching_ReusesWithinExpiry(t *testing.T) {
	var tokenHits int32
	gw, _ := mockGateway(t, pathCreateVA,
		func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&tokenHits, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(accessTokenResponse{
				ResponseCode: "2007300", ResponseMessage: "Successful",
				AccessToken: "cached-token", TokenType: "Bearer", ExpiresIn: 900,
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CreateVAResponse{ErrorResponse: ErrorResponse{ResponseCode: "2002700", ResponseMessage: "Successful"}})
		},
	)

	for i := 0; i < 3; i++ {
		if _, err := gw.CreateVA(NewSingleUseVARequest("bca", "   19008", "1", "N", "T", Amount{Value: "1.00", Currency: "IDR"})); err != nil {
			t.Fatalf("CreateVA call %d: %v", i, err)
		}
	}

	if hits := atomic.LoadInt32(&tokenHits); hits != 1 {
		t.Fatalf("token endpoint hit %d times across 3 gateway calls within expiry, want 1", hits)
	}
}

func TestScenario07_TokenCaching_RefreshesAfterExpiry(t *testing.T) {
	var tokenHits int32
	gw, _ := mockGateway(t, pathCreateVA,
		func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&tokenHits, 1)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// expiresIn shorter than tokenSafetyMargin forces immediate
			// re-fetch on the very next call, without needing to sleep in
			// the test for a realistic 900s window.
			_ = json.NewEncoder(w).Encode(accessTokenResponse{
				ResponseCode: "2007300", ResponseMessage: "Successful",
				AccessToken: "short-lived-token", TokenType: "Bearer", ExpiresIn: 1,
			})
		},
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(CreateVAResponse{ErrorResponse: ErrorResponse{ResponseCode: "2002700", ResponseMessage: "Successful"}})
		},
	)

	if _, err := gw.CreateVA(NewSingleUseVARequest("bca", "   19008", "1", "N", "T", Amount{Value: "1.00", Currency: "IDR"})); err != nil {
		t.Fatalf("first CreateVA: %v", err)
	}
	if _, err := gw.CreateVA(NewSingleUseVARequest("bca", "   19008", "1", "N", "T", Amount{Value: "1.00", Currency: "IDR"})); err != nil {
		t.Fatalf("second CreateVA: %v", err)
	}

	if hits := atomic.LoadInt32(&tokenHits); hits != 2 {
		t.Fatalf("token endpoint hit %d times across 2 calls with an already-expired cache, want 2", hits)
	}
}
