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

import Chip from '@mui/material/Chip';
import React from 'react';
import { Severity } from '../../lib/cluster-doctor-api';

// Brand severity palette from K8SENSE_CONTEXT.md — kept as literal hex values
// (rather than theme tokens) so severity always reads the same regardless of
// light/dark mode, the same way a CRITICAL badge should never go "muted".
const SEVERITY_COLORS: Record<Severity, string> = {
  CRITICAL: '#EF4444',
  WARNING: '#F59E0B',
  INFO: '#60A5FA',
};

export interface SeverityBadgeProps {
  severity: Severity;
}

export function SeverityBadge({ severity }: SeverityBadgeProps) {
  return (
    <Chip
      label={severity}
      size="small"
      sx={{
        backgroundColor: SEVERITY_COLORS[severity],
        color: '#FFFFFF',
        fontWeight: 700,
        letterSpacing: '0.02em',
      }}
    />
  );
}

export default SeverityBadge;
