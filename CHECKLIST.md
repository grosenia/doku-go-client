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

## Helper/primitive coverage

- ✅ `signAsymmetric` — round-trips against its own public key; verified different timestamps produce different signatures.
- ✅ `signSymmetric` — matches an independently-computed HMAC reference; matches DOKU's own published SHA-256 body-digest example; output confirmed base64; sensitive to every input component (secret/method/path/token/body/timestamp).
- ✅ `minifyJSON` — whitespace removed, key order and values preserved.
- ✅ `ErrorResponse`/`markHTTPError` — set correctly via `errorMarker` interface promotion, exercised by every scenario test's failure case.

## Known gaps (not blocking, tracked in `CLAUDE.md`/`FEATURES.md`)

- Only BCA is seeded in `BankChannels` — add + validate other banks against sandbox before enabling them in `core-api` config.
- `VerifyWebhookSignature`'s `accessToken` parameter role for inbound notifications is inferred from the outbound formula, not confirmed against a real DOKU-signed webhook — validate with DOKU or a captured real notification before depending on it in production.
- `NewPaymentNotificationAck`'s `ResponseCode` ("2002500") is inferred from DOKU's numbering pattern, not a confirmed literal example — verify against sandbox.

## Gate to `core-api`

All of the following must be true before switching `core-api`'s `go.mod` from a local `replace` directive to a tagged version:

1. `./run-tests.sh` passes (unit + scenario).
2. `TestIntegration_CreateStaticVA_CheckStatus_DeleteVA_RoundTrip` passes against real DOKU sandbox credentials.
3. A real DOKU sandbox VA payment notification has been captured at least once and manually verified with `VerifyWebhookSignature` (to close the gap noted above).
