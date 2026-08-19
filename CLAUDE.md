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
  `partnerServiceId + customerNo` (e.g. `"   19008" + "9"`, using the SHORT `customerNo` WE sent) —
  NOT the full DOKU-generated number** from `CreateVAResponseData.VirtualAccountNo` (that's only for
  display/payment). Confirmed 2026-07-11.
- **`UpdateVA`/`DeleteVA` need a DIFFERENT `CustomerNo`/`VirtualAccountNo` convention than
  `CheckStatus`** — confirmed against real DOKU sandbox 2026-08-19, resolving a long-standing
  contradiction (previously both "short" and "full" forms failed, with two different error codes,
  because the actual fix needed BOTH corrections at once, not either alone):
  1. Use the **DOKU-GENERATED `CustomerNo`** from `CreateVAResponseData.CustomerNo` (e.g.
     `"90000161862"`, not the short `"9"` we originally sent) — with `VirtualAccountNo` set to
     `CreateVAResponseData.VirtualAccountNo` (the full number, e.g. `"   1900890000161862"`).
     Using the short `CustomerNo` here produces DOKU's confusing
     `4033115 ... virtualAccountNo should be equal to partnerServiceId + customerNo` — which is
     DOKU telling you the customerNo you're combining it with is wrong, not the virtualAccountNo.
  2. Set `AdditionalInfo.Channel` (e.g. `"VIRTUAL_ACCOUNT_BCA"`, matching
     `CreateVAResponseData.AdditionalInfo.Channel`) — omitting it fails with
     `400280x Invalid Mandatory Field {additionalInfo.channel}` on both Update and Delete.
  3. **`DeleteVA` additionally requires `TrxID` to match the ORIGINAL `CreateVA` call's `TrxID`
     exactly** — a fresh/different `TrxID` fails with `4043119 Invalid Bill/Virtual Account` even
     with the correct `CustomerNo`/`VirtualAccountNo`/`Channel` from steps 1–2. `UpdateVA` does NOT
     have this requirement (a fresh `TrxID` works fine there).
  With all of the above, `UpdateVA` → `2002800 Successful` and `DeleteVA` → `2003100 Successful`,
  confirmed directly against `api-sandbox.doku.com` (not just ASPI's simulator). Order does not
  matter otherwise — Delete works immediately after Create, no Update step required first.

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

`CheckDisbursementStatus`'s endpoint path (`/snap/v1.1/transfer/status`, no "emoney" segment —
different from the naming convention of the other 3 disbursement endpoints above) and full
request/response shape were confirmed 2026-08-14 against ASPI Devsite's sandbox (ASPI's own portal
has the exact same documentation bug DOKU's does — its "Transfer Status Inquiry" menu item also
serves the QR `qr-mpm-status` spec instead; the real endpoint was found on a *different* menu item,
"Transaction Status Inquiry Bank" under Transfer Kredit → PJP AIS Bank, which turns out to be a
generic status-check keyed by `serviceCode` — `"53"` = disbursement, see
`DisbursementCheckStatusServiceCode`). Real response captured:
`2003600 Successful`/`latestTransactionStatus: "00"`/`transactionStatusDesc: "success"` for a
genuine prior `TransferBank` call.

**Update 2026-08-18 — `pathDisbursementCheckStatus` confirmed WRONG against real DOKU.** Tested
directly against `api-sandbox.doku.com` (not ASPI) using a real successful `TransferBank`
transaction: `/snap/v1.1/transfer/status` returns `404 No static resource`. Also tried
`/orders/v1.0/transfer/status`, `/snap/v1.1/emoney/transfer-status`, `/snap/v1.1/transfer-status`,
`/snap/v1.1/emoney/transfer/status`, `/orders/v1.0/emoney/transfer-status` — all 404. The
`api/v1.0` ↔ `snap/v1.1` host-substitution pattern that correctly predicted every other
disbursement endpoint's real path does NOT hold for this one. The real path is unknown — do not
guess further, ask DOKU directly. `AccountInquiry`/`TransferBank`/`BalanceInquiry`'s paths are
unaffected (see below, `AccountInquiry`/`TransferBank` independently confirmed the same day).

**Also confirmed 2026-08-18, first direct (non-ASPI) test of the other 3 disbursement endpoints**
against `api-sandbox.doku.com`:
- `AccountInquiry`: works, but real DOKU requires `additionalInfo.senderCountryCode` and
  `additionalInfo.beneficiaryAccountName` — both tagged `omitempty` in this package (matching the
  docs, which call them optional) but rejected as `4004202 Invalid Mandatory Field` if empty on
  real DOKU (ASPI's simulator let them through blank). Always set both.
- `TransferBank`: works — a real transfer executed successfully
  (`2004300 Successful`/`referenceNo DK02195017`). Found and fixed a real bug in the process:
  `TransferBankResponseAdditionalInfo.Amount` was typed as the nested `Amount{Value,Currency}`
  struct like every other amount field in this package, but DOKU actually returns it as a **plain
  decimal string** (`"50000.00"`) — this broke `json.Unmarshal`, which made a successful transfer
  look like a failed request (confirmed via `4094302 Transaction Has Been Processed` on retrying
  the same `sessionId`). Now typed as `string`.
- `BalanceInquiry`: fails with `4041108 Invalid Merchant` when tried with a placeholder
  `accountNo` — Grosenia's real KirimDOKU balance account number is unknown, not guessable (same
  category of mistake as the DGPC `customerNo` incident above), needs asking DOKU.

See `docs/FEATURES.md`/`docs/CHECKLIST.md` for the full list of remaining disbursement gaps
(including the two webhook types that aren't implemented at all — their schemas aren't extractable
from DOKU's public docs).

## KIRIMDOKU legacy (non-SNAP) API — separate system, separate client

Added 2026-08-14 after DOKU support activated a live legacy sandbox merchant and pointed at it
directly (no mention of SNAP/ASPI) — see `docs/KIRIMDOKU_LEGACY_API_REFERENCE.md` for the full
captured docs. This is a **completely different API** from everything else in this package: base
path `/apikirimdoku` (staging: `staging.doku.com`, prod: `kirimdoku.com/v2/api`) instead of
`/snap/v1.1/...`, and auth via a fixed `agentKey` + AES-128/ECB/PKCS7 `signature` instead of SNAP's
Bearer-token + HMAC/RSA scheme — so it gets its own client type, `KirimDokuLegacyClient`
(`kirimdoku_legacy.go`/`types_kirimdoku_legacy.go`), not `Client`/`Gateway`. Do not merge the two.

- Auth field placement is inconsistent per-endpoint (confirmed by testing both against real DOKU
  staging): `Ping`/`CheckBalance` put `agentKey`/`requestId`/`signature` in the **JSON body**
  (`doRequestBodyAuth`); `CashInInquiry`/`CashInRemit`/`TransactionInfo` put them as **HTTP
  headers** (`doRequestHeaderAuth`) — `TransactionInfo`'s header placement specifically is inferred
  from the same pattern, not independently confirmed.
  - **Confirmed live 2026-08-14** against real staging (`agentKey A47438` /
    `encryptionKey pl6jn16fvkb64fit`): `Ping` → `{"status":0,"message":"Ok"}`; `CashInInquiry`
    (channel `07` Bank, BCA account) → real resolved account holder name, proving genuine sandbox
    connectivity (not a canned response).
  - **`CashInInquiry`'s response is significantly richer than the docs summarize** — `fund` and
    `beneficiaryAccount` nest INSIDE `inquiry` (not top-level as the docs' prose implies), `fund.fees`
    is a nested object (`total`/`currency`/`components`/`additionalFee`/`fixedFee`), and
    `beneficiaryAccount` includes DOKU's own resolved bank master data. `CheckBalance`'s numeric
    fields (`creditLimit` etc.) are JSON numbers, not strings. Both confirmed 2026-08-14.
  - **`CashInRemit` blocked, unresolved 2026-08-14**: real staging consistently rejects with
    `status: 11` (`errors: {"beneficiary.country": ["Invalid value"], "sender.personalIdCountry":
    ["Invalid value"]}`) — tried `"ID"`, `"IDN"`, `"360"` (ISO numeric), `"Indonesia"`, and an
    object form `{"code": "ID"}` (which broke differently: `"": ["Invalid parameter null"]`), none
    accepted. Also needed (undocumented) top-level `senderCountry`/`senderCurrency`/
    `beneficiaryCountry`/`beneficiaryCurrency` on the remit request, same as inquiry, before those
    two fields became the only remaining errors — so the request shape is otherwise right. Ask DOKU
    support for the accepted value/format before spending more sandbox calls guessing (same lesson
    as the DGPC `customerNo` incident above — don't guess a provisioning/format field blind).
    `TransactionInfo` is unexercised entirely (no successful remit yet to look up).
- Go's stdlib intentionally has no ECB cipher mode (it's normally insecure) — `sign()` in
  `kirimdoku_legacy.go` implements AES-ECB-PKCS7 manually, block by block, to match DOKU's Java
  sample (`Cipher.getInstance("AES")`'s bare default is ECB/PKCS5, equivalent to PKCS7 at 16-byte
  blocks).
- Not yet wired into `core-api`/`api-web` — this is standalone client code, pending DOKU's answer on
  whether production KirimDOKU should use this legacy path or the SNAP path built elsewhere in this
  package (question sent to DOKU support 2026-08-14, unanswered as of this writing).

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
