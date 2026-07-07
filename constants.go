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
	"bca": "VIRTUAL_ACCOUNT_BCA",
}
