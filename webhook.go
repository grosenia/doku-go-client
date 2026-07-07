package dokugo

// PaymentNotificationRequest is the payload DOKU POSTs to the merchant's
// registered notification URL when a Virtual Account is paid.
type PaymentNotificationRequest struct {
	PartnerServiceID    string `json:"partnerServiceId"`
	CustomerNo          string `json:"customerNo"`
	VirtualAccountNo    string `json:"virtualAccountNo"`
	VirtualAccountName  string `json:"virtualAccountName"`
	VirtualAccountEmail string `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone string `json:"virtualAccountPhone,omitempty"`
	TrxID               string `json:"trxId"`
	PaymentRequestID    string `json:"paymentRequestId"`
	PaidAmount          Amount `json:"paidAmount"`
	AdditionalInfo      struct {
		Channel              string                `json:"channel"`
		VirtualAccountConfig *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
	} `json:"additionalInfo"`
	TrxDateTime           string `json:"trxDateTime,omitempty"`
	VirtualAccountTrxType string `json:"virtualAccountTrxType,omitempty"`
}

// PaymentNotificationResponse is the exact shape the merchant's handler must
// return in its HTTP response body — dictated by DOKU's spec, not ours.
type PaymentNotificationResponse struct {
	ResponseCode       string `json:"responseCode"`
	ResponseMessage    string `json:"responseMessage"`
	VirtualAccountData struct {
		PartnerServiceID      string `json:"partnerServiceId"`
		CustomerNo            string `json:"customerNo"`
		VirtualAccountNo      string `json:"virtualAccountNo"`
		VirtualAccountName    string `json:"virtualAccountName"`
		PaymentRequestID      string `json:"paymentRequestId"`
		PaidAmount            Amount `json:"paidAmount"`
		VirtualAccountTrxType string `json:"virtualAccountTrxType,omitempty"`
	} `json:"virtualAccountData"`
	AdditionalInfo struct {
		Channel              string                `json:"channel"`
		VirtualAccountConfig *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
	} `json:"additionalInfo"`
}

// NewPaymentNotificationAck builds the acknowledgement response DOKU expects
// after a successfully processed payment notification, echoing back the
// fields DOKU's spec requires.
func NewPaymentNotificationAck(req PaymentNotificationRequest) PaymentNotificationResponse {
	resp := PaymentNotificationResponse{
		// "2002500" follows DOKU's observed pattern (2 + 00 + serviceCode + 00,
		// e.g. Create VA/27 -> "2002700", Delete/31 -> "2003100") assuming
		// service code 25 for payment notifications — NOT a literal example
		// confirmed in public docs. Verify this exact value against a real
		// DOKU sandbox notification before relying on it in production.
		ResponseCode:    "2002500",
		ResponseMessage: "Successful",
	}
	resp.VirtualAccountData.PartnerServiceID = req.PartnerServiceID
	resp.VirtualAccountData.CustomerNo = req.CustomerNo
	resp.VirtualAccountData.VirtualAccountNo = req.VirtualAccountNo
	resp.VirtualAccountData.VirtualAccountName = req.VirtualAccountName
	resp.VirtualAccountData.PaymentRequestID = req.PaymentRequestID
	resp.VirtualAccountData.PaidAmount = req.PaidAmount
	resp.VirtualAccountData.VirtualAccountTrxType = req.VirtualAccountTrxType
	resp.AdditionalInfo.Channel = req.AdditionalInfo.Channel
	return resp
}
