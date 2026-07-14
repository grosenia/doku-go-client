# doku-go-client

Go client for DOKU's SNAP (Standar Nasional Open API Pembayaran) payment API, used by `core-api`
via a `replace` directive during development (see root `core-api/go.mod`).

## Conventions (mirrors `xendit-go-client/v3/`)

- `Client` + `Gateway` pattern: `Client` holds credentials/HTTP config, `Gateway` wraps a `Client` and
  exposes one method per API operation. Construct via `NewClient(...)` then `NewGateway(&client)`.
- `Client.BaseURL` is a plain overridable string (real sandbox/production hosts differ for DOKU,
  unlike Xendit) — always keep it overridable for `httptest` mocking, never hardcode into request builders.
- Every response type embeds `ErrorResponse` anonymously; check `resp.ErrorStatus` after every call,
  don't assume a nil `error` means success — DOKU returns structured error bodies on 4xx/5xx, and only
  transport-level failures surface as a Go `error`.
- Two signature schemes, do not confuse them:
  - **Asymmetric** (RSA, SHA256withRSA) — ONLY for the access-token endpoint. String-to-sign uses UTC
    timestamp with NO offset.
  - **Symmetric** (HMAC-SHA512, base64 output) — every transactional call AND inbound webhook
    verification. String-to-sign uses local timestamp WITH offset. When verifying an INBOUND webhook,
    sign over *our own* notification path, not DOKU's.
- Access tokens expire in 900s — always go through `Client.getAccessToken()` (cached), never call the
  token endpoint directly per-request.
- Static vs Non-static (single-use) VA is ONE endpoint (`CreateVA`) with one field:
  `additionalInfo.virtualAccountConfig.reusableStatus`. `true` = Static (reusable), omitted/`false` =
  single-use (DOKU's default). Use `NewStaticVARequest`/`NewSingleUseVARequest` constructors rather than
  setting the flag by hand at call sites.
- **DGPC's `customerNo` is just the merchant-assigned prefix digit (e.g. "9" for BCA), NOT a long
  number you invent.** DOKU appends the rest of the payment code itself ("D" = DOKU-**generated**
  Payment Code) to reach the required total VA length (16 digits for BCA). Confirmed against real
  sandbox 2026-07-10: a made-up long `customerNo` (e.g. `"90000000"`) produced
  `4032701: Feature Not Allowed [Identifier for BIN ... not configured properly]` on every bank —
  looked exactly like an account-provisioning error, wasted a support thread — while
  `customerNo = "9"` alone succeeds immediately. Per-bank prefix comes from DOKU's dashboard "BIN
  Rules" page ("Prefix Customer No" column), not something to guess.
- **`CheckStatus`'s `virtualAccountNo` for a DGPC-created VA is the short literal
  `partnerServiceId + customerNo` (e.g. `"   19008" + "9"`) — NOT the full DOKU-generated number**
  from `CreateVAResponseData.VirtualAccountNo` (that's only for display/payment). Confirmed 2026-07-11.
  **`DeleteVA`'s requirement is still unconfirmed** — neither the short combo (`4043119 Invalid
  Bill/Virtual Account`) nor the full generated number (`4033115 ... virtualAccountNo should be
  equal to partnerServiceId + customerNo`) succeeds. Flagged in `integration_test.go`, not solved.

## VA Direct Inquiry (DIPC) — Token URL / Inquiry URL support

Added 2026-07-14 for the merchant-hosted side of DIPC (see `core-api`'s
`docs/DOKU_CHECKOUT_FLOW_DESIGN.md` §2.2 step 3 for the full design). Unlike everything else in this
package (where we call DOKU), these calls go the other way — DOKU calls **us**:

- `token_url.go`: `ParseRSAPublicKeyPEM` + `VerifyTokenURLRequestSignature` verify DOKU's inbound
  "Get Token" request using the dashboard's "DOKU Public Key" (asymmetric, same formula as the B2B
  Get Token endpoint's `signAsymmetric`, roles reversed). `TokenURLResponse`/`NewTokenURLResponse`
  build the response body — confirmed by DOKU support to mirror the B2B Get Token API's own response
  shape.
- `inquiry.go`: `InquiryRequest`/`InquiryResponse` + `NewInquiryResponseSuccess`/
  `NewInquiryResponseNotFound` for DOKU's inbound Direct Inquiry call. Signature verification reuses
  the existing `VerifyWebhookSignature` (same symmetric HMAC scheme as the payment notification
  webhook) — no new verify function needed for this half.
- `auth.go`: added `verifyAsymmetric` — the missing inverse of `signAsymmetric`, needed because this
  package only ever signed requests before (we were always the client); now it also verifies one
  (DOKU calling us).

## Testing

- `go test .` — unit + httptest mock-server scenario tests, no network.
- `go test -tags=integration -run Integration .` — hits DOKU sandbox for real; requires
  `DOKU_CLIENT_ID`, `DOKU_SECRET_KEY`, `DOKU_PRIVATE_KEY_PATH` env vars, skips if unset.
- `./run-tests.sh` — runs both stages plus the `examples/` CLI smoke tests.
- `cd examples && go run . <command>` — manual exploration against sandbox, reads `config.props`
  (gitignored; copy from `config-sample.props`).

## Disbursement (Kirim DOKU)

Pays FROM Grosenia TO a bank account (e.g. seller payout) — the reverse direction of Virtual
Account, which collects FROM a buyer TO Grosenia. Same SNAP auth scheme (symmetric HMAC per call,
asymmetric only for the token endpoint), different path family (`/snap/v1.1/emoney/...` and
`/snap/v1.1/balance-inquiry`, vs VA's `/virtual-accounts/bi-snap-va/v1.1/...`).

Flow: **Account Inquiry** (validate the destination account, get a `sessionId`) → **Balance
Inquiry** (check Grosenia's own DOKU balance, optional/informational) → **Transfer Bank** (moves
the money, `sessionId` from Account Inquiry required) → **Check Status** (poll for completion).

`CheckDisbursementStatus`'s endpoint path is a **guess**, not confirmed — DOKU's own docs page for
it has a genuine documentation bug (titled "KIRIMDOKU Check Status" but the embedded OpenAPI spec
is actually the QR `qr-mpm-status` endpoint). Don't trust it in production without confirming with
DOKU directly. See `docs/FEATURES.md`/`docs/CHECKLIST.md` for the full list of disbursement gaps
(including the two webhook types that aren't implemented at all — their schemas aren't extractable
from DOKU's public docs).

## Adding a new bank

Add an entry to `BankChannels` in `constants.go` — no other code changes needed for VA creation.
Confirm the bank's exact `partnerServiceId` format/length with DOKU before adding (this varies per
merchant contract, not a fixed public constant).

## Known documentation gaps (confirmed against DOKU's public docs, not yet resolved)

- No literal BCA example for the Check Status response (only other banks shown in docs) — same
  endpoint/schema assumed bank-agnostic, unconfirmed for BCA specifically.
- DOKU's own OpenAPI spec for Update VA shows `virtualAccountConfig.status` enum as `ACTIVE`/`INACTIVE`
  but the accompanying example value is `"DELETED"` — treat as a free-form string, don't assume a fixed enum.
- No documented refund-VA endpoint.
