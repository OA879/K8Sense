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
import Paper from '@mui/material/Paper';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import React from 'react';
import { AuditEntry, listAuditLog } from '../../lib/cluster-doctor-audit-api';
import { useCluster } from '../../lib/k8s';

function formatTimestamp(unixSeconds: number): string {
  return new Date(unixSeconds * 1000).toLocaleString();
}

function ResultChip({ result }: { result: string }) {
  const color = result === 'success' ? 'success' : 'error';

  return <Chip size="small" label={result} color={color} />;
}

export default function AuditLogPage() {
  const cluster = useCluster();
  const [entries, setEntries] = React.useState<AuditEntry[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!cluster) return;

    let cancelled = false;

    listAuditLog(cluster)
      .then(result => {
        if (!cancelled) setEntries(result);
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
        Audit Log
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Guided Fix actions recorded for <strong>{cluster}</strong>.
      </Typography>

      {error && <Alert severity="error">{error}</Alert>}

      {!error && !entries && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {entries && entries.length === 0 && (
        <Alert severity="info">No audit entries yet for this cluster.</Alert>
      )}

      {entries && entries.length > 0 && (
        <Paper sx={{ p: 2 }}>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Time</TableCell>
                <TableCell>Actor</TableCell>
                <TableCell>Action</TableCell>
                <TableCell>Resource</TableCell>
                <TableCell>Result</TableCell>
                <TableCell>Error</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {entries.map(entry => (
                <TableRow key={entry.id} hover>
                  <TableCell>{formatTimestamp(entry.performedAt)}</TableCell>
                  <TableCell>{entry.actor}</TableCell>
                  <TableCell>{entry.action}</TableCell>
                  <TableCell>
                    {[entry.namespace, entry.resourceName].filter(Boolean).join('/')}
                  </TableCell>
                  <TableCell>
                    <ResultChip result={entry.result} />
                  </TableCell>
                  <TableCell>{entry.error}</TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Paper>
      )}
    </Box>
  );
}
