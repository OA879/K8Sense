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
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import LinearProgress from '@mui/material/LinearProgress';
import Paper from '@mui/material/Paper';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import React from 'react';
import { CostCategory, CostReport, getCost } from '../../lib/cluster-doctor-cost-api';
import { useCluster } from '../../lib/k8s';

const CATEGORY_LABEL: Record<CostCategory, string> = {
  'idle-loadbalancer': 'Idle load balancer',
  'unused-pvc': 'Unused volume claim',
  'orphaned-pv': 'Orphaned volume',
};

function usd(n: number): string {
  return `$${n.toFixed(2)}`;
}

function pct(part: number, whole: number): number {
  return whole > 0 ? Math.min(100, Math.round((part / whole) * 100)) : 0;
}

function ReservationBar({
  label,
  used,
  total,
  unit,
}: {
  label: string;
  used: string;
  total: string;
  unit: string;
}) {
  const value = Number(used);
  const max = Number(total);
  const p = pct(value, max);

  return (
    <Box sx={{ mb: 1.5 }}>
      <Box sx={{ display: 'flex', justifyContent: 'space-between', mb: 0.5 }}>
        <Typography variant="body2">{label}</Typography>
        <Typography variant="body2" color="text.secondary">
          {used} / {total} {unit} reserved ({p}%)
        </Typography>
      </Box>
      <LinearProgress variant="determinate" value={p} sx={{ height: 8, borderRadius: 4 }} />
    </Box>
  );
}

export default function CostPage() {
  const cluster = useCluster();
  const [report, setReport] = React.useState<CostReport | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!cluster) return;

    let cancelled = false;
    getCost(cluster)
      .then(r => !cancelled && setReport(r))
      .catch(e => !cancelled && setError(e instanceof Error ? e.message : String(e)));

    return () => {
      cancelled = true;
    };
  }, [cluster]);

  const cpuUsed = report ? (report.cpuRequestedMilli / 1000).toFixed(1) : '0';
  const cpuTotal = report ? (report.cpuAllocatableMilli / 1000).toFixed(1) : '0';
  const gib = (b: number) => (b / (1024 * 1024 * 1024)).toFixed(1);

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4">Cost &amp; Waste</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        Spend that&apos;s provably wasted on <strong>{cluster}</strong> — derived from the Kubernetes
        API, with conservative unit-cost estimates.
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {!error && !report && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {report && (
        <>
          <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap', mb: 3 }}>
            <Paper sx={{ p: 2.5, minWidth: 240, flex: '0 0 auto' }}>
              <Typography variant="overline" color="text.secondary">
                Estimated monthly waste
              </Typography>
              <Typography variant="h3" sx={{ color: report.estMonthlyWasteUsd > 0 ? 'error.main' : 'success.main' }}>
                {usd(report.estMonthlyWasteUsd)}
              </Typography>
              <Typography variant="caption" color="text.secondary">
                across {report.items.length} item{report.items.length === 1 ? '' : 's'}
              </Typography>
            </Paper>

            <Paper sx={{ p: 2.5, flex: 1, minWidth: 300 }}>
              <Typography variant="overline" color="text.secondary">
                Cluster reservation
              </Typography>
              <Box sx={{ mt: 1 }}>
                <ReservationBar label="CPU" used={cpuUsed} total={cpuTotal} unit="vCPU" />
                <ReservationBar
                  label="Memory"
                  used={gib(report.memRequestedBytes)}
                  total={gib(report.memAllocatableBytes)}
                  unit="GiB"
                />
              </Box>
              <Typography variant="caption" color="text.secondary">
                How much of the cluster your workloads reserve via requests.
              </Typography>
            </Paper>
          </Box>

          {report.items.length === 0 ? (
            <Alert severity="success">No obvious waste found — nice and lean.</Alert>
          ) : (
            <Paper sx={{ p: 2 }}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Category</TableCell>
                    <TableCell>Resource</TableCell>
                    <TableCell>Detail</TableCell>
                    <TableCell align="right">Est. $/mo</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {report.items.map(item => (
                    <TableRow key={`${item.category}-${item.namespace}-${item.resourceName}`} hover>
                      <TableCell>
                        <Chip size="small" label={CATEGORY_LABEL[item.category] ?? item.category} />
                      </TableCell>
                      <TableCell>
                        {[item.namespace, item.resourceName].filter(Boolean).join('/')}
                        <Typography variant="caption" color="text.secondary" sx={{ ml: 0.5 }}>
                          {item.resourceKind}
                        </Typography>
                      </TableCell>
                      <TableCell>{item.detail}</TableCell>
                      <TableCell align="right">{usd(item.estMonthlyUsd)}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Paper>
          )}

          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 2 }}>
            Estimates use conservative flat unit costs (managed load balancer ≈ $18/mo, storage ≈
            $0.10/GiB-mo) and are indicative, not a bill.
          </Typography>
        </>
      )}
    </Box>
  );
}
