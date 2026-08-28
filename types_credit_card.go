package dokugo

// Credit/debit card payment (DOKU's "Card" product) is a separate system
// from everything else in this package — non-SNAP, its own HMAC-SHA256
// signature scheme (see credit_card.go's signNonSNAP) and its own host path
// family (`/credit-card/...`, not `/snap/v1.1/...`). It DOES reuse the same
// Client-Id/Secret Key as SNAP, though (confirmed 2026-08-27 against real
// sandbox — Grosenia's "BRN-0268-..." credential worked unchanged; DOKU's
// docs example showing an "MCH-0001-..." Client-Id was just a generic
// placeholder, not proof of a separate namespace). Captured 2026-08-24 from
// developers.doku.com's "Payment Page Integration Guide" (non-PCI-DSS path —
// Grosenia isn't PCI-DSS certified, so the H2H path that handles raw card
// data directly doesn't apply). Do not mix these types with the SNAP
// VA/disbursement types elsewhere in this package.

// PaymentPageOrder is CreatePaymentPageRequest's "order" object.
type PaymentPageOrder struct {
	InvoiceNumber string                `json:"invoice_number"` // max 64 (30 if Mandiri acquirer)
	Amount        int64                 `json:"amount"`         // IDR, no decimals
	LineItems     []PaymentPageLineItem `json:"line_items,omitempty"`
	CallbackURL   string                `json:"callback_url,omitempty"` // mandatory if AutoRedirect is true
	FailedURL     string                `json:"failed_url,omitempty"`   // falls back to CallbackURL if empty
	AutoRedirect  bool                  `json:"auto_redirect"`
	Descriptor    string                `json:"descriptor,omitempty"` // max 22, needs DOKU to activate first
}

type PaymentPageLineItem struct {
	Name     string `json:"name,omitempty"`
	Price    int64  `json:"price,omitempty"`
	Quantity int64  `json:"quantity,omitempty"`
}

type PaymentPageCustomer struct {
	ID      string `json:"id,omitempty"` // mandatory to use tokenization
	Name    string `json:"name,omitempty"`
	Email   string `json:"email,omitempty"` // one of Email/Phone required — never send a static/dummy value, hurts approval rate
	Phone   string `json:"phone,omitempty"` // format: {calling_code}{number}, e.g. "6281122334455"
	Address string `json:"address,omitempty"`
	Country string `json:"country,omitempty"` // ISO 3166-1 alpha-2, e.g. "ID"
}

// PaymentTypeSale/Authorize/Installment are payment.type values. MOTO and
// Recurring exist too but require the H2H API path, not the Payment Page —
// deliberately not modeled here.
const (
	PaymentTypeSale        = "SALE"
	PaymentTypeAuthorize   = "AUTHORIZE"
	PaymentTypeInstallment = "INSTALLMENT"
)

type PaymentPagePayment struct {
	Type        string `json:"type,omitempty"`
	AutoCapture bool   `json:"auto_capture,omitempty"`
	// Acquirer/Tenor become mandatory when Type is PaymentTypeInstallment.
	// Acquirer possible values: BNI, BRI, BANK_CIMB, BANK_MANDIRI, BCA,
	// BANK_PERMATA, DANAMON, BUKOPIN, HSBC, OCBC_NISP.
	Acquirer string `json:"acquirer,omitempty"`
	Tenor    int    `json:"tenor,omitempty"`
}

type PaymentPageThemes struct {
	Language              string `json:"language,omitempty"` // "EN" or "ID", default "EN"
	BackgroundColor       string `json:"background_color,omitempty"`
	FontColor             string `json:"font_color,omitempty"`
	ButtonBackgroundColor string `json:"button_background_color,omitempty"`
	ButtonFontColor       string `json:"button_font_color,omitempty"`
}

type PaymentPagePromo struct {
	BIN            string `json:"bin,omitempty"`
	DiscountAmount int64  `json:"discount_amount,omitempty"`
}

type PaymentPageOverrideConfiguration struct {
	Themes     *PaymentPageThemes `json:"themes,omitempty"`
	Promo      []PaymentPagePromo `json:"promo,omitempty"`
	AllowBIN   []string           `json:"allow_bin,omitempty"`
	AllowTenor []int              `json:"allow_tenor,omitempty"`
}

type PaymentPageDisclaimer struct {
	ID string `json:"id,omitempty"` // Indonesian message
	EN string `json:"en,omitempty"` // English message (default)
}

type PaymentPageAdditionalInfo struct {
	OverrideNotificationURL string                 `json:"override_notification_url,omitempty"`
	Disclaimer              *PaymentPageDisclaimer `json:"disclaimer,omitempty"`
}

// PaymentPageCard lets the caller pre-fill a previously-tokenized card, or
// force the customer to save this payment's card for next time.
type PaymentPageCard struct {
	Token string `json:"token,omitempty"`
	Save  bool   `json:"save,omitempty"`
}

// CreatePaymentPageRequest is the body for POST /credit-card/v1/payment-page
// — generates a hosted payment page URL (or a DOKU JS session) for the
// customer to enter their card details on, so Grosenia (non-PCI-DSS) never
// touches raw card data.
type CreatePaymentPageRequest struct {
	Order                 PaymentPageOrder                  `json:"order"`
	Card                  *PaymentPageCard                  `json:"card,omitempty"`
	Customer              PaymentPageCustomer               `json:"customer,omitempty"`
	Payment               PaymentPagePayment                `json:"payment,omitempty"`
	OverrideConfiguration *PaymentPageOverrideConfiguration `json:"override_configuration,omitempty"`
	AdditionalInfo        *PaymentPageAdditionalInfo        `json:"additional_info,omitempty"`
}

type creditCardPaymentPage struct {
	URL string `json:"url"`
}

// PaymentPlanCode is one of credit_card_js.payment_plan_codes — the installment/full-payment
// options DOKU JS lets the customer choose from, only present alongside a real SessionID.
type PaymentPlanCode struct {
	Code        string `json:"code"`
	Type        string `json:"type"` // e.g. "FULL_PAYMENT"
	TotalAmount int64  `json:"total_amount"`
	Currency    string `json:"currency"`
}

type creditCardJS struct {
	SessionID        string            `json:"session_id"`
	PaymentPlanCodes []PaymentPlanCode `json:"payment_plan_codes,omitempty"`
}

// CreatePaymentPageResponse is what DOKU returns for a successful payment-page
// request. CreditCardPaymentPage.URL is what the merchant shows the customer
// (iframe or dedicated page); CreditCardJS.SessionID is only populated for
// merchants using the DOKU JS integration instead — confirmed EMPTY against
// Grosenia's real sandbox merchant as of 2026-08-28 (same request that returns
// a valid CreditCardPaymentPage.URL returns an empty SessionID). Per
// developers.doku.com's Payment Page Integration Guide, this is a
// merchant-level toggle DOKU enables on their side, not something requestable
// per-call — same category as the customerNo/avaliableBalance/KirimDOKU
// balance-account gotchas elsewhere in this package: ask DOKU support to
// enable "DOKU JS integration" for the merchant before expecting a non-empty
// SessionID here.
type CreatePaymentPageResponse struct {
	Order struct {
		InvoiceNumber string                `json:"invoice_number"`
		LineItems     []PaymentPageLineItem `json:"line_items,omitempty"`
	} `json:"order"`
	CreditCardPaymentPage creditCardPaymentPage      `json:"credit_card_payment_page"`
	CreditCardJS          creditCardJS               `json:"credit_card_js"`
	AdditionalInfo        *PaymentPageAdditionalInfo `json:"additional_info,omitempty"`
	CreditCardErrorResponse
}

// CreditCardErrorResponse is DOKU's error shape for this product family —
// deliberately NOT the SNAP ErrorResponse type (different field names,
// nested under "error"). Check ErrorStatus after every call, same
// convention as the rest of this package.
//
// The JSON key is singular "error", NOT "errors" as
// developers.doku.com's own "Payment Page Integration Guide" sample
// response shows — confirmed wrong against a real sandbox call 2026-08-27
// (`curl` with a deliberately-missing Client-Id returned
// `{"error":{"code":"invalid_header_request","message":"Header Client-Id is required","type":"invalid_request_error"}}`).
// Don't "fix" this back to match the docs.
type CreditCardErrorResponse struct {
	Errors struct {
		Code    string `json:"code"`
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
	ErrorStatus bool `json:"-"`
}

func (e *CreditCardErrorResponse) markHTTPError(status int) {
	e.ErrorStatus = status < 200 || status >= 300
}

func (e CreditCardErrorResponse) Error() string {
	return e.Errors.Code + ": " + e.Errors.Message
}

// CaptureRequest is the body for POST /credit-card/capture — the second step
// of an AUTHORIZE transaction, must be called within 7 days of the
// authorization or DOKU auto-releases the hold.
type CaptureRequest struct {
	Payment CapturePaymentRequest `json:"payment"`
}

type CapturePaymentRequest struct {
	AuthorizeID   string `json:"authorize_id"`             // from the AUTHORIZE notification/response
	CaptureAmount int64  `json:"capture_amount,omitempty"` // omit to capture the full authorized amount
}

type CaptureIdentifier struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CaptureResponse mirrors DOKU's real captured example (2026-08-24) — note
// Card.Type is "CREDIT" or "DEBIT" (this product handles both despite the
// "credit-card" path naming).
type CaptureResponse struct {
	Order struct {
		InvoiceNumber string `json:"invoice_number"`
		Amount        int64  `json:"amount"`
	} `json:"order"`
	Customer struct {
		ID string `json:"id,omitempty"`
	} `json:"customer"`
	Payment struct {
		Type            string              `json:"type"`
		Identifier      []CaptureIdentifier `json:"identifier,omitempty"`
		RequestID       string              `json:"request_id"`
		AuthorizeID     string              `json:"authorize_id"`
		ResponseCode    string              `json:"response_code"`
		ResponseMessage string              `json:"response_message"`
		ECI             string              `json:"eci"`
		Status          string              `json:"status"` // SUCCESS, FAILED, PENDING
		ApprovalCode    string              `json:"approval_code,omitempty"`
	} `json:"payment"`
	ThreeDSecure struct {
		AuthenticationID string `json:"authentication_id"`
	} `json:"three_dsecure"`
	Card struct {
		Masked string `json:"masked,omitempty"`
		Type   string `json:"type"` // CREDIT or DEBIT
		Issuer string `json:"issuer"`
		Brand  string `json:"brand"` // VISA, MASTER, JCB, AMEX
		Token  string `json:"token,omitempty"`
	} `json:"card"`
	CreditCardErrorResponse
}

// CardNotification is the body DOKU POSTs to the Card product's Notification
// URL (or additional_info.override_notification_url) after a transaction —
// captured verbatim 2026-08-27 from a real sandbox AUTHORIZE payment
// notification. Genuinely different shape from what
// developers.doku.com's docs implied elsewhere in this product (no
// "errors"/"payment" top-level wrapper — flat top-level objects instead):
// this is the authoritative source of payment.authorize_id, which is NOT
// returned anywhere else (not in CreatePaymentPage's response, not in the
// DOKU Back Office dashboard's transaction detail page — confirmed by
// checking both first).
type CardNotification struct {
	Order struct {
		InvoiceNumber string `json:"invoice_number"`
		Amount        int64  `json:"amount"`
	} `json:"order"`
	Customer struct {
		Name  string `json:"name,omitempty"`
		Email string `json:"email,omitempty"`
	} `json:"customer"`
	Transaction struct {
		Type              string `json:"type"` // SALE, AUTHORIZE, INSTALLMENT
		Status            string `json:"status"`
		Date              string `json:"date"`
		OriginalRequestID string `json:"original_request_id"` // echoes the Request-Id from CreatePaymentPage
	} `json:"transaction"`
	Service struct {
		ID string `json:"id"` // "CREDIT_CARD"
	} `json:"service"`
	Acquirer struct {
		ID string `json:"id"`
	} `json:"acquirer"`
	Channel struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"channel"`
	AdditionalInfo struct {
		OverrideNotificationURL string `json:"override_notification_url,omitempty"`
	} `json:"additional_info,omitempty"`
	CardPayment struct {
		MaskedCardNumber   string              `json:"masked_card_number,omitempty"`
		ApprovalCode       string              `json:"approval_code,omitempty"`
		ResponseCode       string              `json:"response_code"`
		ResponseMessage    string              `json:"response_message"`
		Issuer             string              `json:"issuer,omitempty"`
		Identifier         []CaptureIdentifier `json:"identifier,omitempty"`
		AuthorizeID        string              `json:"authorize_id,omitempty"` // present when Transaction.Type is AUTHORIZE — pass to CaptureRequest
		Brand              string              `json:"brand,omitempty"`
		AuthenticationID   string              `json:"authentication_id,omitempty"`
		ThreeDSecureStatus string              `json:"three_d_secure_status,omitempty"`
	} `json:"card_payment"`
	Verification struct {
		Status string `json:"status"`
		Reason string `json:"reason,omitempty"`
	} `json:"verification,omitempty"`
}
