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

// InquiryResponseData is InquiryResponse's virtualAccountData payload —
// pulled out to a named type so it can be held as a pointer (see
// InquiryResponse below for why that matters).
type InquiryResponseData struct {
	PartnerServiceID      string `json:"partnerServiceId"`
	CustomerNo            string `json:"customerNo"`
	VirtualAccountNo      string `json:"virtualAccountNo"`
	VirtualAccountName    string `json:"virtualAccountName,omitempty"`
	VirtualAccountEmail   string `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone   string `json:"virtualAccountPhone,omitempty"`
	InquiryRequestID      string `json:"inquiryRequestId"`
	TotalAmount           Amount `json:"totalAmount"`
	VirtualAccountTrxType string `json:"virtualAccountTrxType,omitempty"`
	ExpiredDate           string `json:"expiredDate,omitempty"`
	InquiryStatus         string `json:"inquiryStatus"`
	InquiryReason         struct {
		English   string `json:"english"`
		Indonesia string `json:"indonesia"`
	} `json:"inquiryReason"`
	FreeText []FreeText `json:"freeText,omitempty"`
}

// InquiryResponseAdditionalInfo is InquiryResponse's additionalInfo payload.
type InquiryResponseAdditionalInfo struct {
	Channel              string                `json:"channel,omitempty"`
	TrxID                string                `json:"trxId,omitempty"`
	VirtualAccountConfig *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
}

// InquiryResponse is the response body our Inquiry URL handler must return.
// Fields beyond the original capture (VirtualAccountName/VirtualAccountTrxType,
// AdditionalInfo.Channel) added 2026-07-14 after a real DOKU sandbox call kept
// showing "General Error" despite HTTP 200 — confirmed against the fuller
// schema on developers.doku.com's Direct Inquiry section (BRI page).
//
// VirtualAccountData/AdditionalInfo are POINTERS, not embedded structs — a
// struct field's zero value is never considered "empty" by encoding/json, so
// `omitempty` on a plain struct field is a no-op (Go gotcha); a nil pointer
// is the only way to actually omit the whole object. DOKU flagged this
// 2026-08-20: negative-case Inquiry responses (NotFound/BillPaid/BillExpired
// below) must omit virtualAccountData/additionalInfo entirely, not send them
// full of empty strings — confirmed via their own example
// (`{"responseCode": "4042412", "responseMessage": "Bill not found"}`, no
// virtualAccountData at all).
type InquiryResponse struct {
	ResponseCode       string                         `json:"responseCode"`
	ResponseMessage    string                         `json:"responseMessage"`
	VirtualAccountData *InquiryResponseData           `json:"virtualAccountData,omitempty"`
	AdditionalInfo     *InquiryResponseAdditionalInfo `json:"additionalInfo,omitempty"`
}

// NewInquiryResponseSuccess builds a successful ("00") inquiry response
// echoing the VA identity DOKU asked about, plus the amount we expect to be
// paid. responseCode "2002400" confirmed by DOKU support 2026-07-17 (format:
// HTTP code + service code + case code; service code 24 = Inquiry, 25 =
// Payment — "2002500" was wrong, that's Payment's code, not Inquiry's).
// virtualAccountTrxType is always VATrxTypeClosed ("C") — every VA this
// project creates is a fixed-bill (FIX_BILL) VA, never open/variable-amount.
func NewInquiryResponseSuccess(req InquiryRequest, amount Amount, virtualAccountName, trxID string) InquiryResponse {
	data := &InquiryResponseData{
		PartnerServiceID:      req.PartnerServiceID,
		CustomerNo:            req.CustomerNo,
		VirtualAccountNo:      req.VirtualAccountNo,
		VirtualAccountName:    virtualAccountName,
		InquiryRequestID:      req.InquiryRequestID,
		TotalAmount:           amount,
		VirtualAccountTrxType: VATrxTypeClosed,
		InquiryStatus:         "00",
	}
	data.InquiryReason.English = "Success"
	data.InquiryReason.Indonesia = "Sukses"
	return InquiryResponse{
		ResponseCode:       "2002400",
		ResponseMessage:    "Successful",
		VirtualAccountData: data,
		AdditionalInfo: &InquiryResponseAdditionalInfo{
			Channel: req.AdditionalInfo.Channel,
			TrxID:   trxID,
			// DIPC VA numbers are inherently permanent/merchant-owned, unlike
			// a single-use DGPC VA — reflect that as reusableStatus=true,
			// matching the same VirtualAccountConfig struct CreateVA's
			// Static VA option uses.
			VirtualAccountConfig: &VirtualAccountConfig{ReusableStatus: boolPtr(true)},
		},
	}
}

// newInquiryResponseMinimal builds a negative-case Inquiry response matching
// DOKU's exact requested shape (2026-08-20 feedback): responseCode +
// responseMessage only, no virtualAccountData/additionalInfo at all.
func newInquiryResponseMinimal(code, message string) InquiryResponse {
	return InquiryResponse{ResponseCode: code, ResponseMessage: message}
}

// NewInquiryResponseNotFound builds a "not found" ("01") inquiry response —
// DOKU-facing signal that the VA number doesn't correspond to any known bill.
// responseCode "4042412": service code 24 = Inquiry, case code 12 = Not
// Found — confirmed against ASPI Devsite's own sandbox response 2026-08-13.
// Same class of mixup as NewInquiryResponseSuccess above ("4042514" is
// Payment's (25) not-found code, not Inquiry's (24)) — this sibling function
// was missed when that one got fixed.
func NewInquiryResponseNotFound(req InquiryRequest) InquiryResponse {
	return newInquiryResponseMinimal("4042412", "Bill not found")
}

// NewInquiryResponseBillPaid builds a "bill has been paid" inquiry response —
// the VA number is real, but its invoice is already PAID/SUCCEEDED, so it
// can't be paid again. responseCode "4042414" confirmed against ASPI
// Devsite's Tabel Jenis Pengujian (service code 24 = Inquiry).
func NewInquiryResponseBillPaid(req InquiryRequest) InquiryResponse {
	return newInquiryResponseMinimal("4042414", "Bill has been paid")
}

// NewInquiryResponseBillExpired builds a "bill expired" inquiry response —
// the VA number is real, but its invoice's expiry_date has passed.
// responseCode "4042419" confirmed against ASPI Devsite's Tabel Jenis
// Pengujian (service code 24 = Inquiry).
func NewInquiryResponseBillExpired(req InquiryRequest) InquiryResponse {
	return newInquiryResponseMinimal("4042419", "Bill expired")
}
