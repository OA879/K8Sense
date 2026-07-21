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
 * Typed client for Cluster Doctor's finding suppression + comment endpoints.
 * A suppression mutes a finding for a resource across scans, so requests are
 * keyed by resource identity (cluster, rule, namespace, kind, name) rather
 * than the per-scan finding UUID.
 */
import { apiFetch } from './cluster-doctor-api';

export interface SuppressRequest {
  cluster: string;
  ruleId: string;
  namespace?: string;
  resourceKind: string;
  resourceName: string;
  reason?: string;
  comment?: string;
  by?: string;
}

/** Mutes a finding for a resource across scans. Reason is required. */
export function suppressFinding(req: SuppressRequest): Promise<{ result: string }> {
  return apiFetch('/findings/suppress', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
}

/** Removes a resource's suppression, un-muting its finding. */
export function unsuppressFinding(req: SuppressRequest): Promise<{ result: string }> {
  return apiFetch('/findings/unsuppress', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
}

/** Attaches (or updates) a comment on a resource without necessarily muting it. */
export function commentFinding(req: SuppressRequest): Promise<{ result: string }> {
  return apiFetch('/findings/comment', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
}
