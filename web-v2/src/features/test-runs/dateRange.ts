// Maps the custom date-range inputs (native <input type="date"> values,
// `YYYY-MM-DD`) to the RFC3339 bounds the test-runs API expects on
// `from`/`to`. `from` anchors to the start of that UTC day and `to` to the
// end of it, so selecting a single day includes the whole day rather than
// an empty instant. Empty inputs map to `undefined` (that bound is left
// unconstrained). UTC boundaries keep the mapping deterministic regardless
// of the viewer's timezone.
export function customRangeToQuery(
  fromDate: string,
  toDate: string,
): { from?: string; to?: string } {
  const out: { from?: string; to?: string } = {};
  if (fromDate) out.from = `${fromDate}T00:00:00.000Z`;
  if (toDate) out.to = `${toDate}T23:59:59.999Z`;
  return out;
}

// Inverse of customRangeToQuery: recover the `YYYY-MM-DD` input values from
// the filter's ISO bounds so the date inputs stay populated across
// re-renders and deep-links. Empty when the bound is unset.
export function queryToRangeInputs(
  from: string | undefined,
  to: string | undefined,
): { fromDate: string; toDate: string } {
  return {
    fromDate: from ? from.slice(0, 10) : '',
    toDate: to ? to.slice(0, 10) : '',
  };
}
