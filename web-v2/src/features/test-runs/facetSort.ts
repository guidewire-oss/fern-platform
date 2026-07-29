// Sorting helpers shared by the filter sidebar's facet groups. Kept out
// of the component files so fast refresh keeps working.

/** The text an entry reads as: its label when it has one, else its value. */
export function displayText(entry: { value: string; label?: string }): string {
  return entry.label || entry.value;
}

// One collator, reused across every comparison — building one per call
// is the expensive part of Intl-aware sorting.
const collator = new Intl.Collator(undefined, {
  numeric: true,
  sensitivity: 'base',
});

/** Case- and number-aware collation, shared by every facet sort. */
export function naturalCompare(a: string, b: string): number {
  return collator.compare(a, b);
}
