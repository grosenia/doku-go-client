# Testing

## 1. Unit + scenario tests (no network)

```
go test ./... -v
```

Coverage:
- `auth_test.go` — signature generation (asymmetric RSA round-trip, symmetric HMAC against an independently-computed reference and against DOKU's own published SHA-256 body-digest example), timestamp formatting, JSON minification, webhook signature accept/reject/tamper.
- `va_test.go` — Create VA (Static success, Single-use success, DOKU-error response), access-token caching (reuse within expiry, refresh after expiry).
- `va_manage_test.go` — Delete VA, Update VA, Check Status (success + failure responses).
- `webhook_test.go` — payment-notification acknowledgement builder.

## 2. Integration tests (real DOKU sandbox)

```
DOKU_CLIENT_ID=... DOKU_SECRET_KEY=... DOKU_PRIVATE_KEY_PATH=./doku-private-key.pem \
  go test -tags=integration -run Integration ./...
```

Skips automatically (not fails) if the env vars aren't set. Optional: `DOKU_PARTNER_SERVICE_ID`, `DOKU_BANK` (default `bca`).

Covers: Create Static VA → Check Status → Delete VA round trip, and a deliberate-failure case (invalid `partnerServiceId`).

## 3. Manual CLI examples

```
cd examples
cp config-sample.props config.props   # fill in real sandbox credentials + private key path
go run . create-va-static
go run . create-va-single-use
go run . check-status <customerNo> <virtualAccountNo>
go run . update-va <customerNo> <virtualAccountNo>
go run . delete-va <customerNo> <virtualAccountNo>
```

## 4. Everything at once

```
./run-tests.sh                 # unit + scenario tests only
MODE=integration ./run-tests.sh   # also runs integration tests (needs env vars above)
```

## Success/fail matrix

| Scenario | Success test | Failure test |
|---|---|---|
| Create VA (Static) | `TestScenario01_CreateVA_Static_Success` | `TestScenario01_CreateVA_Fail_DokuError` |
| Create VA (Single-use) | `TestScenario02_CreateVA_SingleUse_Success` | — |
| Delete VA | `TestScenario03_DeleteVA_Success` | `TestScenario03_DeleteVA_Fail_NotFound` |
| Update VA | `TestScenario04_UpdateVA_Success` | — |
| Check Status | `TestScenario05_CheckStatus_Success` | `TestScenario05_CheckStatus_Fail_NotPaid` |
| Webhook signature | `TestVerifyWebhookSignature_AcceptsGenuineRejectsTampered` (valid case) | same test (tampered + wrong-secret cases) |
| Token caching | `TestScenario07_TokenCaching_ReusesWithinExpiry` | `TestScenario07_TokenCaching_RefreshesAfterExpiry` |
