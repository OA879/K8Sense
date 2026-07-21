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
 * Typed client for K8sense's Cluster Doctor audit log (/cluster-doctor/audit-log).
 * Reuses the shared apiFetch helper from cluster-doctor-api.
 */
import { apiFetch, apiUrl } from './cluster-doctor-api';
import { getHeadlampAPIHeaders } from '../helpers/getHeadlampAPIHeaders';

export interface AuditEntry {
  id: string;
  actor: string;
  action: string;
  clusterId: string;
  namespace?: string;
  resourceKind?: string;
  resourceName?: string;
  payload?: string;
  result: string;
  error?: string;
  performedAt: number;
}

/** Lists Guided Fix audit entries for a cluster, most recent first. */
export function listAuditLog(cluster: string): Promise<AuditEntry[]> {
  return apiFetch(`/audit-log?cluster=${encodeURIComponent(cluster)}`);
}

/** Downloads the full audit log for a cluster as a CSV file. */
export async function downloadAuditCSV(cluster: string): Promise<void> {
  const response = await fetch(apiUrl(`/audit-log/export?cluster=${encodeURIComponent(cluster)}`), {
    headers: { ...getHeadlampAPIHeaders() },
  });

  if (!response.ok) {
    throw new Error(`audit export failed: ${response.status}`);
  }

  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = `k8sense-audit-${cluster}.csv`;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
