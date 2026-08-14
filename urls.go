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

	// pathDisbursementCheckStatus: DOKU's own docs page
	// (developers.doku.com/payout/kirim-doku/check-status) has a
	// documentation bug: it's titled "KIRIMDOKU Check Status" but the
	// embedded OpenAPI operation is actually /snap/v1.1/qr/qr-mpm-status
	// (a QR payment status endpoint, unrelated to bank transfers) — ASPI's
	// own Devsite portal has the exact same mix-up on its "Transfer Status
	// Inquiry" menu item. The REAL generic status endpoint was found on
	// ASPI Devsite instead, under Transfer Kredit > Transfer Credit PJP AIS
	// Bank > "Transaction Status Inquiry Bank": `/api/v1.0/transfer/status`
	// (no "emoney" segment, unlike the naming convention of the other
	// confirmed emoney endpoints above), confirmed live against ASPI's
	// sandbox 2026-08-14 with a real prior TransferBank transaction
	// (serviceCode "53" = Transfer Status Inquiry/Disbursement per ASPI's
	// Tabel Jenis Pengujian) — real response: `2003600 Successful`,
	// `latestTransactionStatus: "00"`, `transactionStatusDesc: "success"`.
	// Translated to DOKU's real `/snap/v1.1/...` host by the same api/v1.0
	// <-> snap/v1.1 substitution already confirmed for every other
	// endpoint in this file — still not confirmed against DOKU's own
	// production/sandbox host directly, only against ASPI's simulator.
	pathDisbursementCheckStatus = "/snap/v1.1/transfer/status"
)
