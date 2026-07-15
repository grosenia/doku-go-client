package dokugo

// Base URLs. DOKU, unlike Xendit, has genuinely different sandbox and
// production hosts, so callers must pick the right one at Client construction.
const (
	DefaultSandboxBaseURL    = "https://api-sandbox.doku.com"
	DefaultProductionBaseURL = "https://api.doku.com"
)

// Fixed header value required on every transactional (symmetric-signed) request.
const ChannelIDHostToHost = "H2H"

// Virtual Account transaction types.
const (
	VATrxTypeClosed       = "C" // Closed payment: exact amount only
	VATrxTypeOpen         = "O" // Open payment: any amount
	VATrxTypeBillVariable = "V" // Bill Variable
)

// BankChannels maps a lowercase bank key to DOKU's SNAP VA channel string
// (`additionalInfo.channel` on Create/Update/Delete VA requests). Add a new
// bank here — no other code changes are needed for VA creation. Confirm the
// bank's exact partnerServiceId format/length with DOKU before enabling it in
// production config; that's merchant-specific and not a public constant.
var BankChannels = map[string]string{
	"bca":     "VIRTUAL_ACCOUNT_BCA",
	"bni":     "VIRTUAL_ACCOUNT_BNI",
	"mandiri": "VIRTUAL_ACCOUNT_BANK_MANDIRI",
	// "VIRTUAL_ACCOUNT_BRI" follows the same pattern as BCA/BNI, but unlike
	// those two this hasn't been confirmed against a real DOKU sandbox call
	// or a literal docs example yet — Mandiri broke the naive
	// "VIRTUAL_ACCOUNT_<BANK>" pattern ("BANK_MANDIRI" instead), so don't
	// treat this as confirmed. Verify before enabling BRI VA in production.
	"bri": "VIRTUAL_ACCOUNT_BRI",
}
