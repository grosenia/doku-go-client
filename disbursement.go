package dokugo

import "net/http"

// AccountInquiry validates a destination bank account/holder name and
// returns a SessionID that must be passed into the following TransferBank
// call. Always call this immediately before TransferBank — DOKU's docs don't
// specify the SessionID's expiry window.
func (g *Gateway) AccountInquiry(req *AccountInquiryRequest) (*AccountInquiryResponse, error) {
	resp := &AccountInquiryResponse{}
	_, err := g.doRequest(http.MethodPost, pathAccountInquiry, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// TransferBank disburses funds to a bank account previously validated via
// AccountInquiry. PartnerReferenceNo should be a stable idempotency key
// (e.g. derived from the payout record ID), since DOKU's retry/duplicate
// behavior on repeated PartnerReferenceNo values isn't documented.
func (g *Gateway) TransferBank(req *TransferBankRequest) (*TransferBankResponse, error) {
	resp := &TransferBankResponse{}
	_, err := g.doRequest(http.MethodPost, pathTransferBank, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// BalanceInquiry checks Grosenia's own DOKU account balance.
func (g *Gateway) BalanceInquiry(req *BalanceInquiryRequest) (*BalanceInquiryResponse, error) {
	resp := &BalanceInquiryResponse{}
	_, err := g.doRequest(http.MethodPost, pathBalanceInquiry, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// CheckDisbursementStatus polls a prior TransferBank call's status. The
// endpoint path is UNCONFIRMED — see pathDisbursementCheckStatus in urls.go.
// Do not rely on this in production until verified against DOKU sandbox.
func (g *Gateway) CheckDisbursementStatus(req *DisbursementCheckStatusRequest) (*DisbursementCheckStatusResponse, error) {
	resp := &DisbursementCheckStatusResponse{}
	_, err := g.doRequest(http.MethodPost, pathDisbursementCheckStatus, req, resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
