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
import Switch from '@mui/material/Switch';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import React from 'react';
import { SeverityBadge } from '../../components/cluster-doctor/SeverityBadge';
import { Rule } from '../../lib/cluster-doctor-api';
import { listRulesForCluster, toggleRule } from '../../lib/cluster-doctor-rules-api';
import { useCluster } from '../../lib/k8s';

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

  React.useEffect(() => {
    if (!cluster) return;

    let cancelled = false;

    listRulesForCluster(cluster)
      .then(result => {
        if (!cancelled) setRules(result);
      })
      .catch(e => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      });

    return () => {
      cancelled = true;
    };
  }, [cluster]);

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
