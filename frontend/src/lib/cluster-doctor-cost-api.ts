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

/** Typed client for the cost & waste finder (/cluster-doctor/cost). */
import { apiFetch } from './cluster-doctor-api';

export type CostCategory = 'idle-loadbalancer' | 'unused-pvc' | 'orphaned-pv';

export interface CostItem {
  category: CostCategory;
  namespace?: string;
  resourceKind: string;
  resourceName: string;
  detail: string;
  estMonthlyUsd: number;
}

export interface CostReport {
  cpuRequestedMilli: number;
  cpuAllocatableMilli: number;
  memRequestedBytes: number;
  memAllocatableBytes: number;
  estMonthlyWasteUsd: number;
  items: CostItem[];
}

/** Fetches the cost & waste report for a cluster (optionally one namespace). */
export function getCost(cluster: string, namespace?: string): Promise<CostReport> {
  const q = new URLSearchParams({ cluster });
  if (namespace) {
    q.set('namespace', namespace);
  }

  return apiFetch(`/cost?${q.toString()}`);
}
