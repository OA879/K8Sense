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
import { useHistory } from 'react-router';
import { startScan } from '../../lib/cluster-doctor-api';
import { useLatestScan } from '../../lib/cluster-doctor-badge';
import { useCluster } from '../../lib/k8s';
import { createRouteURL } from '../../lib/router/createRouteURL';
import { useShortcut } from '../../lib/useShortcut';

/**
 * Binds the global Cluster Doctor keyboard shortcuts. Renders nothing — it is
 * mounted once in the app layout. Shortcuts come from the configurable
 * registry, so users can remap them in Settings.
 */
export function ClusterDoctorShortcuts() {
  const cluster = useCluster();
  const routerHistory = useHistory();
  const latest = useLatestScan(cluster);
  const scanningRef = React.useRef(false);

  useShortcut(
    'CLUSTER_DOCTOR_SCAN',
    () => {
      // Guard against a held-down key firing several scans.
      if (!cluster || scanningRef.current) return;

      scanningRef.current = true;

      startScan(cluster)
        .then(({ scanId }) => {
          routerHistory.push(createRouteURL('clusterDoctorFindings', { scanId }));
        })
        .catch(() => undefined)
        .finally(() => {
          scanningRef.current = false;
        });
    },
    undefined,
    [cluster, routerHistory]
  );

  useShortcut(
    'CLUSTER_DOCTOR_FINDINGS',
    () => {
      if (latest?.id) {
        routerHistory.push(createRouteURL('clusterDoctorFindings', { scanId: latest.id }));
      } else if (cluster) {
        // No scan yet — send them somewhere useful rather than doing nothing.
        routerHistory.push(createRouteURL('clusterDoctorScan'));
      }
    },
    undefined,
    [latest?.id, cluster, routerHistory]
  );

  return null;
}

export default ClusterDoctorShortcuts;
