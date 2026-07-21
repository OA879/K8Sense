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
 * Typed client for K8sense's Cluster Doctor backend (/cluster-doctor/*).
 * This is K8sense's own addition on top of the forked Headlamp base — it
 * talks to backend/pkg/clusterdoctor/api, not to the Kubernetes API proxy.
 */
import { getAppUrl } from '../helpers/getAppUrl';
import { getHeadlampAPIHeaders } from '../helpers/getHeadlampAPIHeaders';

export type Severity = 'CRITICAL' | 'WARNING' | 'INFO';

export interface Finding {
  id: string;
  scanId: string;
  ruleId: string;
  ruleName: string;
  severity: Severity;
  category: string;
  namespace?: string;
  resourceKind: string;
  resourceName: string;
  description: string;
  remediation: string;
  references?: string[];
  rawObject?: string;
  detectedAt: string;
  guidedFixAvailable: boolean;
  guidedFixAction?: string;
  guidedFixWarning?: string;
}

export interface ScanSummary {
  id: string;
  clusterId: string;
  startedAt: number;
  completedAt?: number;
  status: 'running' | 'completed' | 'failed' | 'partial';
  totalFindings: number;
  criticalCount: number;
  warningCount: number;
  infoCount: number;
  skippedChecks: number;
  errorMessage?: string;
}

export interface Rule {
  id: string;
  name: string;
  severity: Severity;
  category: string;
  minK8sVersion?: string;
  clusterTypes?: string[];
  checkFn: string;
  description: string;
  remediation: string;
  guidedFix?: { action: string; warning: string };
  references?: string[];
  enabled: boolean;
}

function apiUrl(path: string): string {
  return getAppUrl() + 'cluster-doctor/' + path.replace(/^\//, '');
}

async function apiFetch<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(apiUrl(path), {
    ...init,
    headers: { ...getHeadlampAPIHeaders(), ...(init?.headers ?? {}) },
  });

  if (!response.ok) {
    const body = await response.text().catch(() => '');
    throw new Error(`cluster-doctor ${path} failed: ${response.status} ${body}`);
  }

  return response.json() as Promise<T>;
}

/** Starts a scan on the given cluster context and returns its scan id. */
export function startScan(cluster: string): Promise<{ scanId: string }> {
  return apiFetch('/scan', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ cluster }),
  });
}

/** Fetches every finding recorded for a completed (or in-progress) scan. */
export function getFindings(scanId: string): Promise<Finding[]> {
  return apiFetch(`/findings/${encodeURIComponent(scanId)}`);
}

/** Lists past scans for a cluster, most recent first. */
export function listHistory(cluster: string): Promise<ScanSummary[]> {
  return apiFetch(`/history?cluster=${encodeURIComponent(cluster)}`);
}

/** Lists all built-in + custom rules with their enabled state. */
export function listRules(): Promise<Rule[]> {
  return apiFetch('/rules');
}

/** Full URL for the SSE progress stream of one scan — see sse-client.ts. */
export function scanStatusUrl(scanId: string): string {
  return apiUrl(`/scan/${encodeURIComponent(scanId)}/status`);
}
