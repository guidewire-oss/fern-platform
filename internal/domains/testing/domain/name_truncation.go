package domain

import "unicode/utf8"

const (
	// MaxNameLengthBytes bounds spec/suite/tag names by UTF-8 byte length
	// (not rune count) before they're persisted. spec_runs.spec_name and
	// suite_runs.suite_name are both indexed, and tags.name carries a
	// unique constraint backed by an index -- Postgres B-tree index
	// entries have a hard ceiling around 2704 bytes (1/3 of an 8KB
	// page), and exceeding it fails the insert with "index row size ...
	// exceeds btree version 4 maximum 2704", regardless of the TEXT
	// column widening. A rune-counted bound doesn't protect against
	// that: a 2048-*rune* name can be up to 8192 bytes for 4-byte-per-
	// rune content (emoji, some CJK), which still blows the index limit
	// and reintroduces the batch-insert failure this fix exists to
	// close. Bounding by byte length directly ties the limit to what
	// Postgres actually enforces; 2048 bytes leaves solid headroom under
	// 2704 regardless of the name's character mix.
	// See https://github.com/guidewire-oss/fern-platform/issues/230.
	MaxNameLengthBytes = 2048

	// TruncationMarker is appended to any name that TruncateName had to
	// shorten, so a truncated name is visibly distinguishable from one
	// that was always short.
	TruncationMarker = " …[truncated]"
)

// TruncateName bounds name to MaxNameLengthBytes UTF-8 bytes, appending
// TruncationMarker when it had to cut something off. The cut point is
// backed off to the nearest rune boundary so a multi-byte character is
// never split in half, which would otherwise corrupt the string's UTF-8
// encoding.
//
// This lives in the domain package (rather than application, where the
// spec/suite-name callers are) so both the application layer and the
// infrastructure layer -- which is where tag names are converted for
// persistence -- can call it without infrastructure having to depend on
// application, which would invert the normal dependency direction.
func TruncateName(name string) string {
	if len(name) <= MaxNameLengthBytes {
		return name
	}

	keep := max(MaxNameLengthBytes-len(TruncationMarker), 0)
	for keep > 0 && !utf8.RuneStart(name[keep]) {
		keep--
	}

	return name[:keep] + TruncationMarker
}
