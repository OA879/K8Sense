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
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import CircularProgress from '@mui/material/CircularProgress';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import React from 'react';
import { useParams } from 'react-router';
import {
  FindingsFilter,
  FindingsFilterValue,
} from '../../components/cluster-doctor/FindingsFilter';
import { FindingsTable } from '../../components/cluster-doctor/FindingsTable';
import { GuidedFixModal } from '../../components/cluster-doctor/GuidedFixModal';
import { downloadReport, Finding, getFindings, Severity } from '../../lib/cluster-doctor-api';
import { useCluster } from '../../lib/k8s';

const ALL_SEVERITIES: Severity[] = ['CRITICAL', 'WARNING', 'INFO'];

export default function FindingsPage() {
  const { scanId } = useParams<{ scanId: string }>();
  const cluster = useCluster();
  const [findings, setFindings] = React.useState<Finding[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [filter, setFilter] = React.useState<FindingsFilterValue>({
    severities: ALL_SEVERITIES,
    search: '',
  });
  const [fixTarget, setFixTarget] = React.useState<Finding | null>(null);

  React.useEffect(() => {
    let cancelled = false;

    getFindings(scanId)
      .then(result => {
        if (!cancelled) setFindings(result);
      })
      .catch(e => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });

    return () => {
      cancelled = true;
    };
  }, [scanId]);

  const counts = React.useMemo(() => {
    const c: Record<Severity, number> = { CRITICAL: 0, WARNING: 0, INFO: 0 };
    (findings ?? []).forEach(f => c[f.severity]++);
    return c;
  }, [findings]);

  const filtered = React.useMemo(() => {
    if (!findings) return [];

    const search = filter.search.trim().toLowerCase();

    return findings.filter(f => {
      if (!filter.severities.includes(f.severity)) return false;
      if (!search) return true;

      return (
        f.ruleName.toLowerCase().includes(search) ||
        f.ruleId.toLowerCase().includes(search) ||
        f.resourceName.toLowerCase().includes(search) ||
        (f.namespace ?? '').toLowerCase().includes(search)
      );
    });
  }, [findings, filter]);

  const [exporting, setExporting] = React.useState(false);
  const [exportError, setExportError] = React.useState<string | null>(null);

  async function handleExport(format: 'html' | 'json') {
    setExporting(true);
    setExportError(null);
    try {
      await downloadReport(scanId, format);
    } catch (e) {
      setExportError(e instanceof Error ? e.message : String(e));
    } finally {
      setExporting(false);
    }
  }

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 2 }}>
        <Typography variant="h4">Findings</Typography>
        <Box sx={{ display: 'flex', gap: 1 }}>
          <Button
            size="small"
            variant="outlined"
            disabled={exporting || !findings}
            startIcon={<Icon icon="mdi:file-download-outline" />}
            onClick={() => handleExport('html')}
          >
            Export HTML
          </Button>
          <Button
            size="small"
            disabled={exporting || !findings}
            startIcon={<Icon icon="mdi:code-json" />}
            onClick={() => handleExport('json')}
          >
            JSON
          </Button>
        </Box>
      </Box>

      {exportError && (
        <Alert severity="error" sx={{ mb: 2 }}>
          Export failed: {exportError}
        </Alert>
      )}

      {error && <Alert severity="error">{error}</Alert>}

      {!error && !findings && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {findings && (
        <Paper sx={{ p: 2 }}>
          <FindingsFilter value={filter} onChange={setFilter} counts={counts} />
          <FindingsTable findings={filtered} onApplyFix={setFixTarget} />
        </Paper>
      )}

      <GuidedFixModal
        finding={fixTarget}
        cluster={cluster ?? ''}
        open={fixTarget !== null}
        onClose={() => setFixTarget(null)}
        onApplied={() => {
          // Re-fetch so the resolved finding disappears on next scan; the
          // current scan's stored findings are historical, so we just reload.
          getFindings(scanId)
            .then(setFindings)
            .catch(() => undefined);
        }}
      />
    </Box>
  );
}
