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

import React from 'react';
import { Finding, getFindings, listHistory, ScanSummary } from './cluster-doctor-api';

/** How often the sidebar badge re-checks for a newer scan. */
const REFRESH_MS = 60_000;

/**
 * Returns the most recent scan summary for a cluster, refreshed periodically.
 * Used for the sidebar's critical-count badge. Failures resolve to null rather
 * than throwing — a missing badge must never break navigation.
 */
export function useLatestScan(cluster: string | null | undefined): ScanSummary | null {
  const [scan, setScan] = React.useState<ScanSummary | null>(null);

  React.useEffect(() => {
    if (!cluster) {
      setScan(null);
      return;
    }

    let cancelled = false;

    const load = () => {
      listHistory(cluster)
        .then(history => {
          if (!cancelled) setScan(history?.[0] ?? null);
        })
        .catch(() => {
          if (!cancelled) setScan(null);
        });
    };

    load();
    const timer = setInterval(load, REFRESH_MS);

    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [cluster]);

  return scan;
}

/**
 * Returns the findings from a cluster's latest scan that concern one specific
 * resource, so a resource detail page can show its own diagnostics. Matching is
 * by kind + name (+ namespace when the resource is namespaced).
 */
export function useFindingsForResource(
  cluster: string | null | undefined,
  kind: string,
  name: string,
  namespace?: string
): Finding[] {
  const latest = useLatestScan(cluster);
  const [findings, setFindings] = React.useState<Finding[]>([]);

  React.useEffect(() => {
    if (!latest?.id || !name) {
      setFindings([]);
      return;
    }

    let cancelled = false;

    getFindings(latest.id)
      .then(all => {
        if (cancelled) return;

        setFindings(
          (all ?? []).filter(
            f =>
              f.resourceKind === kind &&
              f.resourceName === name &&
              (!namespace || f.namespace === namespace)
          )
        );
      })
      .catch(() => {
        if (!cancelled) setFindings([]);
      });

    return () => {
      cancelled = true;
    };
  }, [latest?.id, kind, name, namespace]);

  return findings;
}
