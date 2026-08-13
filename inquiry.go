package dokugo

// InquiryRequest is what DOKU POSTs to our own Inquiry URL for VA Direct
// Inquiry (DIPC) — DOKU generates no VA numbers itself in this mode; we do,
// so DOKU asks us to validate a VA number and supply the expected amount.
// Schema captured 2026-07-11 from developers.doku.com's per-bank VA pages,
// "Direct Inquiry" section (see docs/DOKU_FUNCTION_REFERENCE.md §3.5).
type InquiryRequest struct {
	PartnerServiceID string `json:"partnerServiceId"`
	CustomerNo       string `json:"customerNo"`
	VirtualAccountNo string `json:"virtualAccountNo"`
	ChannelCode      string `json:"channelCode,omitempty"`
	TrxDateInit      string `json:"trxDateInit,omitempty"`
	Language         string `json:"language,omitempty"`
	InquiryRequestID string `json:"inquiryRequestId"`
	AdditionalInfo   struct {
		Channel string `json:"channel"`
	} `json:"additionalInfo,omitempty"`
}

// InquiryResponse is the response body our Inquiry URL handler must return.
// Fields beyond the original capture (VirtualAccountName/VirtualAccountTrxType,
// AdditionalInfo.Channel) added 2026-07-14 after a real DOKU sandbox call kept
// showing "General Error" despite HTTP 200 — confirmed against the fuller
// schema on developers.doku.com's Direct Inquiry section (BRI page).
type InquiryResponse struct {
	ResponseCode       string `json:"responseCode"`
	ResponseMessage    string `json:"responseMessage"`
	VirtualAccountData struct {
		PartnerServiceID      string `json:"partnerServiceId"`
		CustomerNo            string `json:"customerNo"`
		VirtualAccountNo      string `json:"virtualAccountNo"`
		VirtualAccountName    string `json:"virtualAccountName"`
		VirtualAccountEmail   string `json:"virtualAccountEmail"`
		VirtualAccountPhone   string `json:"virtualAccountPhone"`
		InquiryRequestID      string `json:"inquiryRequestId"`
		TotalAmount           Amount `json:"totalAmount"`
		VirtualAccountTrxType string `json:"virtualAccountTrxType"`
		ExpiredDate           string `json:"expiredDate,omitempty"`
		InquiryStatus         string `json:"inquiryStatus"`
		InquiryReason         struct {
			English   string `json:"english"`
			Indonesia string `json:"indonesia"`
		} `json:"inquiryReason"`
		FreeText []FreeText `json:"freeText"`
	} `json:"virtualAccountData"`
	AdditionalInfo struct {
		Channel              string                `json:"channel,omitempty"`
		TrxID                string                `json:"trxId,omitempty"`
		VirtualAccountConfig *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
	} `json:"additionalInfo,omitempty"`
}

// NewInquiryResponseSuccess builds a successful ("00") inquiry response
// echoing the VA identity DOKU asked about, plus the amount we expect to be
// paid. responseCode "2002400" confirmed by DOKU support 2026-07-17 (format:
// HTTP code + service code + case code; service code 24 = Inquiry, 25 =
// Payment — "2002500" was wrong, that's Payment's code, not Inquiry's).
// virtualAccountTrxType is always VATrxTypeClosed ("C") — every VA this
// project creates is a fixed-bill (FIX_BILL) VA, never open/variable-amount.
func NewInquiryResponseSuccess(req InquiryRequest, amount Amount, virtualAccountName, trxID string) InquiryResponse {
	resp := InquiryResponse{ResponseCode: "2002400", ResponseMessage: "Successful"}
	resp.VirtualAccountData.PartnerServiceID = req.PartnerServiceID
	resp.VirtualAccountData.CustomerNo = req.CustomerNo
	resp.VirtualAccountData.VirtualAccountNo = req.VirtualAccountNo
	resp.VirtualAccountData.VirtualAccountName = virtualAccountName
	resp.VirtualAccountData.InquiryRequestID = req.InquiryRequestID
	resp.VirtualAccountData.TotalAmount = amount
	resp.VirtualAccountData.VirtualAccountTrxType = VATrxTypeClosed
	resp.VirtualAccountData.InquiryStatus = "00"
	resp.VirtualAccountData.InquiryReason.English = "Success"
	resp.VirtualAccountData.InquiryReason.Indonesia = "Sukses"
	resp.VirtualAccountData.FreeText = []FreeText{}
	resp.AdditionalInfo.Channel = req.AdditionalInfo.Channel
	resp.AdditionalInfo.TrxID = trxID
	// DIPC VA numbers are inherently permanent/merchant-owned, unlike a
	// single-use DGPC VA — reflect that as reusableStatus=true, matching the
	// same VirtualAccountConfig struct CreateVA's Static VA option uses.
	resp.AdditionalInfo.VirtualAccountConfig = &VirtualAccountConfig{ReusableStatus: boolPtr(true)}
	return resp
}

// NewInquiryResponseNotFound builds a "not found" ("01") inquiry response —
// DOKU-facing signal that the VA number doesn't correspond to any known bill.
// responseCode "4042412": service code 24 = Inquiry, case code 12 = Not
// Found — confirmed against ASPI Devsite's own sandbox response 2026-08-13.
// Same class of mixup as NewInquiryResponseSuccess above ("4042514" is
// Payment's (25) not-found code, not Inquiry's (24)) — this sibling function
// was missed when that one got fixed.
func NewInquiryResponseNotFound(req InquiryRequest) InquiryResponse {
	resp := InquiryResponse{ResponseCode: "4042412", ResponseMessage: "Invalid Bill/Virtual Account Not Found"}
	resp.VirtualAccountData.PartnerServiceID = req.PartnerServiceID
	resp.VirtualAccountData.CustomerNo = req.CustomerNo
	resp.VirtualAccountData.VirtualAccountNo = req.VirtualAccountNo
	resp.VirtualAccountData.InquiryRequestID = req.InquiryRequestID
	resp.VirtualAccountData.InquiryStatus = "01"
	resp.VirtualAccountData.InquiryReason.English = "Bill not found"
	resp.VirtualAccountData.InquiryReason.Indonesia = "Tagihan tidak ditemukan"
	resp.VirtualAccountData.FreeText = []FreeText{}
	resp.AdditionalInfo.Channel = req.AdditionalInfo.Channel
	return resp
}

// NewInquiryResponseBillPaid builds a "bill has been paid" inquiry response —
// the VA number is real, but its invoice is already PAID/SUCCEEDED, so it
// can't be paid again. responseCode "4042414" confirmed against ASPI
// Devsite's Tabel Jenis Pengujian (service code 24 = Inquiry).
func NewInquiryResponseBillPaid(req InquiryRequest) InquiryResponse {
	resp := InquiryResponse{ResponseCode: "4042414", ResponseMessage: "Bill has been paid"}
	resp.VirtualAccountData.PartnerServiceID = req.PartnerServiceID
	resp.VirtualAccountData.CustomerNo = req.CustomerNo
	resp.VirtualAccountData.VirtualAccountNo = req.VirtualAccountNo
	resp.VirtualAccountData.InquiryRequestID = req.InquiryRequestID
	resp.VirtualAccountData.InquiryStatus = "01"
	resp.VirtualAccountData.InquiryReason.English = "Bill has been paid"
	resp.VirtualAccountData.InquiryReason.Indonesia = "Tagihan sudah dibayar"
	resp.VirtualAccountData.FreeText = []FreeText{}
	resp.AdditionalInfo.Channel = req.AdditionalInfo.Channel
	return resp
}

// NewInquiryResponseBillExpired builds a "bill expired" inquiry response —
// the VA number is real, but its invoice's expiry_date has passed.
// responseCode "4042419" confirmed against ASPI Devsite's Tabel Jenis
// Pengujian (service code 24 = Inquiry).
func NewInquiryResponseBillExpired(req InquiryRequest) InquiryResponse {
	resp := InquiryResponse{ResponseCode: "4042419", ResponseMessage: "Bill expired"}
	resp.VirtualAccountData.PartnerServiceID = req.PartnerServiceID
	resp.VirtualAccountData.CustomerNo = req.CustomerNo
	resp.VirtualAccountData.VirtualAccountNo = req.VirtualAccountNo
	resp.VirtualAccountData.InquiryRequestID = req.InquiryRequestID
	resp.VirtualAccountData.InquiryStatus = "01"
	resp.VirtualAccountData.InquiryReason.English = "Bill expired"
	resp.VirtualAccountData.InquiryReason.Indonesia = "Tagihan kadaluarsa"
	resp.VirtualAccountData.FreeText = []FreeText{}
	resp.AdditionalInfo.Channel = req.AdditionalInfo.Channel
	return resp
}
