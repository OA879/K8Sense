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

/** Typed client for the offline image vulnerability scanner
 *  (/cluster-doctor/vulnscan). Runs Trivy as an in-cluster Job. */
import { apiFetch } from './cluster-doctor-api';

export interface ImageVuln {
  vulnId: string;
  severity: string;
  pkgName: string;
  installedVersion: string;
  fixedVersion?: string;
  title?: string;
}

export interface ImageResult {
  image: string;
  namespaces: string[];
  counts: Record<string, number>;
  vulns: ImageVuln[];
  error?: string;
}

export interface VulnReport {
  totals: Record<string, number>;
  images: ImageResult[];
}

export interface VulnScanStatus {
  phase: string;
  finished: boolean;
  report?: VulnReport;
  /** Set when the scan Job is wedged (e.g. the cluster can't pull the Trivy image). */
  error?: string;
  /** Partial runner output while the scan is still running, for progress. */
  logs?: string;
}

export interface VulnConfig {
  image: string;
  dbRepository?: string;
  airGapped: boolean;
}

/** Starts a scan of the cluster's running images. Returns the run id. */
export function runVulnScan(cluster: string): Promise<{ runId: string; imageCount: number }> {
  return apiFetch(
    '/vulnscan',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cluster }),
    },
    cluster
  );
}

/** Polls scan phase; returns the parsed report once finished. */
export function vulnScanStatus(cluster: string, runId: string): Promise<VulnScanStatus> {
  return apiFetch(
    `/vulnscan/${encodeURIComponent(runId)}?cluster=${encodeURIComponent(cluster)}`
  );
}

/** Reads the Trivy image / DB-mirror config, plus the air-gap flag. */
export function getVulnConfig(): Promise<VulnConfig> {
  return apiFetch('/vulnscan/config');
}

/** Sets the Trivy image and optional mirrored DB repository. */
export function setVulnConfig(config: {
  image: string;
  dbRepository?: string;
}): Promise<VulnConfig> {
  return apiFetch('/vulnscan/config', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(config),
  });
}
