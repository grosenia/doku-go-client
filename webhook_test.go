package dokugo

import "testing"

func TestNewPaymentNotificationAck_EchoesRequiredFields(t *testing.T) {
	req := PaymentNotificationRequest{
		PartnerServiceID:   "   19008",
		CustomerNo:         "00000000000000000001",
		VirtualAccountNo:   "   190080" + "0000000000000001",
		VirtualAccountName: "Customer Name",
		PaymentRequestID:   "abcdef-123456",
		PaidAmount:         Amount{Value: "11500.00", Currency: "IDR"},
	}
	req.AdditionalInfo.Channel = "VIRTUAL_ACCOUNT_BCA"

	ack := NewPaymentNotificationAck(req)

	if ack.ResponseCode == "" || ack.ResponseMessage == "" {
		t.Fatal("expected non-empty responseCode/responseMessage")
	}
	if ack.VirtualAccountData.PartnerServiceID != req.PartnerServiceID ||
		ack.VirtualAccountData.CustomerNo != req.CustomerNo ||
		ack.VirtualAccountData.VirtualAccountNo != req.VirtualAccountNo ||
		ack.VirtualAccountData.VirtualAccountName != req.VirtualAccountName ||
		ack.VirtualAccountData.PaymentRequestID != req.PaymentRequestID ||
		ack.VirtualAccountData.PaidAmount != req.PaidAmount {
		t.Fatalf("ack did not correctly echo request fields: %+v", ack.VirtualAccountData)
	}
	if ack.AdditionalInfo.Channel != req.AdditionalInfo.Channel {
		t.Fatalf("ack.AdditionalInfo.Channel = %s, want %s", ack.AdditionalInfo.Channel, req.AdditionalInfo.Channel)
	}
}
