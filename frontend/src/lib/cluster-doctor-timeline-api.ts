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

/** Typed client for the "What changed?" timeline (/cluster-doctor/timeline). */
import { apiFetch } from './cluster-doctor-api';

export type TimelineType = 'action' | 'event' | 'finding';
export type TimelineLevel = 'info' | 'warning' | 'critical';

export interface TimelineEntry {
  time: number; // unix seconds
  type: TimelineType;
  level: TimelineLevel;
  title: string;
  detail?: string;
  actor?: string;
  namespace?: string;
  resourceKind?: string;
  resourceName?: string;
}

export interface TimelineQuery {
  namespace?: string;
  resource?: string;
  sinceMinutes?: number;
}

/** Fetches the merged audit + events + findings timeline for a cluster. */
export function getTimeline(cluster: string, opts: TimelineQuery = {}): Promise<{ entries: TimelineEntry[] }> {
  const q = new URLSearchParams({ cluster });
  if (opts.namespace) q.set('namespace', opts.namespace);
  if (opts.resource) q.set('resource', opts.resource);
  if (opts.sinceMinutes) q.set('sinceMinutes', String(opts.sinceMinutes));

  return apiFetch(`/timeline?${q.toString()}`);
}
