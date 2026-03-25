// Package apperrors defines sentinel errors used across the backend.
// Centralising errors in one package makes them easy to find, document,
// and test with errors.Is / errors.As.
package apperrors

import (
	"errors"

	"connectrpc.com/connect"
)

// ── JWT / Auth ─────────────────────────────────────────

// ErrUnexpectedSigningMethod indicates the JWT was signed with an algorithm
// the server does not accept.
var ErrUnexpectedSigningMethod = errors.New("unexpected signing method")

// ErrInvalidToken indicates the JWT failed validation (expired, malformed, etc.).
var ErrInvalidToken = errors.New("invalid token")

// ── OAuth ──────────────────────────────────────────────

// ErrNoIDToken indicates Apple's token response did not contain an id_token.
var ErrNoIDToken = errors.New("no id_token in Apple token response")

// ErrMalformedIDToken indicates the Apple id_token JWT could not be parsed.
var ErrMalformedIDToken = errors.New("malformed Apple id_token")

// ErrUniqueUsernameFailed indicates all attempts to generate a unique username
// during OAuth user creation were exhausted.
var ErrUniqueUsernameFailed = errors.New("could not create unique username")

// ── OAuth provider fetch errors ────────────────────────

// ErrGoogleRequest indicates the Google userinfo HTTP request could not be created or sent.
var ErrGoogleRequest = errors.New("google userinfo request failed")

// ErrGoogleDecode indicates the Google userinfo response could not be decoded.
var ErrGoogleDecode = errors.New("google userinfo decode failed")

// ErrGitHubRequest indicates the GitHub user HTTP request could not be created or sent.
var ErrGitHubRequest = errors.New("github user request failed")

// ErrGitHubDecode indicates the GitHub user response could not be decoded.
var ErrGitHubDecode = errors.New("github user decode failed")

// ErrAppleIDTokenDecode indicates the Apple id_token payload could not be base64-decoded.
var ErrAppleIDTokenDecode = errors.New("decode Apple id_token payload failed")

// ErrAppleIDTokenParse indicates the Apple id_token claims could not be parsed.
var ErrAppleIDTokenParse = errors.New("parse Apple id_token claims failed")

// ── ConnectRPC errors ──────────────────────────────────
// Pre-built connect errors avoid allocating identical errors on every request.

// ErrUnauthenticated is returned when no valid credentials are provided.
var ErrUnauthenticated = connect.NewError(connect.CodeUnauthenticated, nil)

// ErrPermissionDenied is returned when the caller lacks the required role or ownership.
var ErrPermissionDenied = connect.NewError(connect.CodePermissionDenied, nil)

// ErrInvalidArgument is returned when a required field is missing or invalid.
var ErrInvalidArgument = connect.NewError(connect.CodeInvalidArgument, nil)

// ErrNotFound is returned when the requested resource does not exist.
var ErrNotFound = connect.NewError(connect.CodeNotFound, nil)

// ErrAlreadyExists is returned when a unique constraint would be violated.
var ErrAlreadyExists = connect.NewError(connect.CodeAlreadyExists, nil)

// ErrInternal is returned on unexpected server-side failures.
var ErrInternal = connect.NewError(connect.CodeInternal, nil)

// ErrFailedPrecondition is returned when a prerequisite for the operation is not met.
var ErrFailedPrecondition = connect.NewError(connect.CodeFailedPrecondition, nil)
