package dokugo

// Disbursement (Kirim DOKU) covers sending funds FROM Grosenia TO a bank
// account (e.g. paying a seller) — the reverse direction of Virtual Account,
// which collects funds FROM a buyer TO Grosenia. See developers.doku.com/payout/kirim-doku.

// AccountInquiryAdditionalInfo carries the destination bank's channel/code —
// required so DOKU knows which bank rail to validate the account against.
type AccountInquiryAdditionalInfo struct {
	ChannelCode            string `json:"channelCode,omitempty"`
	BeneficiaryBankCode    string `json:"beneficiaryBankCode"`
	BeneficiaryAccountName string `json:"beneficiaryAccountName,omitempty"`
	SenderCountryCode      string `json:"senderCountryCode,omitempty"`
}

// AccountInquiryRequest validates a destination bank account and its holder
// name BEFORE calling TransferBank — DOKU's docs describe this as the
// mandatory first step ("Account Inquiry → Balance Inquiry → Transfer →
// Status Check").
type AccountInquiryRequest struct {
	PartnerReferenceNo       string                       `json:"partnerReferenceNo,omitempty"` // max 64
	CustomerNumber           string                       `json:"customerNumber"`               // Grosenia's own DOKU-registered sender identifier, NOT the beneficiary's phone
	BeneficiaryAccountNumber string                       `json:"beneficiaryAccountNumber"`     // max 32
	Amount                   Amount                       `json:"amount"`
	AdditionalInfo           AccountInquiryAdditionalInfo `json:"additionalInfo"`
}

type AccountInquiryResponseAdditionalInfo struct {
	Forex map[string]interface{} `json:"forex,omitempty"`
	Fee   map[string]interface{} `json:"fee,omitempty"`
}

type AccountInquiryResponse struct {
	BeneficiaryAccountNumber string                               `json:"beneficiaryAccountNumber"`
	BeneficiaryAccountName   string                               `json:"beneficiaryAccountName"`
	BeneficiaryBankCode      string                               `json:"beneficiaryBankCode"`
	BeneficiaryBankShortName string                               `json:"beneficiaryBankShortName,omitempty"`
	BeneficiaryBankName      string                               `json:"beneficiaryBankName,omitempty"`
	Amount                   Amount                               `json:"amount"`
	SessionID                string                               `json:"sessionId"` // must be echoed back into TransferBankRequest.SessionID
	AdditionalInfo           AccountInquiryResponseAdditionalInfo `json:"additionalInfo,omitempty"`
	ErrorResponse
}

// TransferBankAdditionalInfo — all beneficiary/sender fields are REQUIRED per
// DOKU's docs (unlike AccountInquiry, where most are optional), since this is
// the actual money-movement call.
type TransferBankAdditionalInfo struct {
	BeneficiaryFirstName   string `json:"beneficiaryFirstName"`
	BeneficiaryLastName    string `json:"beneficiaryLastName"`
	BeneficiaryPhoneNumber string `json:"beneficiaryPhoneNumber"`
	BeneficiaryAccountName string `json:"beneficiaryAccountName"`
	BeneficiaryBankName    string `json:"beneficiaryBankName,omitempty"`
	SenderCountryCode      string `json:"senderCountryCode"` // ISO 3166-2, e.g. "ID"
	SenderFirstName        string `json:"senderFirstName"`
	SenderLastName         string `json:"senderLastName"`
	SenderPersonalID       string `json:"senderPersonalId"`
	SenderPersonalIDType   string `json:"senderPersonalIdType"` // "KTP" or "PASSPORT"
	// ChannelCode: "07" = Bank Deposit, "11" = BI-FAST.
	ChannelCode string `json:"channelCode,omitempty"`
	Remark      string `json:"remark,omitempty"`
}

// TransferBankRequest executes the actual fund transfer. SessionID MUST come
// from a prior AccountInquiryResponse — DOKU rejects transfers whose
// SessionID doesn't match a recent inquiry (exact expiry window unconfirmed).
type TransferBankRequest struct {
	PartnerReferenceNo       string                     `json:"partnerReferenceNo"` // max 64, unique per transfer — use as idempotency key
	CustomerNumber           string                     `json:"customerNumber"`
	BeneficiaryAccountNumber string                     `json:"beneficiaryAccountNumber"`
	BeneficiaryBankCode      string                     `json:"beneficiaryBankCode"`
	Amount                   Amount                     `json:"amount"`
	SessionID                string                     `json:"sessionId"` // from AccountInquiryResponse.SessionID, max 25
	FeeType                  string                     `json:"feeType,omitempty"`
	AdditionalInfo           TransferBankAdditionalInfo `json:"additionalInfo"`
}

type TransferBankResponseAdditionalInfo struct {
	SessionID string `json:"sessionId,omitempty"`
	Amount    Amount `json:"amount,omitempty"`
}

type TransferBankResponse struct {
	ReferenceNo        string                             `json:"referenceNo"`
	PartnerReferenceNo string                             `json:"partnerReferenceNo"`
	TransactionDate    string                             `json:"transactionDate"`
	ReferenceNumber    string                             `json:"referenceNumber,omitempty"`
	AdditionalInfo     TransferBankResponseAdditionalInfo `json:"additionalInfo,omitempty"`
	ErrorResponse
}

// BalanceInquiryRequest checks Grosenia's own DOKU account balance before
// disbursing — DOKU's docs list this as step 2 of the Kirim DOKU flow.
type BalanceInquiryRequest struct {
	PartnerReferenceNo string `json:"partnerReferenceNo,omitempty"` // max 64
	AccountNo          string `json:"accountNo"`                    // max 16
}

type BalanceInquiryResponseAccountInfos struct {
	HoldAmount Amount `json:"holdAmount"`
	// AvaliableBalance is spelled exactly as DOKU's own API returns it
	// (missing the second "a" in "Available") — do not "fix" the JSON tag,
	// it must match the wire format.
	AvaliableBalance Amount `json:"avaliableBalance"`
}

type BalanceInquiryResponse struct {
	AccountNo      string                             `json:"accountNo"`
	Name           string                             `json:"name"`
	AccountInfos   BalanceInquiryResponseAccountInfos `json:"accountInfos"`
	AdditionalInfo struct {
		CreditAlertLimit Amount `json:"creditAlertLimit,omitempty"`
	} `json:"additionalInfo,omitempty"`
	ErrorResponse
}

// DisbursementCheckStatusServiceCode is the fixed serviceCode value for
// polling a Transfer to Bank (KirimDOKU disbursement) transaction —
// "Transfer Status Inquiry (Disbursement)" per ASPI's Tabel Jenis
// Pengujian Devsite. This is a generic status-check endpoint shared across
// several transfer types; other serviceCode values poll other kinds of
// transactions (not implemented here — this package only ever originates
// TransferBank calls).
const DisbursementCheckStatusServiceCode = "53"

// DisbursementCheckStatusRequest polls the status of a prior TransferBank
// call — confirmed against ASPI Devsite's sandbox 2026-08-14, see
// pathDisbursementCheckStatus in urls.go.
type DisbursementCheckStatusRequest struct {
	OriginalPartnerReferenceNo string `json:"originalPartnerReferenceNo"` // from TransferBankRequest.PartnerReferenceNo
	OriginalReferenceNo        string `json:"originalReferenceNo"`        // from TransferBankResponse.ReferenceNo
	OriginalExternalID         string `json:"originalExternalId,omitempty"`
	ServiceCode                string `json:"serviceCode"` // use DisbursementCheckStatusServiceCode
	TransactionDate            string `json:"transactionDate"`
	Amount                     Amount `json:"amount"`
	AdditionalInfo             struct {
		DeviceID string `json:"deviceId,omitempty"`
		Channel  string `json:"channel,omitempty"`
	} `json:"additionalInfo,omitempty"`
}

// Disbursement transaction status codes (latestTransactionStatus).
const (
	DisbursementStatusSuccess  = "00"
	DisbursementStatusPending  = "03"
	DisbursementStatusRefunded = "04"
	DisbursementStatusFailed   = "06"
)

type DisbursementCheckStatusResponseAdditionalInfo struct {
	DeviceID string `json:"deviceId,omitempty"`
	Channel  string `json:"channel,omitempty"`
}

// DisbursementCheckStatusResponse — field set confirmed against a real
// response from ASPI Devsite's sandbox 2026-08-14 (see
// pathDisbursementCheckStatus in urls.go for the full request/response
// captured). ErrorResponse is embedded for responseCode/responseMessage,
// same convention as every other response type in this package.
type DisbursementCheckStatusResponse struct {
	OriginalReferenceNo        string                                        `json:"originalReferenceNo"`
	OriginalPartnerReferenceNo string                                        `json:"originalPartnerReferenceNo"`
	OriginalExternalID         string                                        `json:"originalExternalId,omitempty"`
	ServiceCode                string                                        `json:"serviceCode"`
	TransactionDate            string                                        `json:"transactionDate"`
	Amount                     Amount                                        `json:"amount"`
	BeneficiaryAccountNo       string                                        `json:"beneficiaryAccountNo,omitempty"`
	BeneficiaryBankCode        string                                        `json:"beneficiaryBankCode,omitempty"`
	SourceAccountNo            string                                        `json:"sourceAccountNo,omitempty"`
	PreviousResponseCode       string                                        `json:"previousResponseCode,omitempty"`
	ReferenceNumber            string                                        `json:"referenceNumber,omitempty"`
	TransactionID              string                                        `json:"transactionId,omitempty"`
	LatestTransactionStatus    string                                        `json:"latestTransactionStatus"` // one of DisbursementStatus*
	TransactionStatusDesc      string                                        `json:"transactionStatusDesc,omitempty"`
	AdditionalInfo             DisbursementCheckStatusResponseAdditionalInfo `json:"additionalInfo,omitempty"`
	ErrorResponse
}
