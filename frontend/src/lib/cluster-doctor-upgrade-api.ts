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

/** Typed client for upgrade & deprecation readiness (/cluster-doctor/upgrade). */
import { apiFetch } from './cluster-doctor-api';

export interface UpgradeItem {
  namespace?: string;
  kind: string;
  name: string;
  apiVersion: string;
  deprecatedIn?: string;
  removedIn?: string;
  replacement: string;
  severity: 'blocker' | 'warning';
  source: 'applied' | 'helm';
}

export interface UpgradeReport {
  currentVersion: string;
  targetMinor: number;
  blockers: number;
  warnings: number;
  items: UpgradeItem[];
}

/** Fetches upgrade readiness for a cluster; target like "1.32" (optional). */
export function getUpgradeReadiness(cluster: string, target?: string): Promise<UpgradeReport> {
  const q = new URLSearchParams({ cluster });
  if (target) {
    q.set('target', target);
  }

  return apiFetch(`/upgrade?${q.toString()}`);
}
