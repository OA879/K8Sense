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

import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import React from 'react';
import { useHistory } from 'react-router';
import { CategoryState,ScanProgress } from '../../components/cluster-doctor/ScanProgress';
import { startScan } from '../../lib/cluster-doctor-api';
import { useCluster } from '../../lib/k8s';
import { createRouteURL } from '../../lib/router/createRouteURL';
import { ScanProgressEvent,watchScanProgress } from '../../lib/sse-client';

export default function ScanPage() {
  const cluster = useCluster();
  const history = useHistory();

  const [scanId, setScanId] = React.useState<string | null>(null);
  const [categories, setCategories] = React.useState<CategoryState[]>([]);
  const [findingCount, setFindingCount] = React.useState(0);
  const [complete, setComplete] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const stopWatching = React.useRef<() => void>();

  React.useEffect(() => () => stopWatching.current?.(), []);

  function upsertCategory(name: string, status: CategoryState['status']) {
    setCategories(prev => {
      const idx = prev.findIndex(c => c.name === name);
      if (idx === -1) {
        return [...prev, { name, status }];
      }
      const next = [...prev];
      next[idx] = { name, status };
      return next;
    });
  }

  function handleEvent(event: ScanProgressEvent) {
    switch (event.type) {
      case 'category_started':
        if (event.category) upsertCategory(event.category, 'running');
        break;
      case 'category_completed':
        if (event.category) upsertCategory(event.category, 'done');
        break;
      case 'finding':
        setFindingCount(c => c + 1);
        break;
      case 'scan_completed':
        setComplete(true);
        break;
      case 'scan_failed':
        setComplete(true);
        setError(event.error || 'Scan failed');
        break;
    }
  }

  async function handleScan() {
    if (!cluster) return;

    setError(null);
    setComplete(false);
    setFindingCount(0);
    setCategories([]);
    setScanId(null);

    try {
      const { scanId: id } = await startScan(cluster);
      setScanId(id);

      stopWatching.current = watchScanProgress(id, {
        onEvent: handleEvent,
        onError: () => setError('Lost connection to scan progress stream'),
      });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <Box sx={{ p: 3, maxWidth: 720 }}>
      <Typography variant="h4" gutterBottom>
        Cluster Doctor
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Scan <strong>{cluster}</strong> against K8sense's rule library and get a prioritised,
        remediation-ready findings report.
      </Typography>

      <Paper sx={{ p: 3 }}>
        <Button variant="contained" onClick={handleScan} disabled={!!scanId && !complete}>
          {scanId && !complete ? 'Scanning…' : 'Scan Cluster'}
        </Button>

        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {error}
          </Alert>
        )}

        {scanId && (
          <Box sx={{ mt: 3 }}>
            <ScanProgress categories={categories} findingCount={findingCount} complete={complete} />
          </Box>
        )}

        {complete && scanId && !error && (
          <Button
            sx={{ mt: 3 }}
            variant="outlined"
            onClick={() => history.push(createRouteURL('clusterDoctorFindings', { scanId }))}
          >
            View Findings
          </Button>
        )}
      </Paper>
    </Box>
  );
}
