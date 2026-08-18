// Command examples is a manual CLI for exercising doku-go-client against
// DOKU's sandbox. Copy config-sample.props to config.props and fill in real
// sandbox credentials + the path to your RSA private key PEM before running.
package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	dokugo "github.com/grosenia/doku-go-client"
)

func loadProps(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	props := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		props[strings.TrimSpace(parts[0])] = parts[1]
	}
	return props, nil
}

func mustGateway(props map[string]string) *dokugo.Gateway {
	keyPath := props["DOKU_PRIVATE_KEY_PATH"]
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		logFail(fmt.Sprintf("read private key %s: %v", keyPath, err))
		os.Exit(1)
	}
	sandbox := props["DOKU_SANDBOX"] != "false"
	client, err := dokugo.NewClient(props["DOKU_CLIENT_ID"], props["DOKU_SECRET_KEY"], keyPEM, sandbox)
	if err != nil {
		logFail(fmt.Sprintf("NewClient: %v", err))
		os.Exit(1)
	}
	return dokugo.NewGateway(&client)
}

func mustKirimDokuLegacyClient(props map[string]string) *dokugo.KirimDokuLegacyClient {
	sandbox := props["KIRIMDOKU_LEGACY_SANDBOX"] != "false"
	return dokugo.NewKirimDokuLegacyClient(props["KIRIMDOKU_LEGACY_AGENT_KEY"], props["KIRIMDOKU_LEGACY_ENCRYPTION_KEY"], sandbox)
}

func arg(i int, fallback string) string {
	if i < len(os.Args) {
		return os.Args[i]
	}
	return fallback
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("usage: go run . <command> [args...]")
		fmt.Println("VA commands: create-va-static, create-va-single-use, delete-va, update-va, check-status")
		fmt.Println("disbursement commands:")
		fmt.Println("  account-inquiry <beneficiaryAccountNumber> <beneficiaryBankCode> [amount]")
		fmt.Println("  transfer-bank <beneficiaryAccountNumber> <beneficiaryBankCode> <sessionId> [amount]")
		fmt.Println("  balance-inquiry <accountNo>")
		fmt.Println("kirimdoku legacy (non-SNAP) commands:")
		fmt.Println("  kirimdoku-ping")
		fmt.Println("  kirimdoku-check-balance")
		fmt.Println("  kirimdoku-inquiry <beneficiaryAccountNumber> <bankID> [amount]")
		fmt.Println("  kirimdoku-remit <inquiryIdToken> <beneficiaryAccountNumber> <bankID> <beneficiaryName>")
		fmt.Println("  kirimdoku-transaction-info <transactionId>")
		os.Exit(1)
	}
	command := os.Args[1]

	props, err := loadProps("config.props")
	if err != nil {
		logFail(fmt.Sprintf("load config.props (copy from config-sample.props first): %v", err))
		os.Exit(1)
	}

	gw := mustGateway(props)
	bank := props["DOKU_BANK"]
	partnerServiceID := props["DOKU_PARTNER_SERVICE_ID"]
	trxID := fmt.Sprintf("TRX-%d", time.Now().Unix())

	switch command {
	case "create-va-static":
		logBanner("Create Static VA")
		customerNo := arg(2, props["DOKU_CUSTOMER_NO_PREFIX"])
		resp, err := gw.CreateVA(dokugo.NewStaticVARequest(bank, partnerServiceID, customerNo, "Example Customer", trxID, dokugo.Amount{Value: "150000.00", Currency: "IDR"}))
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK(fmt.Sprintf("virtualAccountNo=%s", resp.VirtualAccountData.VirtualAccountNo))

	case "create-va-single-use":
		logBanner("Create Single-Use VA")
		customerNo := arg(2, props["DOKU_CUSTOMER_NO_PREFIX"])
		resp, err := gw.CreateVA(dokugo.NewSingleUseVARequest(bank, partnerServiceID, customerNo, "Example Customer", trxID, dokugo.Amount{Value: "75000.00", Currency: "IDR"}))
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK(fmt.Sprintf("virtualAccountNo=%s", resp.VirtualAccountData.VirtualAccountNo))

	case "delete-va":
		logBanner("Delete VA")
		customerNo := arg(2, "")
		virtualAccountNo := arg(3, dokugo.ZeroPadPartnerServiceID(partnerServiceID)+customerNo)
		resp, err := gw.DeleteVA(&dokugo.DeleteVARequest{
			PartnerServiceID: partnerServiceID,
			CustomerNo:       customerNo,
			VirtualAccountNo: virtualAccountNo,
			TrxID:            trxID,
		})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK("deleted")

	case "update-va":
		logBanner("Update VA")
		customerNo := arg(2, "")
		virtualAccountNo := arg(3, dokugo.ZeroPadPartnerServiceID(partnerServiceID)+customerNo)
		resp, err := gw.UpdateVA(&dokugo.UpdateVARequest{
			PartnerServiceID: partnerServiceID,
			CustomerNo:       customerNo,
			VirtualAccountNo: virtualAccountNo,
			TrxID:            trxID,
		})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK("updated")

	case "check-status":
		logBanner("Check Status")
		customerNo := arg(2, "")
		virtualAccountNo := arg(3, dokugo.ZeroPadPartnerServiceID(partnerServiceID)+customerNo)
		resp, err := gw.CheckStatus(&dokugo.CheckStatusRequest{
			PartnerServiceID: partnerServiceID,
			CustomerNo:       customerNo,
			VirtualAccountNo: virtualAccountNo,
			AdditionalInfo:   map[string]interface{}{},
		})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK(fmt.Sprintf("paidAmount=%s %s", resp.VirtualAccountData.PaidAmount.Value, resp.VirtualAccountData.PaidAmount.Currency))

	case "account-inquiry":
		logBanner("Account Inquiry")
		beneficiaryAccountNumber := arg(2, "")
		beneficiaryBankCode := arg(3, "")
		amount := arg(4, "100000.00")
		refNo := fmt.Sprintf("REF-%d", time.Now().Unix())
		resp, err := gw.AccountInquiry(&dokugo.AccountInquiryRequest{
			PartnerReferenceNo:       refNo,
			CustomerNumber:           props["DOKU_CUSTOMER_NUMBER"],
			BeneficiaryAccountNumber: beneficiaryAccountNumber,
			Amount:                   dokugo.Amount{Value: amount, Currency: "IDR"},
			AdditionalInfo:           dokugo.AccountInquiryAdditionalInfo{BeneficiaryBankCode: beneficiaryBankCode, SenderCountryCode: "ID", BeneficiaryAccountName: arg(5, "Example Customer")},
		})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK(fmt.Sprintf("beneficiaryAccountName=%s sessionId=%s", resp.BeneficiaryAccountName, resp.SessionID))

	case "transfer-bank":
		logBanner("Transfer Bank")
		beneficiaryAccountNumber := arg(2, "")
		beneficiaryBankCode := arg(3, "")
		sessionID := arg(4, "")
		amount := arg(5, "100000.00")
		refNo := fmt.Sprintf("PAYOUT-%d", time.Now().Unix())
		resp, err := gw.TransferBank(&dokugo.TransferBankRequest{
			PartnerReferenceNo:       refNo,
			CustomerNumber:           props["DOKU_CUSTOMER_NUMBER"],
			BeneficiaryAccountNumber: beneficiaryAccountNumber,
			BeneficiaryBankCode:      beneficiaryBankCode,
			Amount:                   dokugo.Amount{Value: amount, Currency: "IDR"},
			SessionID:                sessionID,
			AdditionalInfo: dokugo.TransferBankAdditionalInfo{
				BeneficiaryFirstName:   "Example",
				BeneficiaryLastName:    "Customer",
				BeneficiaryPhoneNumber: "0812345678",
				BeneficiaryAccountName: "Example Customer",
				SenderCountryCode:      "ID",
				SenderFirstName:        "Grosenia",
				SenderLastName:         "Niaga",
				SenderPersonalID:       "1234567890123456",
				SenderPersonalIDType:   "KTP",
			},
		})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK(fmt.Sprintf("referenceNo=%s partnerReferenceNo=%s", resp.ReferenceNo, resp.PartnerReferenceNo))

	case "balance-inquiry":
		logBanner("Balance Inquiry")
		accountNo := arg(2, "")
		resp, err := gw.BalanceInquiry(&dokugo.BalanceInquiryRequest{AccountNo: accountNo})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.ErrorStatus {
			logFail(fmt.Sprintf("%s: %s", resp.ResponseCode, resp.ResponseMessage))
			return
		}
		logOK(fmt.Sprintf("name=%s avaliableBalance=%s %s", resp.Name, resp.AccountInfos.AvaliableBalance.Value, resp.AccountInfos.AvaliableBalance.Currency))

	case "kirimdoku-ping":
		logBanner("KIRIMDOKU Legacy Ping")
		kc := mustKirimDokuLegacyClient(props)
		resp, err := kc.Ping()
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.Status != 0 {
			logFail(fmt.Sprintf("status=%d message=%s", resp.Status, resp.Message))
			return
		}
		logOK(resp.Message)

	case "kirimdoku-check-balance":
		logBanner("KIRIMDOKU Legacy Check Balance")
		kc := mustKirimDokuLegacyClient(props)
		resp, err := kc.CheckBalance()
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.Status != 0 {
			logFail(fmt.Sprintf("status=%d message=%s", resp.Status, resp.Message))
			return
		}
		logOK(fmt.Sprintf("corporateName=%s creditLastBalance=%.2f", resp.Balance.CorporateName, resp.Balance.CreditLastBalance))

	case "kirimdoku-inquiry":
		logBanner("KIRIMDOKU Legacy Cash-In Inquiry")
		beneficiaryAccountNumber := arg(2, "")
		bankID := arg(3, "")
		amount := arg(4, "50000")
		kc := mustKirimDokuLegacyClient(props)
		resp, err := kc.CashInInquiry(&dokugo.KirimDokuLegacyInquiryRequest{
			Channel:             dokugo.KirimDokuLegacyChannel{Code: dokugo.KirimDokuLegacyChannelBank},
			SenderCountry:       dokugo.KirimDokuLegacyCountry{Code: "ID"},
			SenderCurrency:      dokugo.KirimDokuLegacyCurrency{Code: "IDR"},
			SenderAmount:        amount,
			BeneficiaryCountry:  dokugo.KirimDokuLegacyCountry{Code: "ID"},
			BeneficiaryCurrency: dokugo.KirimDokuLegacyCurrency{Code: "IDR"},
			BeneficiaryAccount: &dokugo.KirimDokuLegacyBeneficiaryAccount{
				Number: beneficiaryAccountNumber,
				Bank:   dokugo.KirimDokuLegacyBank{ID: bankID},
			},
		})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.Status != 0 {
			logFail(fmt.Sprintf("status=%d message=%s", resp.Status, resp.Message))
			return
		}
		logOK(fmt.Sprintf("idToken=%s beneficiaryName=%s fee=%.2f %s", resp.Inquiry.IDToken, resp.Inquiry.BeneficiaryAccount.Name, resp.Inquiry.Fund.Fees.Total, resp.Inquiry.Fund.Fees.Currency))

	case "kirimdoku-remit":
		logBanner("KIRIMDOKU Legacy Cash-In Remit")
		inquiryIDToken := arg(2, "")
		beneficiaryAccountNumber := arg(3, "")
		bankID := arg(4, "")
		beneficiaryName := arg(5, "Example Beneficiary")
		kc := mustKirimDokuLegacyClient(props)
		resp, err := kc.CashInRemit(&dokugo.KirimDokuLegacyRemitRequest{
			Channel:        dokugo.KirimDokuLegacyChannel{Code: dokugo.KirimDokuLegacyChannelBank},
			InquiryIDToken: inquiryIDToken,
			SendTrxID:      fmt.Sprintf("SEND-%d", time.Now().Unix()),
			Sender: dokugo.KirimDokuLegacySender{
				Name: "Grosenia Niaga",
				KirimDokuLegacyPersonalID: dokugo.KirimDokuLegacyPersonalID{
					Type:    "KTP",
					ID:      "1234567890123456",
					Country: "ID",
				},
			},
			Beneficiary: dokugo.KirimDokuLegacyBeneficiary{Name: beneficiaryName, Country: "ID"},
			BeneficiaryAccount: &dokugo.KirimDokuLegacyBeneficiaryAccount{
				Number: beneficiaryAccountNumber,
				Name:   beneficiaryName,
				Bank:   dokugo.KirimDokuLegacyBank{ID: bankID},
			},
		})
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.Status != 0 {
			logFail(fmt.Sprintf("status=%d message=%s warning=%s errors=%v", resp.Status, resp.Message, resp.Warning, resp.Errors))
			return
		}
		logOK(fmt.Sprintf("transactionId=%s transactionStatus=%s responseCode=%s", resp.Remit.TransactionID, resp.Remit.TransactionStatus, resp.Remit.PaymentData.ResponseCode))

	case "kirimdoku-transaction-info":
		logBanner("KIRIMDOKU Legacy Transaction Info")
		transactionID := arg(2, "")
		kc := mustKirimDokuLegacyClient(props)
		resp, err := kc.TransactionInfo(transactionID)
		if err != nil {
			logFail(err.Error())
			return
		}
		if resp.Status != 0 {
			logFail(fmt.Sprintf("status=%d message=%s", resp.Status, resp.Message))
			return
		}
		logOK(fmt.Sprintf("id=%s status=%s statusMessage=%s", resp.Transaction.ID, resp.Transaction.Status, resp.Transaction.TransactionLog.StatusMessage))

	default:
		fmt.Println("unknown command:", command)
		os.Exit(1)
	}
}
