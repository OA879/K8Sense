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
import Accordion from '@mui/material/Accordion';
import AccordionDetails from '@mui/material/AccordionDetails';
import AccordionSummary from '@mui/material/AccordionSummary';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
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
import {
  ComplianceReport,
  ComplianceSnapshot,
  ControlResult,
  getCompliance,
  getComplianceHistory,
} from '../../lib/cluster-doctor-compliance-api';
import { useCluster } from '../../lib/k8s';

/** A tiny dependency-free SVG sparkline of compliance scores over time. */
function Sparkline({ points }: { points: number[] }) {
  const w = 180;
  const h = 44;
  const max = 100;
  const min = Math.min(80, ...points) - 5;
  const span = Math.max(1, max - min);
  const coords = points.map((p, i) => {
    const x = points.length === 1 ? w : (i / (points.length - 1)) * w;
    const y = h - ((p - min) / span) * h;
    return `${x.toFixed(1)},${y.toFixed(1)}`;
  });
  const last = points[points.length - 1];
  const stroke = last >= 90 ? '#2e7d32' : last >= 70 ? '#ed6c02' : '#d32f2f';

  return (
    <svg width={w} height={h} style={{ overflow: 'visible' }}>
      <polyline points={coords.join(' ')} fill="none" stroke={stroke} strokeWidth={2} />
      {points.length > 0 && (
        <circle
          cx={coords[coords.length - 1].split(',')[0]}
          cy={coords[coords.length - 1].split(',')[1]}
          r={3}
          fill={stroke}
        />
      )}
    </svg>
  );
}

function scoreColor(score: number): string {
  if (score >= 90) return 'success.main';
  if (score >= 70) return 'warning.main';
  return 'error.main';
}

/** Groups controls by their CIS section, preserving sorted control order. */
function bySection(controls: ControlResult[]): Record<string, ControlResult[]> {
  const groups: Record<string, ControlResult[]> = {};
  for (const c of controls) {
    (groups[c.section] ??= []).push(c);
  }
  return groups;
}

function ControlRow({ control }: { control: ControlResult }) {
  const failed = control.status === 'fail';

  return (
    <Accordion disableGutters sx={{ '&:before': { display: 'none' } }}>
      <AccordionSummary
        expandIcon={
          control.violations.length > 0 ? <Icon icon="mdi:chevron-down" /> : <Box sx={{ width: 24 }} />
        }
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, width: '100%' }}>
          <Icon
            icon={failed ? 'mdi:close-circle' : 'mdi:check-circle'}
            color={failed ? '#d32f2f' : '#2e7d32'}
            width={20}
          />
          <Typography variant="body2" sx={{ fontWeight: 600, minWidth: 90 }}>
            {control.id}
          </Typography>
          <Typography variant="body2" sx={{ flex: 1 }}>
            {control.title}
          </Typography>
          {failed && (
            <Chip
              size="small"
              color="error"
              variant="outlined"
              label={`${control.violations.length} finding${
                control.violations.length === 1 ? '' : 's'
              }`}
            />
          )}
        </Box>
      </AccordionSummary>
      {control.violations.length > 0 && (
        <AccordionDetails sx={{ pt: 0 }}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Namespace</TableCell>
                <TableCell>Resource</TableCell>
                <TableCell>Detail</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {control.violations.map((v, i) => (
                <TableRow key={`${v.kind}-${v.namespace}-${v.name}-${i}`}>
                  <TableCell>{v.namespace || '—'}</TableCell>
                  <TableCell>
                    {v.name}
                    <Typography variant="caption" color="text.secondary" sx={{ ml: 0.5 }}>
                      {v.kind}
                    </Typography>
                  </TableCell>
                  <TableCell>{v.detail}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </AccordionDetails>
      )}
    </Accordion>
  );
}

export default function CompliancePage() {
  const cluster = useCluster();
  const [report, setReport] = React.useState<ComplianceReport | null>(null);
  const [history, setHistory] = React.useState<ComplianceSnapshot[]>([]);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!cluster) return;

    let cancelled = false;
    setReport(null);
    setHistory([]);
    setError(null);
    getCompliance(cluster)
      .then(r => !cancelled && setReport(r))
      .catch(e => !cancelled && setError(e instanceof Error ? e.message : String(e)));
    getComplianceHistory(cluster)
      .then(r => !cancelled && setHistory(r.snapshots))
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [cluster]);

  const sections = report ? bySection(report.controls) : {};

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4">Compliance &amp; CIS Benchmark</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        Audit-ready CIS Kubernetes Benchmark check for <strong>{cluster}</strong> — evaluated live
        against the API, fully offline.
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
            <Paper sx={{ p: 2.5, minWidth: 200, textAlign: 'center' }}>
              <Typography variant="overline" color="text.secondary">
                Compliance score
              </Typography>
              <Typography variant="h2" sx={{ color: scoreColor(report.score) }}>
                {report.score}%
              </Typography>
              <Typography variant="caption" color="text.secondary">
                {report.passed} of {report.total} controls passing
              </Typography>
            </Paper>

            <Paper sx={{ p: 2.5, flex: 1, minWidth: 260, display: 'flex', gap: 3, alignItems: 'center' }}>
              <Box>
                <Typography variant="h4" color="success.main">
                  {report.passed}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Passed
                </Typography>
              </Box>
              <Box>
                <Typography variant="h4" color="error.main">
                  {report.failed}
                </Typography>
                <Typography variant="caption" color="text.secondary">
                  Failed
                </Typography>
              </Box>
              <Box sx={{ flex: 1 }}>
                <Typography variant="caption" color="text.secondary">
                  {report.framework}
                </Typography>
              </Box>
            </Paper>

            {history.length >= 2 && (
              <Paper sx={{ p: 2.5, minWidth: 220 }}>
                <Typography variant="overline" color="text.secondary">
                  Score trend
                </Typography>
                <Sparkline points={history.map(h => h.score)} />
                <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                  {history.length} scheduled checks · {history[0].score}% →{' '}
                  {history[history.length - 1].score}%
                </Typography>
              </Paper>
            )}
          </Box>

          <Alert severity="info" icon={<Icon icon="mdi:radar" />} sx={{ mb: 3 }}>
            {history.length >= 2
              ? 'Compliance is monitored: each scheduled scan records a snapshot and alerts your webhook if a control drifts from passing to failing.'
              : 'Enable scheduled scans in Settings to monitor compliance over time and get a webhook alert when a cluster drifts out of compliance.'}
          </Alert>

          {Object.entries(sections).map(([section, controls]) => (
            <Box key={section} sx={{ mb: 2.5 }}>
              <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
                {section}
              </Typography>
              <Paper variant="outlined">
                {controls.map(c => (
                  <ControlRow key={c.id} control={c} />
                ))}
              </Paper>
            </Box>
          ))}

          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 2 }}>
            {report.note}
          </Typography>
        </>
      )}
    </Box>
  );
}
