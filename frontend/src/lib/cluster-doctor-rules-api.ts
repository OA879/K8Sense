/*
 * Copyright 2025 The Kubernetes Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

/**
 * Typed client for K8sense's per-cluster Rule Management endpoints. It builds
 * on the shared apiFetch/apiUrl helpers from cluster-doctor-api so the rule
 * library and its toggles live in their own module without duplicating the
 * auth/base-URL plumbing.
 */
import { apiFetch, Rule } from './cluster-doctor-api';

/**
 * Lists every rule with its enabled state resolved for the given cluster — a
 * rule disabled for that cluster comes back with enabled: false.
 */
export function listRulesForCluster(cluster: string): Promise<Rule[]> {
  return apiFetch(`/rules?cluster=${encodeURIComponent(cluster)}`);
}

/**
 * Enables or disables one rule for one cluster. The backend UPSERTs the
 * override, so this is safe to call repeatedly for the same rule.
 */
export function toggleRule(
  cluster: string,
  ruleId: string,
  enabled: boolean
): Promise<{ result: string }> {
  return apiFetch(
    `/rules/${encodeURIComponent(ruleId)}/toggle?cluster=${encodeURIComponent(
      cluster
    )}&enabled=${enabled}`,
    { method: 'PUT' }
  );
}
