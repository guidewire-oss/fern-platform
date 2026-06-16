package integrations

import (
	"fmt"
	"strings"
)

// ReleaseDimensionKind enumerates how a project's "release" grouping is sourced
// from JIRA. The leaf->epic->release hierarchy is fixed; only this top layer's
// source varies (see specs/jira-requirements-sync, Req 8).
type ReleaseDimensionKind string

const (
	// ReleaseDimensionFixVersion is the built-in, zero-config dimension backed
	// by JIRA's native fixVersion field.
	ReleaseDimensionFixVersion ReleaseDimensionKind = "FIX_VERSION"
	// ReleaseDimensionCustomField groups by a custom JIRA field (e.g. a roadmap
	// "Release" field), selected via cf[<id>].
	ReleaseDimensionCustomField ReleaseDimensionKind = "CUSTOM_FIELD"
	// ReleaseDimensionLabel groups by a JIRA label value.
	ReleaseDimensionLabel ReleaseDimensionKind = "LABEL"
)

// ReleaseDimension is one grouping a project can view release coverage by.
// fixVersion is always available (BuiltinFixVersionDimension); additional
// custom dimensions are configured per project during JIRA field-mapping.
type ReleaseDimension struct {
	ID        string // "fixVersion" for built-in; JIRA field id for custom
	Label     string // display label, e.g. "Fix Version" or admin-chosen "Release"
	Kind      ReleaseDimensionKind
	FieldID   string // JIRA field id (customfield_NNNNN or NNNNN) for CUSTOM_FIELD; "" otherwise
	IsDefault bool
}

// BuiltinFixVersionDimension is the always-available default dimension.
func BuiltinFixVersionDimension() ReleaseDimension {
	return ReleaseDimension{
		ID:        "fixVersion",
		Label:     "Fix Version",
		Kind:      ReleaseDimensionFixVersion,
		IsDefault: true,
	}
}

// SelectorJQL returns the JQL predicate that selects issues belonging to the
// given release value under this dimension, e.g. `fixVersion = "2026.Bolinas"`.
// The value is JQL-escaped (not Go-quoted) so a crafted value cannot break out
// of the quoted string.
func (d ReleaseDimension) SelectorJQL(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("release value is required")
	}
	quoted := JQLQuoteString(value)

	switch d.Kind {
	case ReleaseDimensionFixVersion:
		return "fixVersion = " + quoted, nil
	case ReleaseDimensionCustomField:
		numericID, err := customFieldNumericID(d.FieldID)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("cf[%s] = %s", numericID, quoted), nil
	case ReleaseDimensionLabel:
		return "labels = " + quoted, nil
	default:
		return "", fmt.Errorf("unknown release dimension kind %q", d.Kind)
	}
}

// Enumerable reports whether release values can be listed up front (vs requiring
// the user to type one). Only fixVersion is statically enumerable; custom-field
// enumerability depends on the field's type and is decided at query time, so it
// reports false here (the UI falls back to manual entry).
func (d ReleaseDimension) Enumerable() bool {
	return d.Kind == ReleaseDimensionFixVersion
}

// customFieldNumericID extracts the numeric id JQL's cf[...] form requires from
// a field id that may be "customfield_10042" or a bare "10042".
func customFieldNumericID(fieldID string) (string, error) {
	id := strings.TrimPrefix(strings.TrimSpace(fieldID), "customfield_")
	if id == "" {
		return "", fmt.Errorf("custom field id is required for a CUSTOM_FIELD dimension")
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("custom field id %q is not numeric (expected customfield_NNNNN or NNNNN)", fieldID)
		}
	}
	return id, nil
}

// JQLQuoteString wraps a value in double quotes and escapes the characters that
// are significant inside a JQL quoted string. This is a JQL escaper, not Go's
// %q: it doubles backslashes (so a trailing backslash cannot escape the closing
// quote) and escapes embedded quotes and control characters. Exported because
// the coverage resolver also builds JQL fragments (project key, epic keys) that
// must use the same escaping rather than fmt's %q.
func JQLQuoteString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
