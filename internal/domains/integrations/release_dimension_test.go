package integrations_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/guidewire-oss/fern-platform/internal/domains/integrations"
)

func TestBuiltinFixVersionDimension(t *testing.T) {
	d := integrations.BuiltinFixVersionDimension()
	assert.Equal(t, "fixVersion", d.ID)
	assert.Equal(t, integrations.ReleaseDimensionFixVersion, d.Kind)
	assert.True(t, d.IsDefault)
	assert.True(t, d.Enumerable(), "fixVersion is always enumerable via the versions endpoint")
}

func TestReleaseDimension_SelectorJQL(t *testing.T) {
	tests := []struct {
		name string
		dim  integrations.ReleaseDimension
		val  string
		want string
	}{
		{
			name: "fix version",
			dim:  integrations.BuiltinFixVersionDimension(),
			val:  "2026.Bolinas",
			want: `fixVersion = "2026.Bolinas"`,
		},
		{
			name: "custom field with customfield_ prefix -> cf[id]",
			dim:  integrations.ReleaseDimension{ID: "customfield_10042", Kind: integrations.ReleaseDimensionCustomField, FieldID: "customfield_10042"},
			val:  "Palisades",
			want: `cf[10042] = "Palisades"`,
		},
		{
			name: "custom field with bare numeric id -> cf[id]",
			dim:  integrations.ReleaseDimension{ID: "10042", Kind: integrations.ReleaseDimensionCustomField, FieldID: "10042"},
			val:  "Revelstoke",
			want: `cf[10042] = "Revelstoke"`,
		},
		{
			name: "label",
			dim:  integrations.ReleaseDimension{ID: "labels", Kind: integrations.ReleaseDimensionLabel},
			val:  "ga-2026",
			want: `labels = "ga-2026"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.dim.SelectorJQL(tc.val)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

// TestReleaseDimension_SelectorJQL_Escaping pins the JQL escaping so a crafted
// release value cannot break out of the quoted string (supersedes the earlier
// fmt.Sprintf("%q") approach, which is a Go-literal quoter, not a JQL escaper).
func TestReleaseDimension_SelectorJQL_Escaping(t *testing.T) {
	d := integrations.BuiltinFixVersionDimension()

	t.Run("embedded double quote is escaped", func(t *testing.T) {
		got, err := d.SelectorJQL(`a" OR project = "X`)
		require.NoError(t, err)
		assert.Equal(t, `fixVersion = "a\" OR project = \"X"`, got)
	})

	t.Run("trailing backslash cannot escape the closing quote", func(t *testing.T) {
		// The classic break-out: a value ending in backslash. A naive quoter
		// emits ...\" which JQL reads as an escaped quote, swallowing the rest.
		// Our escaper doubles the backslash so the closing quote stays literal.
		got, err := d.SelectorJQL(`danger\`)
		require.NoError(t, err)
		assert.Equal(t, `fixVersion = "danger\\"`, got)
	})

	t.Run("control characters are escaped", func(t *testing.T) {
		got, err := d.SelectorJQL("line1\nline2\ttab")
		require.NoError(t, err)
		assert.Equal(t, `fixVersion = "line1\nline2\ttab"`, got)
	})
}

func TestReleaseDimension_SelectorJQL_Errors(t *testing.T) {
	t.Run("unknown kind", func(t *testing.T) {
		d := integrations.ReleaseDimension{ID: "x", Kind: integrations.ReleaseDimensionKind("BOGUS")}
		_, err := d.SelectorJQL("v")
		require.Error(t, err)
	})

	t.Run("custom field with non-numeric id", func(t *testing.T) {
		d := integrations.ReleaseDimension{ID: "f", Kind: integrations.ReleaseDimensionCustomField, FieldID: "notanumber"}
		_, err := d.SelectorJQL("v")
		require.Error(t, err)
	})

	t.Run("empty release value", func(t *testing.T) {
		d := integrations.BuiltinFixVersionDimension()
		_, err := d.SelectorJQL("")
		require.Error(t, err)
	})
}

func TestReleaseDimension_Enumerable(t *testing.T) {
	assert.True(t, integrations.BuiltinFixVersionDimension().Enumerable())
	// Custom fields and labels are not enumerable by static kind alone; the
	// resolver decides at runtime from the field's allowed-values, so the
	// static flag is false (UI falls back to manual entry).
	assert.False(t, integrations.ReleaseDimension{Kind: integrations.ReleaseDimensionCustomField}.Enumerable())
	assert.False(t, integrations.ReleaseDimension{Kind: integrations.ReleaseDimensionLabel}.Enumerable())
}
