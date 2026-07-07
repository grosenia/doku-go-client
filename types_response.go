package dokugo

type CreateVAResponseAdditionalInfo struct {
	Channel      string `json:"channel"`
	HowToPayPage string `json:"howToPayPage,omitempty"`
	HowToPayAPI  string `json:"howToPayApi,omitempty"`
}

type CreateVAResponseData struct {
	PartnerServiceID      string                         `json:"partnerServiceId"`
	CustomerNo            string                         `json:"customerNo"`
	VirtualAccountNo      string                         `json:"virtualAccountNo"`
	VirtualAccountName    string                         `json:"virtualAccountName"`
	VirtualAccountEmail   string                         `json:"virtualAccountEmail,omitempty"`
	VirtualAccountPhone   string                         `json:"virtualAccountPhone,omitempty"`
	TrxID                 string                         `json:"trxId"`
	TotalAmount           Amount                         `json:"totalAmount"`
	VirtualAccountTrxType string                         `json:"virtualAccountTrxType"`
	ExpiredDate           string                         `json:"expiredDate"`
	AdditionalInfo        CreateVAResponseAdditionalInfo `json:"additionalInfo"`
}

type CreateVAResponse struct {
	VirtualAccountData CreateVAResponseData `json:"virtualAccountData"`
	ErrorResponse
}

type DeleteVAResponseData struct {
	PartnerServiceID string `json:"partnerServiceId"`
	CustomerNo       string `json:"customerNo"`
	VirtualAccountNo string `json:"virtualAccountNo"`
	TrxID            string `json:"trxId"`
	AdditionalInfo   struct {
		Channel              string                `json:"channel"`
		VirtualAccountConfig *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
	} `json:"additionalInfo"`
}

type DeleteVAResponse struct {
	VirtualAccountData DeleteVAResponseData `json:"virtualAccountData"`
	ErrorResponse
}

type UpdateVAResponse struct {
	VirtualAccountData CreateVAResponseData `json:"virtualAccountData"`
	ErrorResponse
}

type BillDetail struct {
	BillAmount Amount `json:"billAmount"`
}

type PaymentFlagReason struct {
	English   string `json:"english"`
	Indonesia string `json:"indonesia"`
}

type CheckStatusResponseData struct {
	PaymentFlagReason PaymentFlagReason `json:"paymentFlagReason,omitempty"`
	PartnerServiceID  string            `json:"partnerServiceId"`
	CustomerNo        string            `json:"customerNo"`
	VirtualAccountNo  string            `json:"virtualAccountNo"`
	InquiryRequestID  string            `json:"inquiryRequestId,omitempty"`
	PaymentRequestID  string            `json:"paymentRequestId,omitempty"`
	PaidAmount        Amount            `json:"paidAmount"`
	BillDetails       []BillDetail      `json:"billDetails,omitempty"`
}

type CheckStatusResponse struct {
	VirtualAccountData CheckStatusResponseData `json:"virtualAccountData"`
	AdditionalInfo     struct {
		Acquirer struct {
			ID string `json:"id"`
		} `json:"acquirer"`
		TrxID string `json:"trxId,omitempty"`
	} `json:"additionalInfo"`
	ErrorResponse
}
