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
import CircularProgress from '@mui/material/CircularProgress';
import Paper from '@mui/material/Paper';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import React from 'react';
import { useParams } from 'react-router';
import SeverityBadge from '../../components/cluster-doctor/SeverityBadge';
import { Finding } from '../../lib/cluster-doctor-api';
import { getScanDiff, ScanDiff } from '../../lib/cluster-doctor-diff-api';

function DiffSection({
  title,
  accent,
  findings,
}: {
  title: string;
  accent: string;
  findings: Finding[];
}) {
  return (
    <Box sx={{ mb: 4 }}>
      <Typography variant="h6" gutterBottom sx={{ color: accent }}>
        {title} ({findings.length})
      </Typography>
      {findings.length === 0 ? (
        <Typography color="text.secondary" variant="body2">
          None.
        </Typography>
      ) : (
        <Paper sx={{ p: 2, borderLeft: `4px solid ${accent}` }}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Rule</TableCell>
                <TableCell>Name</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell>Resource</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {findings.map(finding => (
                <TableRow key={finding.id} hover>
                  <TableCell>{finding.ruleId}</TableCell>
                  <TableCell>{finding.ruleName}</TableCell>
                  <TableCell>
                    <SeverityBadge severity={finding.severity} />
                  </TableCell>
                  <TableCell>
                    {[finding.namespace, finding.resourceName].filter(Boolean).join('/')}
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

export default function ScanDiffPage() {
  const { scanId, prevId } = useParams<{ scanId: string; prevId: string }>();
  const [diff, setDiff] = React.useState<ScanDiff | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!scanId || !prevId) return;

    let cancelled = false;

    getScanDiff(scanId, prevId)
      .then(result => {
        if (!cancelled) setDiff(result);
      })
      .catch(e => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });

    return () => {
      cancelled = true;
    };
  }, [scanId, prevId]);

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" gutterBottom>
        Scan Comparison
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Comparing scan <strong>{scanId}</strong> against previous scan{' '}
        <strong>{prevId}</strong>.
      </Typography>

      {error && <Alert severity="error">{error}</Alert>}

      {!error && !diff && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {diff && (
        <>
          <DiffSection title="New" accent="#EF4444" findings={diff.added} />
          <DiffSection title="Resolved" accent="#22C55E" findings={diff.resolved} />
          <DiffSection title="Still present" accent="#9CA3AF" findings={diff.persisted} />
        </>
      )}
    </Box>
  );
}
