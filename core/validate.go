package core

import (
	"fmt"
	"regexp"
)

// App users are addressed by their client-supplied external_id, which is placed
// directly into the request path (/apps/{app}/users/{external_id}/...). The API
// resolves the user by (application_id, external_id) from the authenticated app
// context, so the client never resolves or holds an internal user id.
//
// The value must be URL-path-safe: percent-encoding handles most characters, but
// a "/" (encoded %2F) and whitespace are rejected by common routing layers, so
// they are rejected up front with a clear error.

// UnsafeExternalID matches the characters a URL-path external id must not carry.
var UnsafeExternalID = regexp.MustCompile(`[\s/\\]`)

// AssertExternalID returns a *Error (config_error) unless value is a usable,
// URL-path-safe external id: non-empty, at most 255 characters, with no
// whitespace, "/" or backslash.
func AssertExternalID(value string) error {
	if len(value) == 0 || len(value) > 255 {
		return NewConfigError("`externalId` must be a non-empty string of at most 255 characters.")
	}
	if UnsafeExternalID.MatchString(value) {
		return NewConfigError(fmt.Sprintf(
			"Invalid externalId %q: it must not contain whitespace, \"/\" or \"\\\".", value))
	}
	return nil
}
