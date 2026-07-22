import { describe, it, expect } from 'vitest';
import { FERN_FIELDS, validateMapping, type FieldMappingEntry } from './fieldMapping';

// A fully-valid baseline: both required Fern fields mapped to distinct
// JIRA fields, nothing else set.
function baseline(): FieldMappingEntry[] {
  return [
    { fernField: 'REQUIREMENT_ID', jiraFieldId: 'summary', jiraFieldIsMultiValue: false },
    { fernField: 'REQUIREMENT_TITLE', jiraFieldId: 'customfield_1', jiraFieldIsMultiValue: false },
  ];
}

describe('FERN_FIELDS metadata', () => {
  it('marks REQUIREMENT_ID and REQUIREMENT_TITLE required, TAGS multi-value', () => {
    const byField = Object.fromEntries(FERN_FIELDS.map((f) => [f.field, f]));
    expect(byField.REQUIREMENT_ID.required).toBe(true);
    expect(byField.REQUIREMENT_TITLE.required).toBe(true);
    expect(byField.DESCRIPTION.required).toBe(false);
    expect(byField.TAGS.multiValue).toBe(true);
    expect(FERN_FIELDS).toHaveLength(8);
  });
});

describe('validateMapping', () => {
  it('accepts a mapping with both required fields mapped', () => {
    expect(validateMapping(baseline())).toEqual([]);
  });

  it('flags a required Fern field left unmapped', () => {
    const entries = baseline().map((e) =>
      e.fernField === 'REQUIREMENT_ID' ? { ...e, jiraFieldId: '' } : e,
    );
    const errs = validateMapping(entries);
    expect(errs.some((e) => e.fernField === 'REQUIREMENT_ID')).toBe(true);
  });

  it('flags a required Fern field entirely missing', () => {
    const errs = validateMapping([
      { fernField: 'REQUIREMENT_TITLE', jiraFieldId: 'summary', jiraFieldIsMultiValue: false },
    ]);
    expect(errs.some((e) => e.fernField === 'REQUIREMENT_ID')).toBe(true);
  });

  it('flags two Fern fields mapped to the same JIRA field', () => {
    const entries = baseline().map((e) => ({ ...e, jiraFieldId: 'summary' }));
    const errs = validateMapping(entries);
    expect(errs.some((e) => /already mapped|same JIRA field|duplicate/i.test(e.message))).toBe(true);
  });

  it('requires a reduction strategy for a multi-value JIRA field on a single-value Fern field', () => {
    const entries = [
      ...baseline(),
      {
        fernField: 'DESCRIPTION' as const,
        jiraFieldId: 'labels',
        jiraFieldIsMultiValue: true,
      },
    ];
    const errs = validateMapping(entries);
    expect(errs.some((e) => e.fernField === 'DESCRIPTION')).toBe(true);

    // ...and passes once a strategy is chosen.
    const fixed = entries.map((e) =>
      e.fernField === 'DESCRIPTION' ? { ...e, reductionStrategy: 'FIRST_VALUE' as const } : e,
    );
    expect(validateMapping(fixed)).toEqual([]);
  });

  it('allows a multi-value JIRA field on a multi-value Fern field (TAGS) without a strategy', () => {
    const entries = [
      ...baseline(),
      { fernField: 'TAGS' as const, jiraFieldId: 'labels', jiraFieldIsMultiValue: true },
    ];
    expect(validateMapping(entries)).toEqual([]);
  });
});
