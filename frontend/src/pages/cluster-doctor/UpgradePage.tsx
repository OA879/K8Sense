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
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import { getUpgradeReadiness, UpgradeReport } from '../../lib/cluster-doctor-upgrade-api';
import { useCluster } from '../../lib/k8s';

function StatCard({ label, value, color }: { label: string; value: React.ReactNode; color?: string }) {
  return (
    <Paper sx={{ p: 2, minWidth: 150 }}>
      <Typography variant="overline" color="text.secondary">
        {label}
      </Typography>
      <Typography variant="h4" sx={{ color }}>
        {value}
      </Typography>
    </Paper>
  );
}

export default function UpgradePage() {
  const cluster = useCluster();
  const [report, setReport] = React.useState<UpgradeReport | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [target, setTarget] = React.useState('');

  const load = React.useCallback(
    (t?: string) => {
      if (!cluster) return;

      setLoading(true);
      setError(null);

      getUpgradeReadiness(cluster, t)
        .then(r => {
          setReport(r);
          if (!t) {
            setTarget(`1.${r.targetMinor}`);
          }
        })
        .catch(e => setError(e instanceof Error ? e.message : String(e)))
        .finally(() => setLoading(false));
    },
    [cluster]
  );

  React.useEffect(() => {
    load();
  }, [load]);

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4">Upgrade Readiness</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        Which workloads on <strong>{cluster}</strong> use APIs that are deprecated or removed in a
        target Kubernetes version — a pre-upgrade checklist.
      </Typography>

      <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', mb: 3, flexWrap: 'wrap' }}>
        <TextField
          size="small"
          label="Target version"
          placeholder="1.32"
          value={target}
          onChange={e => setTarget(e.target.value)}
          onKeyDown={e => e.key === 'Enter' && load(target)}
          sx={{ width: 140 }}
        />
        <Button variant="contained" size="small" onClick={() => load(target)}>
          Check
        </Button>
        {report?.currentVersion && (
          <Typography variant="body2" color="text.secondary">
            Current: <strong>{report.currentVersion}</strong>
          </Typography>
        )}
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {loading && !report && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {report && (
        <>
          <Box sx={{ display: 'flex', gap: 2, mb: 3, flexWrap: 'wrap' }}>
            <StatCard label="Target" value={`1.${report.targetMinor}`} />
            <StatCard
              label="Blockers"
              value={report.blockers}
              color={report.blockers > 0 ? 'error.main' : 'success.main'}
            />
            <StatCard
              label="Warnings"
              value={report.warnings}
              color={report.warnings > 0 ? 'warning.main' : 'success.main'}
            />
          </Box>

          {report.items.length === 0 ? (
            <Alert severity="success">
              Nothing uses a deprecated or removed API for 1.{report.targetMinor}. Clear to upgrade
              (from what&apos;s applied in-cluster).
            </Alert>
          ) : (
            <Paper sx={{ p: 2 }}>
              <Table size="small">
                <TableHead>
                  <TableRow>
                    <TableCell>Severity</TableCell>
                    <TableCell>Resource</TableCell>
                    <TableCell>Uses</TableCell>
                    <TableCell>Removed in</TableCell>
                    <TableCell>Migrate to</TableCell>
                    <TableCell>Source</TableCell>
                  </TableRow>
                </TableHead>
                <TableBody>
                  {report.items.map((it, i) => (
                    <TableRow key={`${it.kind}-${it.namespace}-${it.name}-${i}`} hover>
                      <TableCell>
                        <Chip
                          size="small"
                          label={it.severity}
                          color={it.severity === 'blocker' ? 'error' : 'warning'}
                        />
                      </TableCell>
                      <TableCell>
                        <strong>{it.kind}</strong> {[it.namespace, it.name].filter(Boolean).join('/')}
                      </TableCell>
                      <TableCell>
                        <code>{it.apiVersion}</code>
                      </TableCell>
                      <TableCell>{it.removedIn || '—'}</TableCell>
                      <TableCell>
                        <code>{it.replacement}</code>
                      </TableCell>
                      <TableCell>
                        <Chip size="small" variant="outlined" label={it.source} />
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </Paper>
          )}

          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 2 }}>
            Detected from the last-applied-configuration annotation and Helm release manifests.
            Objects applied server-side with neither can be missed — treat a clean result as
            necessary, not sufficient.
          </Typography>
        </>
      )}
    </Box>
  );
}
