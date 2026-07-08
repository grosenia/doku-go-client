# doku-go-client

Go client for [DOKU](https://doku.com)'s SNAP (Standar Nasional Open API Pembayaran) payment API.
Covers Virtual Account (Static and Non-static/single-use, sharing one `CreateVA` endpoint
distinguished by the `reusableStatus` flag) and Disbursement/Kirim DOKU (Account Inquiry, Transfer
Bank, Balance Inquiry).

Mirrors the conventions of the sibling `xendit-go-client` library used elsewhere in this org.

## Install

```
go get github.com/grosenia/doku-go-client
```

## Quick start

```go
privateKeyPEM, _ := os.ReadFile("doku-private-key.pem")
client, err := dokugo.NewClient("BRN-0259-1678068334526", "your-secret-key", privateKeyPEM, true /* sandbox */)
if err != nil {
    log.Fatal(err)
}
gw := dokugo.NewGateway(&client)

resp, err := gw.CreateVA(dokugo.NewSingleUseVARequest(
    "bca",
    "   19008",           // partnerServiceId
    "00000000000000000001", // customerNo
    "Customer Name",
    "TRX-001",
    dokugo.Amount{Value: "150000.00", Currency: "IDR"},
))
if err != nil {
    log.Fatal(err)
}
fmt.Println(resp.VirtualAccountData.VirtualAccountNo)
```

See `CLAUDE.md` for architectural conventions, `docs/TESTING.md` for how to run tests, `docs/FEATURES.md`
for endpoint coverage, `docs/CHECKLIST.md` for release gating/known gaps, and `examples/` for a
runnable CLI against DOKU's sandbox.
