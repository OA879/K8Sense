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
  /** Present on a reversible guided fix; names the inverse action. */
  revertAction?: string;
  /** Unix seconds this entry was undone; set once it has been reverted. */
  revertedAt?: number;
  /** On a revert entry, the id of the original action it reversed. */
  revertOf?: string;
}

/** Lists Guided Fix audit entries for a cluster, most recent first. */
export function listAuditLog(cluster: string): Promise<AuditEntry[]> {
  return apiFetch(`/audit-log?cluster=${encodeURIComponent(cluster)}`);
}

/** True when this entry is a reversible fix that has not yet been undone. */
export function canRevert(entry: AuditEntry): boolean {
  return (
    entry.result === 'success' && !!entry.revertAction && !entry.revertedAt && !entry.revertOf
  );
}

/** Undoes a previously-applied reversible guided fix (scale / uncordon). Pass the
 *  entry's cluster so the undo reaches stateless (browser-added) clusters too. */
export function revertGuidedFix(
  auditId: string,
  cluster?: string
): Promise<{ result: string; message: string }> {
  return apiFetch(
    '/guided-fix/revert',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ auditId, confirmed: true }),
    },
    cluster
  );
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
