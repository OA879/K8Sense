/** API for custom rule import/validate, using shared apiFetch. */
import { apiFetch } from './cluster-doctor-api';

export interface RuleValidateResult {
  valid: boolean;
  rules?: string[];
  error?: string;
}

export function validateRuleYAML(yaml: string): Promise<RuleValidateResult> {
  return apiFetch('/rules/validate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ yaml }),
  });
}

export function importRuleYAML(yaml: string): Promise<{ result: string; imported: number }> {
  return apiFetch('/rules/import', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ yaml }),
  });
}
