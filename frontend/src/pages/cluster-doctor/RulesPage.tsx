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
import CircularProgress from '@mui/material/CircularProgress';
import Paper from '@mui/material/Paper';
import Switch from '@mui/material/Switch';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import { SeverityBadge } from '../../components/cluster-doctor/SeverityBadge';
import { Rule } from '../../lib/cluster-doctor-api';
import { listRulesForCluster, toggleRule } from '../../lib/cluster-doctor-rules-api';
import { importRuleYAML, validateRuleYAML } from '../../lib/cluster-doctor-rules-import-api';
import { useCluster } from '../../lib/k8s';

const SAMPLE_RULE_YAML = `- id: CUSTOM-001
  name: My Custom Rule
  severity: WARNING
  category: custom
  check_fn: check_something
  description: What this detects
  remediation: |
    Steps to fix it
`;

function CustomRuleImport({ onImported }: { onImported: () => void }) {
  const [open, setOpen] = React.useState(false);
  const [yaml, setYaml] = React.useState(SAMPLE_RULE_YAML);
  const [status, setStatus] = React.useState<{ ok: boolean; msg: string } | null>(null);
  const [busy, setBusy] = React.useState(false);

  async function handleValidate() {
    setBusy(true);
    setStatus(null);
    try {
      const res = await validateRuleYAML(yaml);
      setStatus(
        res.valid
          ? { ok: true, msg: `Valid — ${res.rules?.length ?? 0} rule(s): ${res.rules?.join(', ')}` }
          : { ok: false, msg: res.error ?? 'Invalid' }
      );
    } catch (e) {
      setStatus({ ok: false, msg: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  async function handleImport() {
    setBusy(true);
    setStatus(null);
    try {
      const res = await importRuleYAML(yaml);
      setStatus({ ok: true, msg: `Imported ${res.imported} rule(s).` });
      onImported();
    } catch (e) {
      setStatus({ ok: false, msg: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Paper sx={{ p: 2, mb: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography variant="h6">Custom Rules</Typography>
        <Button size="small" onClick={() => setOpen(o => !o)}>
          {open ? 'Cancel' : 'Import YAML Rule'}
        </Button>
      </Box>
      {open && (
        <Box sx={{ mt: 2 }}>
          <TextField
            multiline
            minRows={6}
            fullWidth
            value={yaml}
            onChange={e => setYaml(e.target.value)}
            InputProps={{ style: { fontFamily: 'monospace', fontSize: 13 } }}
          />
          <Box sx={{ display: 'flex', gap: 1, mt: 1 }}>
            <Button size="small" variant="outlined" onClick={handleValidate} disabled={busy}>
              Validate
            </Button>
            <Button size="small" variant="contained" onClick={handleImport} disabled={busy}>
              Import
            </Button>
          </Box>
          {status && (
            <Alert severity={status.ok ? 'success' : 'error'} sx={{ mt: 2 }}>
              {status.msg}
            </Alert>
          )}
        </Box>
      )}
    </Paper>
  );
}

// groupByCategory buckets rules under their category, preserving the order in
// which each category first appears so the page layout stays stable between
// reloads.
function groupByCategory(rules: Rule[]): [string, Rule[]][] {
  const groups = new Map<string, Rule[]>();

  for (const rule of rules) {
    const bucket = groups.get(rule.category);
    if (bucket) {
      bucket.push(rule);
    } else {
      groups.set(rule.category, [rule]);
    }
  }

  return Array.from(groups.entries());
}

export default function RulesPage() {
  const cluster = useCluster();
  const [rules, setRules] = React.useState<Rule[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);

  const loadRules = React.useCallback(() => {
    if (!cluster) return;

    listRulesForCluster(cluster)
      .then(setRules)
      .catch(e => setError(e instanceof Error ? e.message : String(e)));
  }, [cluster]);

  React.useEffect(loadRules, [loadRules]);

  function handleToggle(rule: Rule, nextEnabled: boolean) {
    if (!cluster) return;

    // Optimistically flip the switch, then reconcile with the backend.
    setRules(current =>
      current
        ? current.map(r => (r.id === rule.id ? { ...r, enabled: nextEnabled } : r))
        : current
    );
    setError(null);

    toggleRule(cluster, rule.id, nextEnabled).catch(e => {
      // Revert on failure and surface the reason.
      setRules(current =>
        current
          ? current.map(r => (r.id === rule.id ? { ...r, enabled: !nextEnabled } : r))
          : current
      );
      setError(e instanceof Error ? e.message : String(e));
    });
  }

  const grouped = rules ? groupByCategory(rules) : [];

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4" gutterBottom>
        Rule Library
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Enable or disable diagnostic rules for <strong>{cluster}</strong>. Disabled rules are
        skipped on the next scan of this cluster.
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      <CustomRuleImport onImported={loadRules} />

      {!error && !rules && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {rules && rules.length === 0 && (
        <Alert severity="info">No rules are loaded in the diagnostics engine.</Alert>
      )}

      {grouped.map(([category, categoryRules]) => (
        <Paper key={category} sx={{ p: 2, mb: 3 }}>
          <Typography variant="h6" gutterBottom>
            {category}
          </Typography>
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>ID</TableCell>
                <TableCell>Name</TableCell>
                <TableCell>Severity</TableCell>
                <TableCell align="right">Enabled</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {categoryRules.map(rule => (
                <TableRow key={rule.id} hover>
                  <TableCell>
                    <code>{rule.id}</code>
                  </TableCell>
                  <TableCell>{rule.name}</TableCell>
                  <TableCell>
                    <SeverityBadge severity={rule.severity} />
                  </TableCell>
                  <TableCell align="right">
                    <Switch
                      checked={rule.enabled}
                      onChange={(_event, checked) => handleToggle(rule, checked)}
                      inputProps={{ 'aria-label': `Toggle rule ${rule.id}` }}
                    />
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Paper>
      ))}
    </Box>
  );
}
