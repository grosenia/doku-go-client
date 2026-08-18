# KIRIMDOKU API (legacy, non-SNAP) — reference only

**⚠️ This is NOT the API this package implements.** `disbursement.go`/`urls.go` in this repo call
DOKU's **SNAP-compliant** KirimDOKU endpoints (`/snap/v1.1/emoney/bank-account-inquiry`,
`/snap/v1.1/emoney/transfer-bank`, `/snap/v1.1/balance-inquiry`, `/snap/v1.1/transfer/status`),
authenticated with SNAP's Bearer-token + `X-TIMESTAMP`/`X-SIGNATURE` (HMAC-SHA512) scheme — the
same convention as every other endpoint in this package, and the whole reason for this session's
ASPI Devsite functional testing (SNAP compliance is a DOKU/ASPI prerequisite before KirimDOKU gets
activated for Grosenia's merchant account, see `core-api`'s `docs/DOKU_CHECKLIST.md`).

This document describes a **different, older KirimDOKU API version** — auth via a fixed `agentKey`
+ AES-encrypted `signature` (not SNAP), base path `/apikirimdoku` (not `/snap/v1.1/...`). Captured
2026-08-14 from `https://staging.doku.com/kirimdoku/v2/apiDoc` (login-gated, pasted by the user)
purely as reference material — e.g. its bank code list, and its "Transaction Info" endpoint is a
useful cross-reference for what a status-check response conceptually looks like, even though the
endpoint itself isn't callable from this package. **Do not use these endpoints, this auth scheme, or
these field names when working on this package's actual SNAP implementation** — they're unrelated
systems that happen to share the "KirimDOKU" product name.

## Introduction

KIRIMDOKU API supports three transfer methods:

- **Bank Transfer**: destination is a beneficiary bank account. Partner must validate the account
  via inquiry first.
- **DOKU Wallet Transfer**: destination is a beneficiary DOKU wallet. Partner must validate via
  inquiry first.
- **Cash-to-Cash Transfer**: sender sends cash, beneficiary receives cash after entering the same
  OTP as the sender's transaction.

## Getting Started

To integrate, partner needs an `agentKey`. DOKU requires at minimum 3 registered email credentials:

- **Main Agent**: supervises all transactions, gets API doc access.
- **Supervisors**: supervises transactions/operator accounts, can batch-upload transfers.
- **Operator**: performs transactions, sees insight reports.

Partner must also prepare callback URLs for testing and production.

## Hosts

- Staging: `https://staging.doku.com/apikirimdoku`
- Production: `https://kirimdoku.com/v2/api`

DOKU calls out from only 2 known public IPs (for Identify/Notify callbacks) — filtering by DOKU's
IP reduces (doesn't fully prevent) spoofed callback traffic.

## Auth: SharedKey Hash Value (AES signature)

Every request needs `agentKey`, `requestId` (partner-generated, unique per request, max 36
alphanumeric, e.g. a UUID), and `signature` — an AES-encrypted hash proving the request is genuine.

**Signature generation** (Java example given in the docs):

```java
private static final String ALGORITHM = "AES";

public static String encrypt(String valueToEnc, String encKey) throws Exception {
    Key key = new SecretKeySpec(encKey.getBytes(), ALGORITHM);
    Cipher c = Cipher.getInstance(ALGORITHM);
    c.init(Cipher.ENCRYPT_MODE, key);
    byte[] encValue = c.doFinal(valueToEnc.getBytes());
    return new String(Base64.encodeBase64(encValue));
}

// concatenatedString = agentKey + requestId, e.g. "A47438" + "4201243b-c0a3-495f-8464-bad5f4a2e9d4"
// encryptionKey = 16-char key issued by KIRIMDOKU
String correctSignature = encrypt(concatenatedString, encryptionKey);
```

Steps: (1) concatenate `agentKey + requestId` into a raw string, (2) AES-encrypt it with the
16-char key DOKU issued, (3) base64-encode the encrypted bytes → that's `signature`.

Sample credentials shown in the docs (staging, example only — not Grosenia's real ones):
`agentKey: A47438`, `encryptionKey: pl6jn16fvkb64fit`.

## Endpoints

### `POST /ping`
Check connection availability. Body: `{agentKey, requestId, signature}`. Success:
`{"status": 0, "message": "Ok"}`.

### `POST /cashin/inquiry`
Validate beneficiary account, get applied fee, before remitting. Two channels:

- `channel.code: "07"` — Bank Deposit. Needs `beneficiaryAccount.bank.{code,countryCode,id,name}`,
  `.number`, `.name`, `.city`, `.address`.
- `channel.code: "04"` — DOKU Wallet. Needs `beneficiaryWalletId` (email or DOKU Wallet ID) instead
  of bank fields.

Common fields: `senderCountry.code`, `senderCurrency.code`, `beneficiaryCountry.code`,
`beneficiaryCurrency.code`, `senderAmount` (numeric), `senderNote` (optional).

Success response includes `inquiry.idToken` (needed for the follow-up `remit` call), `fund.origin`/
`fund.destination`/`fund.fees`, and full resolved `beneficiaryAccount` detail (bank name, city,
etc — DOKU fills these in from its own bank master data given just the bank ID).

Failure examples: `status: 11` (invalid/missing params), `status: 1` ("Transfer Inquiry Decline"),
`status: 16` ("No forex found for destination country").

### `POST /cashin/remit`
Executes the actual transfer, using the `inquiry.idToken` from a prior `cashin/inquiry` call.
Requires much more detail than the inquiry call: full `sender` (name, phone, birthDate, gender,
address, `personalIdType`/`personalId`/`personalIdCountry`) and `beneficiary` (name, phone,
country, address) blocks, plus `beneficiaryAccount` (bank details for channel `07`, or
`beneficiaryWalletId` for channel `04`). Also accepts an optional `sendTrxId` (partner's own unique
key per request).

Success: `status: 0`, `remit.paymentData.responseCode: "00"` ("Transfer Approve"),
`remit.transactionId` (DOKU's own reference, e.g. `"DK0252106"`),
`remit.transactionStatus` (numeric, see Status Transaction table below).

Also handles: re-processing an already-used `inquiryId` (`"warning": "Inquiry Process has been
processed"`), invalid params (`status: 11`), amount mismatch vs the inquiry (`status: 13`,
`"Invalid Amount"`), and bank/switching decline (`remit.paymentData.responseCode: "25"`,
`"Transfer Decline"`, `transactionStatus: "35"`).

### `POST /checkbalance`
Check partner's own KIRIMDOKU deposit balance. Body: `{agentKey, requestId, signature}` (no other
fields). Success: `balance.{corporateName, creditLimit, creditAlertLimit, creditLastBalance}`.

### `POST /transaction/info`
**Get transaction status** — the legacy-API equivalent of what this package's
`CheckDisbursementStatus` does for the SNAP API. Body: `{"transactionId": "..."}` — accepts either
DOKU's own transaction ID (e.g. `"DK0826666"`) or the original inquiry's `idToken` (e.g.
`"I02852516927268"`).

Response is a large nested object: `transaction.{id, inquiryId, sendTrxId, status, createdTime,
cashInTime, agent, channel, sender, beneficiary, fund, transactionlog}`. `transactionlog` carries
the human-readable status/reference codes (`status`, `statusMessage`, `referenceStatus`,
`referenceMessage`, `channelCode`).

Failure: `status: 9` (unauthorized), `status: 11` ("Transaction id is invalid").

### Refund Notification (inbound webhook, DOKU → partner)
DOKU POSTs to a partner-registered callback URL when a refund occurs:
`{"activityCode": "200", "transactionId": "...", "processDate": <epoch ms>, "message": "Transaction is refunded"}`.
Retried up to 3 times until the partner responds correctly. Partner must respond:
`{"status": "true"|"false", "transactionId": "...", "responseCode": "00"|"01"|other, "responseMessage": "..."}`
(`"00"` = processed successfully, `"01"` = already processed before).

## API-wide status codes

| Status | Meaning | Example |
|---|---|---|
| `-1` | Default error | Internal engine error / temporary unavailable |
| `0` | Success | OK / Success Queued |
| `9` | Bad request headers | Unauthorized access |
| `1` | Missing required field | `agentKey` required, `inquiry.IdToken` required |
| `2` | Invalid param type | `requestId` must be alphanumeric, `sender.amount` invalid numeric |
| `11` | Invalid param value | Invalid `agentId`, invalid `status` enum value |
| `12` | Illegal param | Illegal character in `password`, illegal reference code |
| `13` | Security issue | `signature`/`agentKey` denied |
| `16` | Unsupported operation | Partner not allowed to perform this |
| `21` | Not found | `transactionId` not found |
| `91` | Access blocked | Permanently blocked |

## Transaction status codes (`transaction.status` / `remit.transactionStatus`)

| Status | Meaning |
|---|---|
| `0` | Unknown |
| `10` | Cash transfer transaction created |
| `20` | Bank/Wallet unpaid transaction |
| `30` | Cash Transfer transaction locked |
| `35` | Transfer failed to be processed |
| `40` | Refunded by KIRIMDOKU |
| `41` | Cash transfer transaction fully refunded |
| `50` | Successfully performed by KIRIMDOKU |
| `64` | Timeout by interswitch bank |

## Supported banks

KIRIMDOKU supports 70+ Indonesian banks. Full `{Bank ID, Bank Name, SWIFT Code}` list captured
2026-08-14 — see the raw pasted content in this session's history if needed; not reproduced here
in full since it's large and easy to re-fetch from DOKU's docs if this drifts out of date. A few
notables: `014` = Bank Central Asia BCA (`CENAIDJA`), `002` = Bank BRI (`BRINIDJA`), `008` = Bank
Mandiri (`BMRIIDJA`), `009` = Bank BNI (`BNINIDJA`), `899` = DOKU itself (`899`) — plus e-wallet
"banks" (`GOPAY`, `OVO`, `DANA`, `SHOPEEPAY`, `LinkAja`/`911`) and marketplace VA codes
(`PVABUKA` Bukalapak VA, `PVASHOP` Shopee VA, `PVATOKO` Tokopedia VA).

Document version: 2.0 (per DOKU's own footer).
