/**
 * API module for K8sense settings, licence, and storage endpoints. Uses the
 * shared apiFetch/apiUrl helpers from cluster-doctor-api.
 */
import { apiFetch } from './cluster-doctor-api';

export type Tier = 'free' | 'pro' | 'enterprise';

export interface LicenceInfo {
  tier: Tier;
  customerName?: string;
  maxClusters: number;
  seatCount: number;
  expiresAt?: string;
  isTrial: boolean;
  valid: boolean;
  inGrace: boolean;
  message?: string;
}

export interface StorageStats {
  scanCount: number;
  findingCount: number;
  auditCount: number;
  dbSizeBytes: number;
}

export interface TestConnResult {
  reachable: boolean;
  k8sVersion?: string;
  latencyMs?: number;
  error?: string;
}

export function getLicence(): Promise<LicenceInfo> {
  return apiFetch('/licence');
}

/** Activate a licence from pasted file content. */
export function activateLicence(content: string): Promise<LicenceInfo> {
  return apiFetch('/licence/activate', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  });
}

export function startTrial(): Promise<LicenceInfo> {
  return apiFetch('/licence/trial', { method: 'POST' });
}

export function getStorageStats(): Promise<StorageStats> {
  return apiFetch('/storage');
}

export function purgeScans(keepPerCluster: number): Promise<{ pruned: number }> {
  return apiFetch('/storage/purge', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ keepPerCluster }),
  });
}

export function testConnection(cluster: string): Promise<TestConnResult> {
  return apiFetch(`/clusters/test?cluster=${encodeURIComponent(cluster)}`);
}
