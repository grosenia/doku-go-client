> **Status: implemented.** This is the original design plan approved before writing any code
> (2026-07-07). All phases below (0–5) were completed — see `CLAUDE.md`, `FEATURES.md`,
> `CHECKLIST.md`, and the actual source for the as-built state. Kept here as a historical record
> of the design rationale (why SNAP, why the Client/Gateway split, why two signature schemes, etc.)
> The task checkboxes below are left as originally written (unchecked) rather than retroactively
> edited.

# Plan: `doku-go-client` — DOKU SNAP payment client + core-api integration

## Context

PT Grosenia is migrating its payment gateway from Xendit to DOKU. The technical questions sent to DOKU ahead of the meeting were not answered (meeting covered business/operational topics only), **except one confirmed fact: DOKU has confirmed SNAP (not Non-SNAP) is the integration path.** Rather than wait further, the user wants to start implementation now, building against DOKU's public SNAP documentation, starting with Virtual Account (both Static and Non-static/single-use, which DOKU implements as one endpoint with a boolean flag — see below).

Decisions already made by the user:
1. **API generation: SNAP**, confirmed directly with DOKU.
2. **First feature: Virtual Account — both Static and Non-static.**
3. **Code structure: a new standalone sibling library `doku-go-client`**, mirroring the existing `xendit-go-client` library (`/Users/reyvin/Grosenia/xendit-go-client`, sibling dir to `/Users/reyvin/Grosenia/core-api`), later consumed by `core-api` the same way `xendit-go-client` is.

This plan covers: the new library's structure, the exact DOKU SNAP VA request/response contracts to implement, the core-api wiring, a `CLAUDE.md` for the new repo, and a phased task breakdown.

---

## 1. `doku-go-client` package layout

New repo at `/Users/reyvin/Grosenia/doku-go-client`, Go module `github.com/grosenia/doku-go-client`. Mirrors `xendit-go-client/v3/`'s conventions (the newer, cleaner package — not the older root `xenditgo` package, which has bugs like methods attached to the wrong receiver type).

```
doku-go-client/
  go.mod                     module github.com/grosenia/doku-go-client
  client.go                  Client struct, NewClient, token cache/refresh, signing dispatch
  auth.go                    asymmetric (RSA) + symmetric (HMAC) signature generation
  gateway.go                 Gateway struct, NewGateway, doRequest executor
  constants.go                channel codes ("VIRTUAL_ACCOUNT_BCA" etc.), CHANNEL-ID literal "H2H", trx types
  errors.go                  ErrorResponse mixin + markHTTPError
  urls.go                    URL-builder functions (Client) string
  types_request.go           CreateVARequest, DeleteVARequest, UpdateVARequest, CheckStatusRequest
  types_response.go          CreateVAResponse, DeleteVAResponse, UpdateVAResponse, CheckStatusResponse
  webhook.go                 PaymentNotificationRequest (inbound) + PaymentNotificationResponse (what we must return) + VerifyWebhookSignature(...)
  va.go                      Gateway methods: CreateVA, DeleteVA, UpdateVA, CheckStatus
  examples/
    main.go                  single CLI, subcommands: create-va, create-va-static, delete-va, update-va, check-status
    log.go                   logBanner/logStep/logOK/logFail helpers
    config-sample.props       checked in; config.props gitignored
  *_test.go                  httptest-based scenario tests (see §6)
  integration_test.go        //go:build integration, gated on DOKU_CLIENT_ID/DOKU_SECRET_KEY/DOKU_PRIVATE_KEY_PATH env vars
  README.md / FEATURES.md / CHECKLIST.md / TESTING.md
  run-tests.sh
```

### `Client` struct (`client.go`)

```go
type Client struct {
    BaseURL      string        // https://api-sandbox.doku.com or https://api.doku.com — real per-env host, unlike Xendit
    ClientID     string        // DOKU "X-PARTNER-ID" / "X-CLIENT-KEY", e.g. "BRN-0259-1678068334526"
    SecretKey    string        // client secret, used for symmetric HMAC-SHA512
    PrivateKey   *rsa.PrivateKey // parsed once from PEM at construction, used for asymmetric token signing
    HTTPClient   *http.Client

    tokenMu      sync.Mutex
    cachedToken  string
    tokenExpiry  time.Time     // set to now + expiresIn - safety margin (e.g. 60s)
}

func NewClient(clientID, secretKey string, privateKeyPEM []byte, sandbox bool) (Client, error)
```
- `NewClient` parses the RSA private key once (`x509`/`pem` parsing) and fails fast on invalid key material, rather than deferring failure to first signed request.
- `sandbox bool` picks the `BaseURL` constant (`DefaultSandboxBaseURL` / `DefaultProductionBaseURL` in `constants.go`) — a real branch, unlike Xendit's cosmetic enum, since DOKU truly has two hosts.
- `BaseURL` remains an exported, overridable field post-construction (needed for `httptest` mock-server tests, matching `xendit-go-client/v3`'s `Client.BaseURL` pattern).

### Token caching (`client.go`, method on `Client`)

```go
func (c *Client) getAccessToken() (string, error) {
    c.tokenMu.Lock()
    defer c.tokenMu.Unlock()
    if c.cachedToken != "" && time.Now().Before(c.tokenExpiry) {
        return c.cachedToken, nil
    }
    token, expiresIn, err := c.requestNewAccessToken() // POST /authorization/v1/access-token/b2b
    if err != nil { return "", err }
    c.cachedToken = token
    c.tokenExpiry = time.Now().Add(time.Duration(expiresIn)*time.Second - 60*time.Second) // 60s safety margin
    return c.cachedToken, nil
}
```
Every transactional `Gateway` method calls `c.getAccessToken()` before signing — token expiry is 900s (15 min) per DOKU docs, so this must be cached, not re-fetched per call.

### `auth.go` — the two signature schemes, as separate testable functions

```go
// Asymmetric — token endpoint only.
// stringToSign = clientID + "|" + xTimestampUTC   (xTimestampUTC format: "2006-01-02T15:04:05Z", NO offset)
func signAsymmetric(privateKey *rsa.PrivateKey, clientID, xTimestampUTC string) (string, error)

// Symmetric — every transactional call (Create/Update/Delete VA, Check Status) AND inbound webhook verification.
// stringToSign = httpMethod + ":" + endpointPath + ":" + accessToken + ":" + lowercaseHex(sha256(minifiedBody)) + ":" + xTimestamp
// signature = base64(hmacSHA512(secretKey, stringToSign))
func signSymmetric(secretKey, httpMethod, endpointPath, accessToken, minifiedBody, xTimestamp string) (string, error)

// VerifyWebhookSignature re-derives the symmetric signature over the MERCHANT's own notification path
// (not DOKU's) and compares against the inbound X-SIGNATURE header. Exported so core-api can call it directly.
func VerifyWebhookSignature(secretKey, httpMethod, notificationPath, accessToken, rawBody, xTimestamp, signatureHeader string) bool
```
Important nuance already confirmed from docs: the symmetric signature's `endpointPath` is **DOKU's path** when *we* call *them*, but the **merchant's own notification path** when verifying an **inbound** webhook. `VerifyWebhookSignature` must take the caller's own path as a parameter — never hardcode which side's path it is.

Two important implementation details to get right (confirmed from docs, easy to get wrong):
- The symmetric signature output is **base64**, not hex (some of DOKU's own OpenAPI example fields show hex-looking placeholders — those are dummy strings, not the real format; trust the worked example in the signature-component doc, which is base64).
- `X-TIMESTAMP` format differs between the token endpoint (UTC, no offset, `Z` suffix) and every transactional call (local time **with** offset, e.g. `+07:00`) — two different timestamp formatters needed, not one.

### `gateway.go` — request executor

```go
type Gateway struct { Client *Client }
func NewGateway(client *Client) *Gateway

func (g *Gateway) doRequest(method, path string, reqBody interface{}, respBody interface{}) (int, error)
```
`doRequest`: marshals `reqBody`, computes symmetric signature via `signSymmetric` (fetching/caching token first via `g.Client.getAccessToken()`), sets headers (`X-TIMESTAMP`, `X-SIGNATURE`, `X-PARTNER-ID`, `X-EXTERNAL-ID` — generated fresh per call, e.g. `time.Now().UnixNano()` string, must be unique per day per docs, not globally unique — a monotonic per-day counter or UUID truncated is safer than relying purely on nanosecond timestamp collisions), `CHANNEL-ID: H2H`, `Authorization: Bearer <token>`), executes, unmarshals into `respBody`, and calls `markHTTPError` on the embedded `ErrorResponse` mixin if status is not 2xx.

### `errors.go`

```go
type ErrorResponse struct {
    ResponseCode    string `json:"responseCode"`
    ResponseMessage string `json:"responseMessage"`
    ErrorStatus     bool   `json:"-"`
}
func (e ErrorResponse) Error() string { return fmt.Sprintf("[%s] %s", e.ResponseCode, e.ResponseMessage) }
func (e *ErrorResponse) markHTTPError(status int) {
    if e == nil { return }
    e.ErrorStatus = status < 200 || status >= 300
}
```
Every response struct anonymously embeds this (matching Xendit's `ErrorResponse` mixin pattern exactly).

### `types_request.go` / `types_response.go` — exact fields per DOKU docs

```go
type Amount struct {
    Value    string `json:"value"`    // e.g. "11500.00" — string, not float, per DOKU's exact wire format
    Currency string `json:"currency"` // "IDR"
}

type VirtualAccountConfig struct {
    ReusableStatus *bool  `json:"reusableStatus,omitempty"` // true = Static VA, false/omitted = single-use (DOKU defaults false)
    MinAmount      string `json:"minAmount,omitempty"`
    MaxAmount      string `json:"maxAmount,omitempty"`
}

type CreateVAAdditionalInfo struct {
    Channel              string                 `json:"channel"` // "VIRTUAL_ACCOUNT_BCA", "VIRTUAL_ACCOUNT_BNI", etc — see per-bank config note below
    VirtualAccountConfig *VirtualAccountConfig  `json:"virtualAccountConfig,omitempty"`
}

type FreeText struct {
    English   string `json:"english"`
    Indonesia string `json:"indonesia"`
}

type CreateVARequest struct {
    PartnerServiceID   string                  `json:"partnerServiceId"`   // 8-char, space-left-padded
    CustomerNo         string                  `json:"customerNo"`         // up to 20 digits
    VirtualAccountNo   string                  `json:"virtualAccountNo"`   // PartnerServiceID+CustomerNo, up to 28 chars
    VirtualAccountName string                  `json:"virtualAccountName"`
    VirtualAccountEmail string                 `json:"virtualAccountEmail,omitempty"`
    VirtualAccountPhone string                 `json:"virtualAccountPhone,omitempty"` // format "62xxxxxxxxxx"
    TrxID              string                  `json:"trxId"`
    TotalAmount        Amount                  `json:"totalAmount"`
    AdditionalInfo     CreateVAAdditionalInfo   `json:"additionalInfo"`
    VirtualAccountTrxType string                `json:"virtualAccountTrxType"` // "C"|"O"|"V"
    ExpiredDate        string                  `json:"expiredDate,omitempty"`  // ISO-8601, e.g. "2023-01-01T10:55:00+07:00"
    FreeText           []FreeText              `json:"freeText,omitempty"`
}

// One constructor makes the Static-vs-Non-static distinction explicit at the call site,
// rather than making callers remember to set a bool deep in a nested struct:
func NewStaticVARequest(...) *CreateVARequest   // sets ReusableStatus = ptr(true)
func NewSingleUseVARequest(...) *CreateVARequest // leaves ReusableStatus nil (DOKU defaults false)

type CreateVAResponseData struct {
    PartnerServiceID    string                 `json:"partnerServiceId"`
    CustomerNo          string                 `json:"customerNo"`
    VirtualAccountNo    string                 `json:"virtualAccountNo"`
    VirtualAccountName  string                 `json:"virtualAccountName"`
    VirtualAccountEmail string                 `json:"virtualAccountEmail,omitempty"`
    VirtualAccountPhone string                 `json:"virtualAccountPhone,omitempty"`
    TrxID               string                 `json:"trxId"`
    TotalAmount         Amount                 `json:"totalAmount"`
    VirtualAccountTrxType string               `json:"virtualAccountTrxType"`
    ExpiredDate         string                 `json:"expiredDate"`
    AdditionalInfo      struct {
        Channel      string `json:"channel"`
        HowToPayPage string `json:"howToPayPage"`
        HowToPayAPI  string `json:"howToPayApi"`
    } `json:"additionalInfo"`
}

type CreateVAResponse struct {
    VirtualAccountData CreateVAResponseData `json:"virtualAccountData"`
    ErrorResponse
}
```

`DeleteVARequest`/`Response`, `UpdateVARequest`/`Response`, `CheckStatusRequest`/`Response` follow the same field-naming conventions per the exact schemas already gathered (Delete: `partnerServiceId, customerNo, virtualAccountNo, trxId, additionalInfo`; Update: same as Create but fewer required fields, partial-update semantics; CheckStatus: hits a **different path family** `/orders/v1.0/transfer-va/status`, request `{partnerServiceId, customerNo, virtualAccountNo, inquiryRequestId, paymentRequestId, additionalInfo}`, response includes `paymentFlagReason{english,indonesia}`, `paidAmount`, `billDetails[]`).

**Per-bank channel config**: `additionalInfo.channel` must be `"VIRTUAL_ACCOUNT_<BANK>"` (`_BCA`, `_BNI`, `_MANDIRI`, `_BRI`, etc.) — mirror core-api's existing `components.xendit.fixed_va.<bank>` config-tree pattern with a new `components.doku.va.<bank>.channel` (or simpler: a Go map `var BankChannels = map[string]string{"bca": "VIRTUAL_ACCOUNT_BCA", ...}` in `constants.go`, extended as more banks are confirmed) so adding a bank is a config/const change, not new code.

### `webhook.go` — inbound notification

```go
type PaymentNotificationRequest struct {
    PartnerServiceID    string `json:"partnerServiceId"`
    CustomerNo          string `json:"customerNo"`
    VirtualAccountNo    string `json:"virtualAccountNo"`
    VirtualAccountName  string `json:"virtualAccountName"`
    VirtualAccountEmail string `json:"virtualAccountEmail,omitempty"`
    TrxID               string `json:"trxId"`
    PaymentRequestID    string `json:"paymentRequestId"`
    PaidAmount          Amount `json:"paidAmount"`
    VirtualAccountPhone string `json:"virtualAccountPhone,omitempty"`
    AdditionalInfo      struct {
        Channel               string                `json:"channel"`
        VirtualAccountConfig  *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
    } `json:"additionalInfo"`
    TrxDateTime         string `json:"trxDateTime,omitempty"`
    VirtualAccountTrxType string `json:"virtualAccountTrxType,omitempty"`
}

// What core-api's handler MUST return as the HTTP response body — DOKU's spec, not our choice.
type PaymentNotificationResponse struct {
    ResponseCode    string `json:"responseCode"`
    ResponseMessage string `json:"responseMessage"`
    VirtualAccountData struct {
        PartnerServiceID   string `json:"partnerServiceId"`
        CustomerNo         string `json:"customerNo"`
        VirtualAccountNo   string `json:"virtualAccountNo"`
        VirtualAccountName string `json:"virtualAccountName"`
        PaymentRequestID   string `json:"paymentRequestId"`
        PaidAmount         Amount `json:"paidAmount"`
        VirtualAccountTrxType string `json:"virtualAccountTrxType"`
    } `json:"virtualAccountData"`
    AdditionalInfo struct {
        Channel               string                `json:"channel"`
        VirtualAccountConfig  *VirtualAccountConfig `json:"virtualAccountConfig,omitempty"`
    } `json:"additionalInfo"`
}
```

---

## 2. `core-api` integration

Follows the exact existing Xendit wiring pattern (confirmed in `contracts/components.go`, `contracts/app.go`, `cmd/bromo-svc/util.go`, `contracts/constants.go`/`config.go`).

**New interface** — `internal/bromo-svc/contracts/components.go`:
```go
type DokuVAProvider interface {
    CreateVA(req *dokugo.CreateVARequest) (*dokugo.CreateVAResponse, error)
    DeleteVA(req *dokugo.DeleteVARequest) (*dokugo.DeleteVAResponse, error)
    UpdateVA(req *dokugo.UpdateVARequest) (*dokugo.UpdateVAResponse, error)
    CheckStatus(req *dokugo.CheckStatusRequest) (*dokugo.CheckStatusResponse, error)
}
```

**New `Components` field** — `contracts/app.go`, alongside the existing `Xendit*Gateway` fields:
```go
DokuVAGateway DokuVAProvider
```

**New init function** — `cmd/bromo-svc/util.go`, mirroring `initXenditPaymentsGateway`:
```go
func initDokuVAGateway(conf *viper.Viper) (*dokugo.Gateway, error) {
    privateKeyPEM := []byte(conf.GetString(contracts.ConfDokuPrivateKey))
    sandbox := conf.GetString(contracts.ConfDokuEnv) != "prod"
    client, err := dokugo.NewClient(
        conf.GetString(contracts.ConfDokuClientID),
        conf.GetString(contracts.ConfDokuSecretKey),
        privateKeyPEM,
        sandbox,
    )
    if err != nil { return nil, fmt.Errorf("init doku client: %w", err) }
    return dokugo.NewGateway(&client), nil
}
```
Wired into `initComponents` alongside the Xendit calls, with the same error-propagation convention used there.

**New config constants** — `contracts/constants.go`, new `// Doku` block:
```go
ConfDokuEnv           = "components.doku.env"
ConfDokuClientID      = "components.doku.client_id"
ConfDokuSecretKey     = "components.doku.secret_key"
ConfDokuPrivateKey    = "components.doku.private_key"  // PEM content or file path — recommend file path + read at init, not inline PEM in yaml
```
Add `ConfDokuEnv, ConfDokuClientID, ConfDokuSecretKey, ConfDokuPrivateKey` to `RequiredConfig` in `contracts/config.go`.

**New `config.yml` block**, mirroring `components.xendit`:
```yaml
  doku:
    env: sandbox
    client_id: ""
    secret_key: ""
    private_key_path: "keys/doku-private-key.pem"   # gitignored, never inline PEM in config.yml
    va:
      bca:
        channel: "VIRTUAL_ACCOUNT_BCA"
        partner_service_id: "   19008"   # 8-char, space left-padded per DOKU spec
```

**Webhook verification helper — the one piece with NO existing precedent to follow** (Xendit's plain-token check is duplicated 7× across 5 files with zero shared helper). Recommendation: **put `VerifyWebhookSignature` inside `doku-go-client` itself** (already specified in §1's `auth.go`), exported, so core-api's handler is a thin one-liner:
```go
ok := dokugo.VerifyWebhookSignature(secretKey, "POST", notificationPath, accessToken, rawBody, xTimestampHeader, xSignatureHeader)
```
Rationale: the signature algorithm is DOKU-specific wire-format logic, not core-api business logic — it belongs with the client library (testable in isolation there with unit tests), the same way Xendit's Basic-Auth application lives inside `xendit-go-client`, not hand-rolled per call site in `core-api`. This also means if DOKU's algorithm ever needs a fix, it's fixed once in the library, not in N call sites.

**Webhook routing** — recommend a **dedicated route** (`POST /payments/doku/va/notification` → new `handlers.PostHandleDokuVANotification`), NOT a new case in the generic `{id}` dispatcher. Rationale: the generic dispatcher's cases (Midtrans/Xendit-invoice/Xendit-disbursement) all share one incoming payload shape family (`entity.XenditNotification`-like); DOKU's inbound payload and **required response shape** are completely different JSON contracts, and forcing it through the generic switch would need a type-unsafe branch. Xendit itself already has a precedent for dedicated routes coexisting with the generic dispatcher (`/payments/xendit/credit-card/callback` is its own route) — follow that precedent, not the generic one, for a structurally different payload.

---

## 3. `CLAUDE.md` for `doku-go-client` (new repo)

See the actual `CLAUDE.md` at the repo root — implemented as designed here.

---

## 4. Testing plan

- **Unit tests, no network** (`urls_test.go`, `auth_test.go`): URL builders; `signAsymmetric`/`signSymmetric` against DOKU's own literal worked examples from the docs (known input → known output, byte-for-byte) — this is the highest-value test in the whole library, since a subtly wrong signature fails silently as "Unauthorized" with no other clue.
- **`httptest` scenario tests** (`va_test.go`), naming convention `TestScenarioNN_Feature_Success` / `_Fail_Reason`, shared `mockGateway(t, handler)` helper (copy verbatim from `xendit-go-client/v3`):
  - `TestScenario01_CreateVA_Static_Success` / `..._Fail_InvalidBank`
  - `TestScenario02_CreateVA_SingleUse_Success` (asserts `reusableStatus` omitted/false in the request the mock server receives)
  - `TestScenario03_DeleteVA_Success` / `_Fail_NotFound`
  - `TestScenario04_UpdateVA_Success`
  - `TestScenario05_CheckStatus_Success` / `_Fail_NotPaid`
  - `TestScenario06_WebhookSignatureVerification_Valid` / `_Invalid` / `_Tampered`
  - `TestScenario07_TokenCaching_ReusesWithinExpiry` / `_RefreshesAfterExpiry` (mock server counts token-endpoint hits, asserts it's called once across N gateway calls within the mocked expiry window, then again after)
- **Integration tests** (`integration_test.go`, `//go:build integration`): real sandbox create→check-status→delete round trip for both static and single-use VA. Env vars: `DOKU_CLIENT_ID`, `DOKU_SECRET_KEY`, `DOKU_PRIVATE_KEY_PATH`.
- **`examples/` CLI**: subcommands `create-va-static`, `create-va-single-use`, `delete-va`, `update-va`, `check-status`, each printing DOKU's raw response via the `logStep`/`logOK`/`logFail` banner helpers.

---

## 5. Phased task breakdown

**Phase 0 — Scaffolding**
- [ ] `go mod init github.com/grosenia/doku-go-client` at `/Users/reyvin/Grosenia/doku-go-client`
- [ ] `CLAUDE.md`, `README.md`, `.gitignore` (exclude `examples/config.props`, private key files)
- [ ] `constants.go` (base URLs, `CHANNEL-ID` literal, `BankChannels` map seeded with BCA only for now)
- **Done when**: repo exists, builds empty, `go vet` clean.

**Phase 1 — Auth (blocks everything else)**
- [ ] `auth.go`: `signAsymmetric`, `signSymmetric`
- [ ] Unit tests against DOKU's literal worked examples from docs (exact input → exact expected output)
- [ ] `client.go`: `Client` struct, `NewClient` (RSA key parsing), `getAccessToken` with caching
- **Done when**: signature unit tests pass against DOKU's own documented examples; token caching test passes (mock token endpoint hit once across repeated calls within expiry).

**Phase 2 — Create VA (Static + Non-static)**
- [ ] `types_request.go`/`types_response.go`: `CreateVARequest`/`Response`, `NewStaticVARequest`/`NewSingleUseVARequest`
- [ ] `errors.go`, `gateway.go` (`doRequest`), `va.go` (`CreateVA`)
- [ ] `httptest` scenario tests 01/02
- [ ] `examples/` CLI: `create-va-static`, `create-va-single-use`
- **Done when**: scenario tests pass; CLI successfully creates a real sandbox VA of each kind (manual run, both `reusableStatus: true` and omitted, confirmed via DOKU sandbox dashboard or Check Status).

**Phase 3 — Delete / Update / Check Status**
- [ ] Request/response types + gateway methods for all three
- [ ] Scenario tests 03/04/05
- **Done when**: scenario tests pass; manual CLI round-trip (create → check-status → update → delete) succeeds against sandbox.

**Phase 4 — Webhook verification**
- [ ] `webhook.go`: `PaymentNotificationRequest`/`Response`, `VerifyWebhookSignature`
- [ ] Scenario test 06 (valid / invalid / tampered signature)
- **Done when**: signature verification correctly accepts a genuine DOKU-shaped signed payload and rejects a tampered one, per unit test.

**Phase 5 — Publish + `core-api` wiring**
- [ ] Tag `doku-go-client` `v0.1.0` (or use local `replace` directive first, per the exact precedent in `core-api`'s own git history for `xendit-go-client`: add `replace github.com/grosenia/doku-go-client => ../doku-go-client` + placeholder `require`, `go mod tidy`, iterate, then remove the replace and bump to a tagged version once ready)
- [ ] `contracts/components.go`: `DokuVAProvider` interface
- [ ] `contracts/app.go`: `Components.DokuVAGateway` field
- [ ] `cmd/bromo-svc/util.go`: `initDokuVAGateway`, wired into `initComponents`
- [ ] `contracts/constants.go` + `config.go`: `ConfDoku*` constants, `RequiredConfig` additions
- [ ] `config.yml`: new `components.doku` block (sandbox credentials only — no production key until DOKU actually goes live)
- [ ] New dedicated route + handler for the payment-notification webhook (`cmd/bromo-svc/routes.go`, `internal/bromo-svc/handlers/`)
- **Done when**: `core-api` builds against the new gateway; a manual sandbox VA can be created through an actual `core-api` code path (even a temporary test-only service method), and a simulated webhook notification is correctly signature-verified and acknowledged.

---

## 6. Open risks / unresolved (flag, don't block on)

- Exact `partnerServiceId` value(s) Grosenia will actually be assigned per bank — this is merchant-specific, not in public docs; needs DOKU's onboarding team, not blocking library development (use a placeholder sandbox value from the docs for now).
- No documented refund-VA endpoint — if a refund flow is needed later, will need to ask DOKU directly (already on the meeting-questions list from the earlier briefing doc).
- `X-EXTERNAL-ID` uniqueness is "per day" per docs but the OpenAPI spec inconsistently types it as a bare number in one place vs "string" in the description — generate as a numeric string (e.g. zero-padded nanosecond timestamp or a per-day atomic counter) to satisfy both readings defensively.
- Only BCA has a fully worked example in DOKU's public docs; other banks (BNI, Mandiri, BRI, CIMB, Permata) are assumed structurally identical (same endpoint, different `channel` string) but not individually confirmed — validate each against sandbox before enabling in production config.

---

## Verification (end-to-end, once Phase 5 is done)

1. `cd doku-go-client && ./run-tests.sh` — all unit + scenario tests green.
2. `cd doku-go-client/examples && go run . create-va-static` against DOKU sandbox — confirm a VA number is returned and visible/payable in DOKU's sandbox tooling.
3. From `core-api` (with the `replace` directive active), call the new service method wrapping `DokuVAGateway.CreateVA` via a temporary test route or existing test harness — confirm the same round trip works through the full app wiring (config → init → gateway → DOKU sandbox).
4. Simulate a webhook POST to the new notification route with a correctly-signed payload (can reuse the `examples/` CLI's signing code or a small script) — confirm `VerifyWebhookSignature` accepts it and the handler returns DOKU's required response shape; then repeat with a tampered payload and confirm rejection.
