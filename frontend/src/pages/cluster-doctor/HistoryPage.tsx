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
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Paper from '@mui/material/Paper';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import React from 'react';
import { useHistory } from 'react-router';
import { listHistory,ScanSummary } from '../../lib/cluster-doctor-api';
import { useCluster } from '../../lib/k8s';
import { createRouteURL } from '../../lib/router/createRouteURL';

function formatTimestamp(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString();
}

function StatusChip({ status }: { status: ScanSummary['status'] }) {
  const color =
    status === 'completed'
      ? 'success'
      : status === 'failed'
      ? 'error'
      : status === 'partial'
      ? 'warning'
      : 'default';

  return <Chip size="small" label={status} color={color} />;
}

export default function HistoryPage() {
  const cluster = useCluster();
  const routerHistory = useHistory();
  const [scans, setScans] = React.useState<ScanSummary[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!cluster) return;

    let cancelled = false;

    listHistory(cluster)
      .then(result => {
        if (!cancelled) setScans(result);
      })
      .catch(e => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });

    return () => {
      cancelled = true;
    };
  }, [cluster]);

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" gutterBottom>
        Scan History
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Past Cluster Doctor scans for <strong>{cluster}</strong>.
      </Typography>

      {error && <Alert severity="error">{error}</Alert>}

      {!error && !scans && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {scans && scans.length === 0 && (
        <Alert severity="info">No scans yet. Run one from the Cluster Doctor page.</Alert>
      )}

      {scans && scans.length > 0 && (
        <Paper sx={{ p: 2 }}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Started</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Critical</TableCell>
                <TableCell align="right">Warning</TableCell>
                <TableCell align="right">Info</TableCell>
                <TableCell align="right">Total</TableCell>
                <TableCell align="right">Skipped</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {scans.map(scan => (
                <TableRow key={scan.id} hover>
                  <TableCell>{formatTimestamp(scan.startedAt)}</TableCell>
                  <TableCell>
                    <StatusChip status={scan.status} />
                  </TableCell>
                  <TableCell align="right">{scan.criticalCount}</TableCell>
                  <TableCell align="right">{scan.warningCount}</TableCell>
                  <TableCell align="right">{scan.infoCount}</TableCell>
                  <TableCell align="right">{scan.totalFindings}</TableCell>
                  <TableCell align="right">{scan.skippedChecks}</TableCell>
                  <TableCell align="right">
                    <Button
                      size="small"
                      onClick={() =>
                        routerHistory.push(
                          createRouteURL('clusterDoctorFindings', { scanId: scan.id })
                        )
                      }
                    >
                      View
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Paper>
      )}
    </Box>
  );
}
