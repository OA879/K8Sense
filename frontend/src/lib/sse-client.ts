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
 * Thin EventSource wrapper for the Cluster Doctor scan progress stream. Per
 * K8SENSE_CONTEXT.md, /cluster-doctor/scan/:id/status must be consumed with
 * EventSource (not fetch) — this is the one place in the app that does.
 */
import { scanStatusUrl } from './cluster-doctor-api';

export type ScanProgressEventType =
  | 'category_started'
  | 'category_completed'
  | 'finding'
  | 'scan_completed'
  | 'scan_failed';

export interface ScanProgressEvent {
  type: ScanProgressEventType;
  category?: string;
  finding?: import('./cluster-doctor-api').Finding;
  error?: string;
}

export interface ScanProgressHandlers {
  onEvent: (event: ScanProgressEvent) => void;
  onError?: (err: Event) => void;
}

/**
 * Opens an EventSource against the given scan's status stream. Returns a
 * close() function the caller must invoke on unmount to avoid leaking the
 * connection.
 */
export function watchScanProgress(scanId: string, handlers: ScanProgressHandlers): () => void {
  const source = new EventSource(scanStatusUrl(scanId));

  source.onmessage = ev => {
    try {
      const parsed = JSON.parse(ev.data) as ScanProgressEvent;
      handlers.onEvent(parsed);

      if (parsed.type === 'scan_completed' || parsed.type === 'scan_failed') {
        source.close();
      }
    } catch {
      // Ignore malformed events rather than crashing the scan page.
    }
  };

  source.onerror = err => {
    handlers.onError?.(err);
  };

  return () => source.close();
}
