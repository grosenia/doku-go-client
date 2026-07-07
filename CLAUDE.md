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

## Testing

- `go test .` — unit + httptest mock-server scenario tests, no network.
- `go test -tags=integration -run Integration .` — hits DOKU sandbox for real; requires
  `DOKU_CLIENT_ID`, `DOKU_SECRET_KEY`, `DOKU_PRIVATE_KEY_PATH` env vars, skips if unset.
- `./run-tests.sh` — runs both stages plus the `examples/` CLI smoke tests.
- `cd examples && go run . <command>` — manual exploration against sandbox, reads `config.props`
  (gitignored; copy from `config-sample.props`).

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
