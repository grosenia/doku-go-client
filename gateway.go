package dokugo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

// Gateway wraps a Client and exposes one method per DOKU SNAP operation.
type Gateway struct {
	Client *Client
}

func NewGateway(client *Client) *Gateway {
	return &Gateway{Client: client}
}

// doRequest marshals reqBody, signs the request per DOKU's SNAP symmetric
// scheme, executes it, and unmarshals the response into respBody. respBody
// must be a pointer to a struct that anonymously embeds ErrorResponse; its
// ErrorStatus field is set based on the HTTP status code. Only transport-level
// failures are returned as a Go error — a non-2xx DOKU response is signaled
// via ErrorStatus/ResponseMessage on respBody, not a Go error, matching DOKU's
// structured-error-body convention.
func (g *Gateway) doRequest(method, path string, reqBody interface{}, respBody interface{}) (int, error) {
	token, err := g.Client.getAccessToken()
	if err != nil {
		return 0, fmt.Errorf("doku: get access token: %w", err)
	}

	var bodyBytes []byte
	if reqBody != nil {
		bodyBytes, err = json.Marshal(reqBody)
		if err != nil {
			return 0, fmt.Errorf("doku: marshal request: %w", err)
		}
	} else {
		bodyBytes = []byte("{}")
	}

	xTimestamp := formatSymmetricTimestamp(time.Now())
	signature, err := signSymmetric(g.Client.SecretKey, method, path, token, bodyBytes, xTimestamp)
	if err != nil {
		return 0, fmt.Errorf("doku: sign request: %w", err)
	}

	httpReq, err := http.NewRequest(method, g.Client.BaseURL+path, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-TIMESTAMP", xTimestamp)
	httpReq.Header.Set("X-SIGNATURE", signature)
	httpReq.Header.Set("X-PARTNER-ID", g.Client.ClientID)
	httpReq.Header.Set("X-EXTERNAL-ID", generateExternalID())
	httpReq.Header.Set("CHANNEL-ID", ChannelIDHostToHost)
	httpReq.Header.Set("Authorization", "Bearer "+token)

	res, err := g.Client.httpClient().Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("doku: execute request: %w", err)
	}
	defer res.Body.Close()

	respBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, fmt.Errorf("doku: read response: %w", err)
	}

	if len(respBytes) > 0 && respBody != nil {
		if err := json.Unmarshal(respBytes, respBody); err != nil {
			return res.StatusCode, fmt.Errorf("doku: unmarshal response: %w", err)
		}
	}

	if marker, ok := respBody.(errorMarker); ok {
		marker.markHTTPError(res.StatusCode)
	}

	return res.StatusCode, nil
}

// externalIDCounter defensively disambiguates X-EXTERNAL-ID values generated
// within the same nanosecond (DOKU requires uniqueness per day, not globally).
var externalIDCounter uint32

func generateExternalID() string {
	n := atomic.AddUint32(&externalIDCounter, 1)
	return fmt.Sprintf("%d%03d", time.Now().UnixNano(), n%1000)
}
