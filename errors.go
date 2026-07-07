package dokugo

import "fmt"

// ErrorResponse is embedded anonymously in every response type. ErrorStatus is
// set by the gateway after every HTTP call based on the response status code;
// a nil Go error does not by itself mean success — always check ErrorStatus.
type ErrorResponse struct {
	ResponseCode    string `json:"responseCode"`
	ResponseMessage string `json:"responseMessage"`
	ErrorStatus     bool   `json:"-"`
}

func (e ErrorResponse) Error() string {
	return fmt.Sprintf("[%s] %s", e.ResponseCode, e.ResponseMessage)
}

func (e *ErrorResponse) markHTTPError(status int) {
	if e == nil {
		return
	}
	e.ErrorStatus = status < 200 || status >= 300
}

// errorMarker lets the gateway set ErrorStatus on any response type without
// knowing its concrete type — satisfied automatically by every response type
// that anonymously embeds ErrorResponse.
type errorMarker interface {
	markHTTPError(status int)
}
