# Features

DOKU SNAP API coverage, current state.

| Feature | Endpoint | Status |
|---|---|---|
| Get Token B2B (asymmetric auth) | `POST /authorization/v1/access-token/b2b` | ✅ implemented, cached (900s expiry, 60s safety margin) |
| Create VA — Static (reusable) | `POST /virtual-accounts/bi-snap-va/v1.1/transfer-va/create-va` | ✅ `NewStaticVARequest` + `Gateway.CreateVA` |
| Create VA — Non-static (single-use) | same endpoint, `reusableStatus` omitted | ✅ `NewSingleUseVARequest` + `Gateway.CreateVA` |
| Delete VA | `DELETE /virtual-accounts/bi-snap-va/v1.1/transfer-va/delete-va` | ✅ `Gateway.DeleteVA` |
| Update VA | `PUT /virtual-accounts/bi-snap-va/v1.1/transfer-va/update-va` | ✅ `Gateway.UpdateVA` |
| Check Status | `POST /orders/v1.0/transfer-va/status` | ✅ `Gateway.CheckStatus` |
| Payment notification (webhook) | inbound, merchant-hosted | ✅ types + `VerifyWebhookSignature` + `NewPaymentNotificationAck` |
| Direct Inquiry (DOKU-initiated) | inbound, merchant-hosted | ❌ not implemented — only relevant if a VA is registered without a `CreateVA` call, which Grosenia's flow doesn't do |
| Bank coverage | `BankChannels` in `constants.go` | only `bca` seeded — add entries as other banks (BNI, Mandiri, BRI, CIMB, Permata) are confirmed against sandbox |
| Refund | — | ❌ no refund-VA endpoint documented publicly; not implemented |
| Non-VA products (cards, e-wallet, QRIS, disbursement) | — | ❌ out of scope for this library so far — Virtual Account only |

## Two auth schemes, one library

- **Asymmetric** (RSA, SHA256withRSA): only `client.go`'s `requestNewAccessToken`, via `signAsymmetric`.
- **Symmetric** (HMAC-SHA512, base64): every transactional `Gateway` method (via `gateway.go`'s `doRequest` → `signSymmetric`), and inbound webhook verification (`VerifyWebhookSignature`, same underlying function, different `endpointPath` argument).

See `CLAUDE.md` for the reasoning behind each design choice.
