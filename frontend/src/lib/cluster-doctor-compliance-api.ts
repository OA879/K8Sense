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

/** Typed client for the CIS compliance benchmark (/cluster-doctor/compliance). */
import { apiFetch } from './cluster-doctor-api';

export interface ComplianceViolation {
  namespace?: string;
  kind: string;
  name: string;
  detail?: string;
}

export interface ControlResult {
  id: string;
  title: string;
  section: string;
  status: 'pass' | 'fail';
  violations: ComplianceViolation[];
}

export interface ComplianceReport {
  framework: string;
  score: number;
  passed: number;
  failed: number;
  total: number;
  controls: ControlResult[];
  note: string;
}

/** Fetches the CIS benchmark report for a cluster (optionally one namespace). */
export function getCompliance(cluster: string, namespace?: string): Promise<ComplianceReport> {
  const q = new URLSearchParams({ cluster });
  if (namespace) {
    q.set('namespace', namespace);
  }

  return apiFetch(`/compliance?${q.toString()}`);
}

export interface ComplianceSnapshot {
  clusterId: string;
  takenAt: number;
  score: number;
  passed: number;
  failed: number;
  total: number;
  failingControls: string[];
}

/** Fetches the compliance-score trend (snapshots recorded by scheduled runs). */
export function getComplianceHistory(cluster: string): Promise<{ snapshots: ComplianceSnapshot[] }> {
  return apiFetch(`/compliance/history?cluster=${encodeURIComponent(cluster)}`);
}
