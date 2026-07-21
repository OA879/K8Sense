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

import { Icon } from '@iconify/react';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Paper from '@mui/material/Paper';
import { useTheme } from '@mui/material/styles';
import Typography from '@mui/material/Typography';
import React from 'react';
import { useHistory } from 'react-router';
import { startScan } from '../../lib/cluster-doctor-api';
import { useLatestScan } from '../../lib/cluster-doctor-badge';
import { useCluster } from '../../lib/k8s';
import { createRouteURL } from '../../lib/router/createRouteURL';

function relativeTime(unixSeconds: number): string {
  const seconds = Math.floor(Date.now() / 1000 - unixSeconds);

  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;

  return `${Math.floor(seconds / 86400)}d ago`;
}

function CountTile({ label, count, color }: { label: string; count: number; color: string }) {
  return (
    <Box sx={{ minWidth: 92 }}>
      <Typography sx={{ fontSize: 30, fontWeight: 800, lineHeight: 1.1, color }}>
        {count}
      </Typography>
      <Typography
        sx={{
          fontSize: 11,
          textTransform: 'uppercase',
          letterSpacing: '0.04em',
          color: 'text.secondary',
        }}
      >
        {label}
      </Typography>
    </Box>
  );
}

/**
 * Cluster Doctor health summary for the cluster overview page: the latest
 * scan's severity counts, when it ran, and a way to act on it. This is what
 * makes the overview K8sense's rather than a stock Kubernetes dashboard.
 */
export function ClusterHealthPanel() {
  const cluster = useCluster();
  const theme = useTheme();
  const routerHistory = useHistory();
  const latest = useLatestScan(cluster);
  const [scanning, setScanning] = React.useState(false);

  async function handleScan() {
    if (!cluster) return;

    setScanning(true);
    try {
      const { scanId } = await startScan(cluster);
      routerHistory.push(createRouteURL('clusterDoctorFindings', { scanId }));
    } catch {
      // Navigation is the happy path; on failure just re-enable the button and
      // let the user retry from the Cluster Doctor page.
      setScanning(false);
    }
  }

  const healthy = latest && latest.totalFindings === 0;
  const accent = !latest
    ? theme.palette.text.secondary
    : latest.criticalCount > 0
    ? theme.palette.error.main
    : latest.warningCount > 0
    ? theme.palette.warning.main
    : theme.palette.success.main;

  return (
    <Paper sx={{ p: 3, mb: 2, borderLeft: `4px solid ${accent}` }}>
      <Box
        sx={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: 2,
        }}
      >
        <Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Icon icon="mdi:stethoscope" width={22} />
            <Typography variant="h6">Cluster Health</Typography>
          </Box>
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {latest
              ? `Last scanned ${relativeTime(latest.startedAt)}`
              : 'This cluster has not been scanned yet.'}
          </Typography>
        </Box>

        {latest && !healthy && (
          <Box sx={{ display: 'flex', gap: 4 }}>
            <CountTile label="Critical" count={latest.criticalCount} color={theme.palette.error.main} />
            <CountTile label="Warning" count={latest.warningCount} color={theme.palette.warning.main} />
            <CountTile label="Info" count={latest.infoCount} color={theme.palette.info.main} />
          </Box>
        )}

        {healthy && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
            <Icon icon="mdi:check-circle" width={22} color={theme.palette.success.main} />
            <Typography sx={{ fontWeight: 600, color: theme.palette.success.main }}>
              No findings — this cluster looks healthy.
            </Typography>
          </Box>
        )}

        <Box sx={{ display: 'flex', gap: 1 }}>
          {latest && (
            <Button
              size="small"
              variant="outlined"
              onClick={() =>
                routerHistory.push(
                  createRouteURL('clusterDoctorFindings', { scanId: latest.id })
                )
              }
            >
              View Findings
            </Button>
          )}
          <Button size="small" variant="contained" disabled={scanning} onClick={handleScan}>
            {scanning ? 'Scanning…' : latest ? 'Re-scan' : 'Run First Scan'}
          </Button>
        </Box>
      </Box>
    </Paper>
  );
}

export default ClusterHealthPanel;
