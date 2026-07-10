# Release checklist

Gate before `core-api` depends on a tagged (non-`replace`) version of this library.

## Test coverage

| Method | Unit/scenario test | Integration test | Example CLI command |
|---|---|---|---|
| `CreateVA` (Static) | ✅ `TestScenario01_CreateVA_Static_Success` | ✅ `TestIntegration_CreateStaticVA_CheckStatus_DeleteVA_RoundTrip` | `create-va-static` |
| `CreateVA` (Single-use) | ✅ `TestScenario02_CreateVA_SingleUse_Success` | — (covered indirectly) | `create-va-single-use` |
| `DeleteVA` | ✅ `TestScenario03_DeleteVA_Success` / `_Fail_NotFound` | ✅ (part of round trip) | `delete-va` |
| `UpdateVA` | ✅ `TestScenario04_UpdateVA_Success` | — | `update-va` |
| `CheckStatus` | ✅ `TestScenario05_CheckStatus_Success` / `_Fail_NotPaid` | ✅ (part of round trip) | `check-status` |
| `VerifyWebhookSignature` | ✅ valid / tampered / wrong-secret | — (no live webhook sender in sandbox testing) | — |
| Token caching | ✅ reuse-within-expiry / refresh-after-expiry | implicit (every integration call) | — |
| `AccountInquiry` | ✅ `TestScenario08_AccountInquiry_Success` | — | `account-inquiry` |
| `TransferBank` | ✅ `TestScenario09_TransferBank_Success` / `TestScenario10_..._Fail_DokuError` | — | `transfer-bank` |
| `BalanceInquiry` | ✅ `TestScenario11_BalanceInquiry_Success` | — | `balance-inquiry` |
| `CheckDisbursementStatus` | ❌ no test — path unconfirmed, see gaps below | — | — |

## Helper/primitive coverage

- ✅ `signAsymmetric` — round-trips against its own public key; verified different timestamps produce different signatures.
- ✅ `signSymmetric` — matches an independently-computed HMAC reference; matches DOKU's own published SHA-256 body-digest example; output confirmed base64; sensitive to every input component (secret/method/path/token/body/timestamp).
- ✅ `minifyJSON` — whitespace removed, key order and values preserved.
- ✅ `ErrorResponse`/`markHTTPError` — set correctly via `errorMarker` interface promotion, exercised by every scenario test's failure case.

## Known gaps (not blocking, tracked in `../CLAUDE.md`/`FEATURES.md`)

- BCA/BNI/Mandiri are seeded in `BankChannels`, all three confirmed working against real sandbox
  (2026-07-10) — add + validate any further bank the same way before enabling it in `core-api` config.
- `VerifyWebhookSignature`'s `accessToken` parameter role for inbound notifications is inferred from the outbound formula, not confirmed against a real DOKU-signed webhook — validate with DOKU or a captured real notification before depending on it in production.
- `NewPaymentNotificationAck`'s `ResponseCode` ("2002500") is inferred from DOKU's numbering pattern, not a confirmed literal example — verify against sandbox.
- `CheckDisbursementStatus`'s endpoint path (`pathDisbursementCheckStatus`) is a **guess** — DOKU's own docs page for this endpoint is bugged (schema mismatch, see `FEATURES.md`). Confirm the real path with DOKU before using this method against sandbox/production.
- Disbursement `UnpaidNotification`/`RefundNotification` webhooks are not implemented — DOKU's docs reference an embedded OpenAPI schema not extractable from the public page. Needs either a captured real webhook payload or a direct answer from DOKU.
- Disbursement (`AccountInquiry`/`TransferBank`/`BalanceInquiry`) still has zero real-sandbox coverage — VA's sandbox blocker is resolved (see `CLAUDE.md`'s `customerNo` note), but disbursement itself hasn't been tried against real sandbox yet, so these are unit/scenario-tested only.

## Gate to `core-api`

All of the following must be true before switching `core-api`'s `go.mod` from a local `replace` directive to a tagged version:

1. `./run-tests.sh` passes (unit + scenario).
2. `TestIntegration_CreateStaticVA_CheckStatus_DeleteVA_RoundTrip` passes against real DOKU sandbox credentials.
3. A real DOKU sandbox VA payment notification has been captured at least once and manually verified with `VerifyWebhookSignature` (to close the gap noted above).
