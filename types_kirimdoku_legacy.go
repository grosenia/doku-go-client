package dokugo

// Types for the LEGACY (non-SNAP) KirimDOKU API — see kirimdoku_legacy.go and
// docs/KIRIMDOKU_LEGACY_API_REFERENCE.md. Field shapes follow the docs
// directly; CashInInquiry/CashInRemit/TransactionInfo response shapes are not
// yet exercised against a live remit/status call (only Ping and
// CashInInquiry itself were confirmed live 2026-08-14) — verify before
// trusting undocumented-but-guessed nested fields in production.

// Channel codes for KirimDokuLegacyBeneficiaryAccount.
const (
	KirimDokuLegacyChannelBank   = "07" // Bank Deposit
	KirimDokuLegacyChannelWallet = "04" // DOKU Wallet
)

// KirimDokuLegacyStatus is the API-wide status code returned on every
// response ("status" field), per the docs' status table.
const (
	KirimDokuLegacyStatusDefaultError      = -1
	KirimDokuLegacyStatusSuccess           = 0
	KirimDokuLegacyStatusUnauthorized      = 9
	KirimDokuLegacyStatusMissingField      = 1
	KirimDokuLegacyStatusInvalidParamType  = 2
	KirimDokuLegacyStatusInvalidParamValue = 11
	KirimDokuLegacyStatusIllegalParam      = 12
	KirimDokuLegacyStatusSecurityIssue     = 13
	KirimDokuLegacyStatusUnsupportedOp     = 16
	KirimDokuLegacyStatusNotFound          = 21
	KirimDokuLegacyStatusAccessBlocked     = 91
)

// KirimDokuLegacyTransactionStatus is transaction.status / remit.transactionStatus.
const (
	KirimDokuLegacyTxStatusUnknown           = "0"
	KirimDokuLegacyTxStatusCashCreated       = "10"
	KirimDokuLegacyTxStatusBankWalletUnpaid  = "20"
	KirimDokuLegacyTxStatusCashLocked        = "30"
	KirimDokuLegacyTxStatusFailed            = "35"
	KirimDokuLegacyTxStatusRefunded          = "40"
	KirimDokuLegacyTxStatusCashFullyRefunded = "41"
	KirimDokuLegacyTxStatusSuccess           = "50"
	KirimDokuLegacyTxStatusTimeout           = "64"
)

type KirimDokuLegacyPingResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// KirimDokuLegacyBalance — CreditLimit/CreditAlertLimit/CreditLastBalance are
// numeric on the wire (e.g. 1000000000000.000000), confirmed against a real
// staging response 2026-08-14 — the docs don't state this, don't assume
// string like other DOKU amount fields elsewhere in this package.
type KirimDokuLegacyBalance struct {
	CorporateName     string  `json:"corporateName"`
	CreditLimit       float64 `json:"creditLimit"`
	CreditAlertLimit  float64 `json:"creditAlertLimit"`
	CreditLastBalance float64 `json:"creditLastBalance"`
}

type KirimDokuLegacyCheckBalanceResponse struct {
	Status  int                    `json:"status"`
	Message string                 `json:"message,omitempty"`
	Balance KirimDokuLegacyBalance `json:"balance"`
}

// KirimDokuLegacyBank identifies the beneficiary bank for a channel-07
// (Bank Deposit) inquiry/remit — id is the DOKU bank code (e.g. "014" BCA),
// see docs/KIRIMDOKU_LEGACY_API_REFERENCE.md's bank list.
type KirimDokuLegacyBank struct {
	Code        string `json:"code,omitempty"`
	CountryCode string `json:"countryCode,omitempty"`
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
}

type KirimDokuLegacyBeneficiaryAccount struct {
	Number  string              `json:"number"`
	Name    string              `json:"name,omitempty"`
	City    string              `json:"city,omitempty"`
	Address string              `json:"address,omitempty"`
	Bank    KirimDokuLegacyBank `json:"bank"`
}

type KirimDokuLegacyChannel struct {
	Code string `json:"code"` // KirimDokuLegacyChannelBank or KirimDokuLegacyChannelWallet
}

// KirimDokuLegacyInquiryRequest validates a beneficiary account before a
// CashInRemit call. For channel "07" set BeneficiaryAccount; for channel
// "04" set BeneficiaryWalletID instead and leave BeneficiaryAccount zero.
type KirimDokuLegacyInquiryRequest struct {
	Channel             KirimDokuLegacyChannel             `json:"channel"`
	SenderCountry       KirimDokuLegacyCountry             `json:"senderCountry"`
	SenderCurrency      KirimDokuLegacyCurrency            `json:"senderCurrency"`
	SenderAmount        string                             `json:"senderAmount"`
	SenderNote          string                             `json:"senderNote,omitempty"`
	BeneficiaryCountry  KirimDokuLegacyCountry             `json:"beneficiaryCountry"`
	BeneficiaryCurrency KirimDokuLegacyCurrency            `json:"beneficiaryCurrency"`
	BeneficiaryAccount  *KirimDokuLegacyBeneficiaryAccount `json:"beneficiaryAccount,omitempty"`
	BeneficiaryWalletID string                             `json:"beneficiaryWalletId,omitempty"`
}

type KirimDokuLegacyCountry struct {
	Code string `json:"code"`
}

type KirimDokuLegacyCurrency struct {
	Code string `json:"code"`
}

type KirimDokuLegacyFund struct {
	Origin      string `json:"origin,omitempty"`
	Destination string `json:"destination,omitempty"`
	Fees        string `json:"fees,omitempty"`
}

type KirimDokuLegacyInquiryData struct {
	IDToken string `json:"idToken"`
}

type KirimDokuLegacyInquiryResponse struct {
	Status             int                                `json:"status"`
	Message            string                             `json:"message,omitempty"`
	Inquiry            KirimDokuLegacyInquiryData         `json:"inquiry"`
	Fund               KirimDokuLegacyFund                `json:"fund,omitempty"`
	BeneficiaryAccount *KirimDokuLegacyBeneficiaryAccount `json:"beneficiaryAccount,omitempty"`
}

type KirimDokuLegacyPersonalID struct {
	Type    string `json:"personalIdType,omitempty"`
	ID      string `json:"personalId,omitempty"`
	Country string `json:"personalIdCountry,omitempty"`
}

type KirimDokuLegacySender struct {
	Name      string `json:"name"`
	Phone     string `json:"phone,omitempty"`
	BirthDate string `json:"birthDate,omitempty"`
	Gender    string `json:"gender,omitempty"`
	Address   string `json:"address,omitempty"`
	KirimDokuLegacyPersonalID
}

type KirimDokuLegacyBeneficiary struct {
	Name    string `json:"name"`
	Phone   string `json:"phone,omitempty"`
	Country string `json:"country,omitempty"`
	Address string `json:"address,omitempty"`
}

// KirimDokuLegacyRemitRequest executes the transfer validated by a prior
// CashInInquiry call — InquiryIDToken must be that call's Inquiry.IDToken.
type KirimDokuLegacyRemitRequest struct {
	Channel             KirimDokuLegacyChannel             `json:"channel"`
	InquiryIDToken      string                             `json:"inquiryIdToken"`
	SendTrxID           string                             `json:"sendTrxId,omitempty"`
	Sender              KirimDokuLegacySender              `json:"sender"`
	Beneficiary         KirimDokuLegacyBeneficiary         `json:"beneficiary"`
	BeneficiaryAccount  *KirimDokuLegacyBeneficiaryAccount `json:"beneficiaryAccount,omitempty"`
	BeneficiaryWalletID string                             `json:"beneficiaryWalletId,omitempty"`
}

type KirimDokuLegacyPaymentData struct {
	ResponseCode string `json:"responseCode"`
}

type KirimDokuLegacyRemitData struct {
	PaymentData       KirimDokuLegacyPaymentData `json:"paymentData"`
	TransactionID     string                     `json:"transactionId"`
	TransactionStatus string                     `json:"transactionStatus"` // one of KirimDokuLegacyTxStatus*
}

type KirimDokuLegacyRemitResponse struct {
	Status  int                      `json:"status"`
	Message string                   `json:"message,omitempty"`
	Warning string                   `json:"warning,omitempty"`
	Remit   KirimDokuLegacyRemitData `json:"remit"`
}

// KirimDokuLegacyTransactionLog carries the human-readable status/reference
// info within a TransactionInfo response.
type KirimDokuLegacyTransactionLog struct {
	Status           string `json:"status,omitempty"`
	StatusMessage    string `json:"statusMessage,omitempty"`
	ReferenceStatus  string `json:"referenceStatus,omitempty"`
	ReferenceMessage string `json:"referenceMessage,omitempty"`
	ChannelCode      string `json:"channelCode,omitempty"`
}

// KirimDokuLegacyTransactionDetail's sender/beneficiary/agent/channel/fund
// sub-shapes aren't detailed in the docs beyond their existence, so they're
// left as raw maps rather than guessed structs — inspect at runtime if
// needed, don't assume a shape until confirmed against a real response.
type KirimDokuLegacyTransactionDetail struct {
	ID             string                        `json:"id"`
	InquiryID      string                        `json:"inquiryId,omitempty"`
	SendTrxID      string                        `json:"sendTrxId,omitempty"`
	Status         string                        `json:"status"` // one of KirimDokuLegacyTxStatus*
	CreatedTime    string                        `json:"createdTime,omitempty"`
	CashInTime     string                        `json:"cashInTime,omitempty"`
	Agent          map[string]interface{}        `json:"agent,omitempty"`
	Channel        map[string]interface{}        `json:"channel,omitempty"`
	Sender         map[string]interface{}        `json:"sender,omitempty"`
	Beneficiary    map[string]interface{}        `json:"beneficiary,omitempty"`
	Fund           map[string]interface{}        `json:"fund,omitempty"`
	TransactionLog KirimDokuLegacyTransactionLog `json:"transactionlog,omitempty"`
}

type KirimDokuLegacyTransactionInfoResponse struct {
	Status      int                              `json:"status"`
	Message     string                           `json:"message,omitempty"`
	Transaction KirimDokuLegacyTransactionDetail `json:"transaction"`
}

// KirimDokuLegacyRefundNotification is the payload DOKU POSTs to a
// partner-registered callback URL when a refund occurs.
type KirimDokuLegacyRefundNotification struct {
	ActivityCode  string `json:"activityCode"`
	TransactionID string `json:"transactionId"`
	ProcessDate   int64  `json:"processDate"` // epoch milliseconds
	Message       string `json:"message,omitempty"`
}

// KirimDokuLegacyRefundNotificationResponse is the exact shape the partner's
// handler must return — dictated by DOKU's spec. ResponseCode "00" =
// processed successfully, "01" = already processed before.
type KirimDokuLegacyRefundNotificationResponse struct {
	Status          string `json:"status"` // "true" or "false"
	TransactionID   string `json:"transactionId"`
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
}

// NewKirimDokuLegacyRefundNotificationAck builds the acknowledgement response
// for a successfully processed refund notification.
func NewKirimDokuLegacyRefundNotificationAck(req KirimDokuLegacyRefundNotification) KirimDokuLegacyRefundNotificationResponse {
	return KirimDokuLegacyRefundNotificationResponse{
		Status:          "true",
		TransactionID:   req.TransactionID,
		ResponseCode:    "00",
		ResponseMessage: "Successful",
	}
}

// NewKirimDokuLegacyRefundNotificationAlreadyProcessed builds the response for
// a refund notification retry that was already handled — DOKU retries up to
// 3 times until the partner responds correctly, so this must be idempotent.
func NewKirimDokuLegacyRefundNotificationAlreadyProcessed(req KirimDokuLegacyRefundNotification) KirimDokuLegacyRefundNotificationResponse {
	return KirimDokuLegacyRefundNotificationResponse{
		Status:          "true",
		TransactionID:   req.TransactionID,
		ResponseCode:    "01",
		ResponseMessage: "Already Processed",
	}
}
