package dokugo

// SNAP endpoint paths, relative to Client.BaseURL.
const (
	pathCreateVA    = "/virtual-accounts/bi-snap-va/v1.1/transfer-va/create-va"
	pathDeleteVA    = "/virtual-accounts/bi-snap-va/v1.1/transfer-va/delete-va"
	pathUpdateVA    = "/virtual-accounts/bi-snap-va/v1.1/transfer-va/update-va"
	pathCheckStatus = "/orders/v1.0/transfer-va/status" // different path family than the VA endpoints above
)

// Kirim DOKU (disbursement/payout) endpoint paths, confirmed against
// developers.doku.com/payout/kirim-doku/*.
const (
	pathAccountInquiry = "/snap/v1.1/emoney/bank-account-inquiry"
	pathBalanceInquiry = "/snap/v1.1/balance-inquiry"
	pathTransferBank   = "/snap/v1.1/emoney/transfer-bank"

	// pathDisbursementCheckStatus is UNCONFIRMED. DOKU's own docs page
	// (developers.doku.com/payout/kirim-doku/check-status) has a
	// documentation bug: it's titled "KIRIMDOKU Check Status" but the
	// embedded OpenAPI operation is actually /snap/v1.1/qr/qr-mpm-status
	// (a QR payment status endpoint, unrelated to bank transfers). This
	// guess follows the naming convention of the other confirmed emoney
	// endpoints above — verify with DOKU directly before relying on it.
	pathDisbursementCheckStatus = "/snap/v1.1/emoney/transfer-status"
)
