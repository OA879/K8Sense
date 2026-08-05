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

/** Typed client for Runbooks (/cluster-doctor/runbooks): governed Ansible
 *  automation executed against the pointed cluster as an in-cluster Job. */
import { apiFetch } from './cluster-doctor-api';

export interface RunbookVar {
  name: string;
  label: string;
  required: boolean;
  default?: string;
}

export interface RunbookTemplate {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  vars: RunbookVar[];
}

export interface RunbooksStatus {
  enabled: boolean;
  airGapped: boolean;
  runnerImage: string;
  namespace: string;
}

export interface RunLog {
  phase: string;
  finished: boolean;
  logs: string;
}

/** Lists the runbook templates. */
export function listRunbooks(): Promise<{ runbooks: RunbookTemplate[] }> {
  return apiFetch('/runbooks');
}

/** Whether the runner is bootstrapped on the cluster, plus air-gap flag. */
export function runbooksStatus(cluster: string): Promise<RunbooksStatus> {
  return apiFetch(`/runbooks/status?cluster=${encodeURIComponent(cluster)}`);
}

/** One-time bootstrap: namespace + runner ServiceAccount + RBAC. */
export function enableRunbooks(cluster: string): Promise<RunbooksStatus> {
  return apiFetch(
    '/runbooks/enable',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cluster }),
    },
    cluster
  );
}

/** Runs a runbook (or dry-runs it with check=true). Returns the run id. */
export function runRunbook(
  cluster: string,
  runbookId: string,
  vars: Record<string, string>,
  check: boolean
): Promise<{ runId: string; namespace: string }> {
  return apiFetch(
    '/runbooks/run',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cluster, runbookId, vars, check }),
    },
    cluster
  );
}

/** Polls a run's phase and accumulated logs. */
export function runLogs(cluster: string, runId: string): Promise<RunLog> {
  return apiFetch(
    `/runbooks/run/${encodeURIComponent(runId)}/logs?cluster=${encodeURIComponent(cluster)}`
  );
}

/** Sets the ansible-runner image (e.g. an internal mirror for air-gapped sites). */
export function setRunnerImage(image: string): Promise<{ image: string }> {
  return apiFetch('/runbooks/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ image }),
  });
}
