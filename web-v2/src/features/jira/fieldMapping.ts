// Client-side model + validation for the JIRA field-mapping editor.
// Mirrors the server invariants in
// internal/domains/integrations/jira_field_mapping.go (validateEntries)
// so the UI can surface problems before a save round-trips and 400s.

export type FernField =
  | 'REQUIREMENT_ID'
  | 'REQUIREMENT_TITLE'
  | 'DESCRIPTION'
  | 'PARENT_REQUIREMENT'
  | 'REQUIREMENT_TYPE'
  | 'RELEASE_VERSION'
  | 'REQUIREMENT_STATUS'
  | 'TAGS';

export type ReductionStrategy = 'FIRST_VALUE' | 'CONCATENATE' | 'SEPARATE_ENTRIES';

export interface FernFieldMeta {
  field: FernField;
  label: string;
  required: boolean;
  multiValue: boolean;
  description: string;
}

// Source of truth mirrors fernFieldRegistry in the backend types.go;
// descriptions match the v1 field-mapping modal.
export const FERN_FIELDS: FernFieldMeta[] = [
  { field: 'REQUIREMENT_ID', label: 'Requirement ID', required: true, multiValue: false, description: 'Unique identifier for the requirement' },
  { field: 'REQUIREMENT_TITLE', label: 'Requirement Title', required: true, multiValue: false, description: 'Brief title of the requirement' },
  { field: 'DESCRIPTION', label: 'Description', required: false, multiValue: false, description: 'Detailed requirement description' },
  { field: 'PARENT_REQUIREMENT', label: 'Parent Requirement', required: false, multiValue: false, description: 'Link to parent epic or story' },
  { field: 'REQUIREMENT_TYPE', label: 'Requirement Type', required: false, multiValue: false, description: 'Type of requirement (Epic, Story, Bug, etc.)' },
  { field: 'RELEASE_VERSION', label: 'Release Version', required: false, multiValue: false, description: 'Target release or fix version' },
  { field: 'REQUIREMENT_STATUS', label: 'Requirement Status', required: false, multiValue: false, description: 'Current workflow status' },
  { field: 'TAGS', label: 'Tags', required: false, multiValue: true, description: 'Labels or categories' },
];

export const REDUCTION_STRATEGIES: ReductionStrategy[] = [
  'FIRST_VALUE',
  'CONCATENATE',
  'SEPARATE_ENTRIES',
];

const META = new Map<FernField, FernFieldMeta>(FERN_FIELDS.map((m) => [m.field, m]));

export interface FieldMappingEntry {
  fernField: FernField;
  jiraFieldId: string;
  jiraFieldIsMultiValue: boolean;
  reductionStrategy?: ReductionStrategy | null;
}

export interface MappingError {
  fernField?: FernField;
  message: string;
}

// validateMapping returns the user-facing subset of the server's
// invariants: required fields must be mapped, a JIRA field can't feed two
// Fern fields, and a multi-value JIRA field mapped onto a single-value
// Fern field needs a reduction strategy.
export function validateMapping(entries: FieldMappingEntry[]): MappingError[] {
  const errors: MappingError[] = [];
  const mapped = new Map<FernField, FieldMappingEntry>();
  for (const e of entries) {
    if (e.jiraFieldId) mapped.set(e.fernField, e);
  }

  // Required fields present and mapped.
  for (const meta of FERN_FIELDS) {
    if (meta.required && !mapped.has(meta.field)) {
      errors.push({ fernField: meta.field, message: `${meta.label} is required and must be mapped.` });
    }
  }

  // No JIRA field feeding two Fern fields.
  const seen = new Map<string, FernField>();
  for (const e of entries) {
    if (!e.jiraFieldId) continue;
    const prior = seen.get(e.jiraFieldId);
    if (prior) {
      errors.push({
        fernField: e.fernField,
        message: `JIRA field "${e.jiraFieldId}" is already mapped to another Fern field.`,
      });
    } else {
      seen.set(e.jiraFieldId, e.fernField);
    }
  }

  // Multi-value JIRA field on a single-value Fern field needs a strategy.
  for (const e of entries) {
    if (!e.jiraFieldId) continue;
    const meta = META.get(e.fernField);
    if (e.jiraFieldIsMultiValue && meta && !meta.multiValue && !e.reductionStrategy) {
      errors.push({
        fernField: e.fernField,
        message: `${meta.label} maps a multi-value JIRA field — choose a reduction strategy.`,
      });
    }
  }

  return errors;
}
