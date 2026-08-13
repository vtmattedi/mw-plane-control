package oidc

import "net/http"

type ProtocolError struct {
	Code            string
	Description     string
	HTTPStatus      int
	RedirectAllowed bool
}

func (e *ProtocolError) Error() string {
	return e.Code + ": " + e.Description
}

func invalidRequest(
	description string,
	redirectAllowed bool,
) *ProtocolError {
	return &ProtocolError{
		Code:            "invalid_request",
		Description:     description,
		HTTPStatus:      http.StatusBadRequest,
		RedirectAllowed: redirectAllowed,
	}
}

func invalidClient(description string) *ProtocolError {
	return &ProtocolError{
		Code:        "invalid_client",
		Description: description,
		HTTPStatus:  http.StatusUnauthorized,
	}
}

func invalidGrant(description string) *ProtocolError {
	return &ProtocolError{
		Code:        "invalid_grant",
		Description: description,
		HTTPStatus:  http.StatusBadRequest,
	}
}

func unsupportedGrantType() *ProtocolError {
	return &ProtocolError{
		Code:        "unsupported_grant_type",
		Description: "grant_type must be authorization_code",
		HTTPStatus:  http.StatusBadRequest,
	}
}

func invalidToken() *ProtocolError {
	return &ProtocolError{
		Code:        "invalid_token",
		Description: "invalid or expired access token",
		HTTPStatus:  http.StatusUnauthorized,
	}
}
