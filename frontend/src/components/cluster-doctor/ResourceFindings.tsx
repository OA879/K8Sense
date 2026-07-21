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

import Box from '@mui/material/Box';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import React from 'react';
import { useFindingsForResource } from '../../lib/cluster-doctor-badge';
import { useCluster } from '../../lib/k8s';
import { SeverityBadge } from './SeverityBadge';

export interface ResourceFindingsProps {
  kind: string;
  name: string;
  namespace?: string;
}

/**
 * Shows the Cluster Doctor findings from the latest scan that concern one
 * specific resource, for embedding on a resource detail page. Renders nothing
 * when the resource is clean, so it never adds noise to a healthy page.
 */
export function ResourceFindings({ kind, name, namespace }: ResourceFindingsProps) {
  const cluster = useCluster();
  const findings = useFindingsForResource(cluster, kind, name, namespace);

  if (findings.length === 0) {
    return null;
  }

  return (
    <Paper sx={{ p: 2, mb: 2 }}>
      <Typography variant="h6" gutterBottom>
        Cluster Doctor — {findings.length} finding{findings.length === 1 ? '' : 's'}
      </Typography>
      {findings.map(f => (
        <Box key={f.id} sx={{ display: 'flex', gap: 1.5, alignItems: 'flex-start', mb: 1.5 }}>
          <SeverityBadge severity={f.severity} />
          <Box>
            <Typography variant="body2" sx={{ fontWeight: 600 }}>
              {f.ruleId} — {f.ruleName}
            </Typography>
            <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap' }}>
              {f.remediation}
            </Typography>
          </Box>
        </Box>
      ))}
    </Paper>
  );
}

export default ResourceFindings;
