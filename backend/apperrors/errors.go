// Package apperrors defines sentinel errors used across the backend.
// Centralising errors in one package makes them easy to find, document,
// and test with errors.Is / errors.As.
package apperrors

import "errors"

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
