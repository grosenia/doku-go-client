package dokugo

import "strings"

// Amount is DOKU's wire format for monetary values: a decimal string (not a
// float — DOKU's own examples always show 2 decimal places, e.g. "11500.00"),
// plus an ISO 4217 currency code.
type Amount struct {
	Value    string `json:"value"`
	Currency string `json:"currency"`
}

// VirtualAccountConfig carries the Static-vs-Non-static distinction.
// ReusableStatus true = Static VA (payable more than once). Omitted or false =
// single-use (DOKU's own default when the field isn't sent).
type VirtualAccountConfig struct {
	ReusableStatus *bool  `json:"reusableStatus,omitempty"`
	MinAmount      string `json:"minAmount,omitempty"`
	MaxAmount      string `json:"maxAmount,omitempty"`
}

// FreeText is an optional bilingual note shown to the payer.
type FreeText struct {
	English   string `json:"english"`
	Indonesia string `json:"indonesia"`
}

type CreateVAAdditionalInfo struct {
	Channel              string                `json:"channel"`
	VirtualAccountConfig *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
}

// CreateVARequest covers BOTH DGPC (DOKU generates the VA number suffix) and
// MGPC (merchant supplies it) — DOKU uses the identical request/response
// shape for both; the only difference is who populates CustomerNo/VirtualAccountNo.
type CreateVARequest struct {
	PartnerServiceID      string                 `json:"partnerServiceId"`
	CustomerNo            string                 `json:"customerNo"`
	VirtualAccountNo      string                 `json:"virtualAccountNo"`
	VirtualAccountName    string                 `json:"virtualAccountName"`
	VirtualAccountEmail   string                 `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone   string                 `json:"virtualAccountPhone,omitempty"`
	TrxID                 string                 `json:"trxId"`
	TotalAmount           Amount                 `json:"totalAmount"`
	AdditionalInfo        CreateVAAdditionalInfo `json:"additionalInfo"`
	VirtualAccountTrxType string                 `json:"virtualAccountTrxType"`
	ExpiredDate           string                 `json:"expiredDate,omitempty"`
	FreeText              []FreeText             `json:"freeText,omitempty"`
}

func boolPtr(b bool) *bool { return &b }

// ZeroPadPartnerServiceID zero-pads partnerServiceID to 8 digits for use
// inside virtualAccountNo. DOKU's docs specify partnerServiceId itself as
// space-padded to 8 chars (e.g. "   19008"), but require the zero-padded form
// (e.g. "00019008") when composing virtualAccountNo — using the space-padded
// form there produces an invalid VA number containing literal spaces.
func ZeroPadPartnerServiceID(partnerServiceID string) string {
	trimmed := strings.TrimSpace(partnerServiceID)
	if len(trimmed) >= 8 {
		return trimmed
	}
	return strings.Repeat("0", 8-len(trimmed)) + trimmed
}

// newCreateVARequest builds the common fields shared by the Static and
// Non-static constructors below. bankKey must be a key present in
// BankChannels (e.g. "bca").
func newCreateVARequest(bankKey, partnerServiceID, customerNo, virtualAccountName, trxID string, amount Amount) *CreateVARequest {
	return &CreateVARequest{
		PartnerServiceID:   partnerServiceID,
		CustomerNo:         customerNo,
		VirtualAccountNo:   partnerServiceID + customerNo,
		VirtualAccountName: virtualAccountName,
		TrxID:              trxID,
		TotalAmount:        amount,
		AdditionalInfo: CreateVAAdditionalInfo{
			Channel: BankChannels[bankKey],
		},
		VirtualAccountTrxType: VATrxTypeClosed,
	}
}

// NewStaticVARequest builds a CreateVARequest with reusableStatus=true — the
// VA number can be paid more than once (Xendit's "Fixed VA" equivalent).
func NewStaticVARequest(bankKey, partnerServiceID, customerNo, virtualAccountName, trxID string, amount Amount) *CreateVARequest {
	req := newCreateVARequest(bankKey, partnerServiceID, customerNo, virtualAccountName, trxID, amount)
	req.AdditionalInfo.VirtualAccountConfig = &VirtualAccountConfig{ReusableStatus: boolPtr(true)}
	return req
}

// NewSingleUseVARequest builds a CreateVARequest that leaves reusableStatus
// unset — DOKU defaults it to false, meaning the VA can only be paid once.
func NewSingleUseVARequest(bankKey, partnerServiceID, customerNo, virtualAccountName, trxID string, amount Amount) *CreateVARequest {
	return newCreateVARequest(bankKey, partnerServiceID, customerNo, virtualAccountName, trxID, amount)
}

type vaAdditionalInfoRef struct {
	Channel string `json:"channel"`
}

type DeleteVARequest struct {
	PartnerServiceID string              `json:"partnerServiceId"`
	CustomerNo       string              `json:"customerNo"`
	VirtualAccountNo string              `json:"virtualAccountNo"`
	TrxID            string              `json:"trxId"`
	AdditionalInfo   vaAdditionalInfoRef `json:"additionalInfo"`
}

// UpdateVARequest supports partial updates — DOKU's spec requires fewer
// fields here than on CreateVARequest (e.g. VirtualAccountName/TotalAmount
// are optional), so fields use omitempty even where CreateVARequest doesn't.
type UpdateVARequest struct {
	PartnerServiceID      string                 `json:"partnerServiceId"`
	CustomerNo            string                 `json:"customerNo"`
	VirtualAccountNo      string                 `json:"virtualAccountNo"`
	VirtualAccountName    string                 `json:"virtualAccountName,omitempty"`
	VirtualAccountEmail   string                 `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone   string                 `json:"virtualAccountPhone,omitempty"`
	TrxID                 string                 `json:"trxId"`
	TotalAmount           *Amount                `json:"totalAmount,omitempty"`
	AdditionalInfo        CreateVAAdditionalInfo `json:"additionalInfo"`
	VirtualAccountTrxType string                 `json:"virtualAccountTrxType,omitempty"`
	ExpiredDate           string                 `json:"expiredDate,omitempty"`
}

type CheckStatusRequest struct {
	PartnerServiceID string      `json:"partnerServiceId"`
	CustomerNo       string      `json:"customerNo"`
	VirtualAccountNo string      `json:"virtualAccountNo"`
	InquiryRequestID string      `json:"inquiryRequestId,omitempty"`
	PaymentRequestID string      `json:"paymentRequestId,omitempty"`
	AdditionalInfo   interface{} `json:"additionalInfo"`
}
