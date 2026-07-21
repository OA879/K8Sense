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
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import Collapse from '@mui/material/Collapse';
import IconButton from '@mui/material/IconButton';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import React from 'react';
import { Finding } from '../../lib/cluster-doctor-api';
import { SeverityBadge } from './SeverityBadge';

export interface FindingsTableProps {
  findings: Finding[];
  /** When set, findings with a guided fix show an "Apply Fix" button. */
  onApplyFix?: (finding: Finding) => void;
  /** When set, findings show a "Suppress" button that opens the suppress modal. */
  onSuppress?: (finding: Finding) => void;
}

const SEVERITY_ORDER: Record<Finding['severity'], number> = { CRITICAL: 0, WARNING: 1, INFO: 2 };

function FindingRow({
  finding,
  onApplyFix,
  onSuppress,
}: {
  finding: Finding;
  onApplyFix?: (finding: Finding) => void;
  onSuppress?: (finding: Finding) => void;
}) {
  const [open, setOpen] = React.useState(false);

  return (
    <>
      <TableRow
        hover
        onClick={() => setOpen(o => !o)}
        sx={{ cursor: 'pointer', opacity: finding.suppressed ? 0.55 : 1 }}
      >
        <TableCell>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.5 }}>
            <SeverityBadge severity={finding.severity} />
            {finding.suppressed && <Chip size="small" label="muted" variant="outlined" />}
          </Box>
        </TableCell>
        <TableCell>{finding.ruleId}</TableCell>
        <TableCell>{finding.ruleName}</TableCell>
        <TableCell>{finding.resourceKind}</TableCell>
        <TableCell>{finding.namespace || '—'}</TableCell>
        <TableCell>{finding.resourceName}</TableCell>
        <TableCell padding="checkbox">
          <Box sx={{ display: 'flex', gap: 0.5 }}>
            {onApplyFix && finding.guidedFixAvailable && (
              <Button
                size="small"
                variant="outlined"
                onClick={e => {
                  e.stopPropagation();
                  onApplyFix(finding);
                }}
              >
                Apply Fix
              </Button>
            )}
            {onSuppress && (
              <IconButton
                size="small"
                aria-label="Suppress finding"
                onClick={e => {
                  e.stopPropagation();
                  onSuppress(finding);
                }}
              >
                <Icon icon="mdi:bell-off-outline" />
              </IconButton>
            )}
          </Box>
        </TableCell>
        <TableCell padding="checkbox">
          <IconButton size="small" aria-label={open ? 'Collapse' : 'Expand'}>
            <Icon icon={open ? 'mdi:chevron-up' : 'mdi:chevron-down'} />
          </IconButton>
        </TableCell>
      </TableRow>
      <TableRow>
        <TableCell colSpan={8} sx={{ py: 0, borderBottom: open ? undefined : 'none' }}>
          <Collapse in={open} timeout="auto" unmountOnExit>
            <Box sx={{ py: 2, px: 1 }}>
              {finding.comment && (
                <Typography
                  variant="body2"
                  color="text.secondary"
                  sx={{ fontStyle: 'italic', whiteSpace: 'pre-wrap', mb: 2 }}
                >
                  Comment: {finding.comment}
                </Typography>
              )}
              <Typography variant="body2" sx={{ whiteSpace: 'pre-wrap', mb: 2 }}>
                {finding.description}
              </Typography>
              <Typography variant="subtitle2" gutterBottom>
                Remediation
              </Typography>
              <Box
                component="pre"
                sx={{
                  fontFamily: 'inherit',
                  whiteSpace: 'pre-wrap',
                  m: 0,
                  p: 1.5,
                  borderRadius: 1,
                  bgcolor: theme => theme.palette.background.default,
                }}
              >
                {finding.remediation}
              </Box>
            </Box>
          </Collapse>
        </TableCell>
      </TableRow>
    </>
  );
}

export function FindingsTable({ findings, onApplyFix, onSuppress }: FindingsTableProps) {
  const sorted = React.useMemo(
    () => [...findings].sort((a, b) => SEVERITY_ORDER[a.severity] - SEVERITY_ORDER[b.severity]),
    [findings]
  );

  if (findings.length === 0) {
    return (
      <Box sx={{ py: 6, textAlign: 'center' }}>
        <Typography color="text.secondary">No findings — this cluster looks healthy.</Typography>
      </Box>
    );
  }

  return (
    <Table size="small">
      <TableHead>
        <TableRow>
          <TableCell>Severity</TableCell>
          <TableCell>Rule</TableCell>
          <TableCell>Name</TableCell>
          <TableCell>Kind</TableCell>
          <TableCell>Namespace</TableCell>
          <TableCell>Resource</TableCell>
          <TableCell padding="checkbox" />
          <TableCell padding="checkbox" />
        </TableRow>
      </TableHead>
      <TableBody>
        {sorted.map(finding => (
          <FindingRow
            key={finding.id}
            finding={finding}
            onApplyFix={onApplyFix}
            onSuppress={onSuppress}
          />
        ))}
      </TableBody>
    </Table>
  );
}

export default FindingsTable;
