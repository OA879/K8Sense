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
import {
  AuditEntry,
  canRevert,
  downloadAuditCSV,
  listAuditLog,
  revertGuidedFix,
} from '../../lib/cluster-doctor-audit-api';
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
  const [reverting, setReverting] = React.useState<string | null>(null);

  const load = React.useCallback(() => {
    if (!cluster) return Promise.resolve();

    return listAuditLog(cluster)
      .then(setEntries)
      .catch(e => setError(e instanceof Error ? e.message : String(e)));
  }, [cluster]);

  React.useEffect(() => {
    let cancelled = false;
    if (cluster) {
      listAuditLog(cluster)
        .then(result => !cancelled && setEntries(result))
        .catch(e => !cancelled && setError(e instanceof Error ? e.message : String(e)));
    }

    return () => {
      cancelled = true;
    };
  }, [cluster]);

  async function handleUndo(entry: AuditEntry) {
    const target = [entry.namespace, entry.resourceName].filter(Boolean).join('/');
    if (
      !window.confirm(
        `Undo "${entry.action}" on ${target}?\n\nThis reverses the fix by restoring the previous state, and is itself recorded in the audit log.`
      )
    ) {
      return;
    }

    setReverting(entry.id);
    setError(null);

    try {
      await revertGuidedFix(entry.id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setReverting(null);
    }
  }

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 1 }}>
        <Typography variant="h4">Audit Log</Typography>
        <Button
          size="small"
          variant="outlined"
          disabled={!entries || entries.length === 0}
          onClick={() => cluster && downloadAuditCSV(cluster).catch(() => undefined)}
        >
          Export CSV
        </Button>
      </Box>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Every action recorded for <strong>{cluster}</strong> — scans, rule and licence changes,
        exports, and guided fixes. Reversible fixes can be undone here.
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
                <TableCell align="right">Undo</TableCell>
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
                  <TableCell align="right">
                    {entry.revertedAt ? (
                      <Chip size="small" label="reverted" variant="outlined" />
                    ) : canRevert(entry) ? (
                      <Button
                        size="small"
                        variant="outlined"
                        color="warning"
                        disabled={reverting === entry.id}
                        onClick={() => handleUndo(entry)}
                      >
                        {reverting === entry.id ? 'Undoing…' : 'Undo'}
                      </Button>
                    ) : null}
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
