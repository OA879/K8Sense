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
import TextField from '@mui/material/TextField';
import ToggleButton from '@mui/material/ToggleButton';
import ToggleButtonGroup from '@mui/material/ToggleButtonGroup';
import React from 'react';
import { Severity } from '../../lib/cluster-doctor-api';

export interface FindingsFilterValue {
  severities: Severity[];
  search: string;
}

export interface FindingsFilterProps {
  value: FindingsFilterValue;
  onChange: (value: FindingsFilterValue) => void;
  counts: Record<Severity, number>;
}

const ALL_SEVERITIES: Severity[] = ['CRITICAL', 'WARNING', 'INFO'];

export function FindingsFilter({ value, onChange, counts }: FindingsFilterProps) {
  return (
    <Box sx={{ display: 'flex', gap: 2, alignItems: 'center', flexWrap: 'wrap', mb: 2 }}>
      <ToggleButtonGroup
        size="small"
        value={value.severities}
        onChange={(_e, next: Severity[]) => onChange({ ...value, severities: next })}
      >
        {ALL_SEVERITIES.map(severity => (
          <ToggleButton key={severity} value={severity}>
            {severity} ({counts[severity] ?? 0})
          </ToggleButton>
        ))}
      </ToggleButtonGroup>
      <TextField
        size="small"
        placeholder="Search findings…"
        value={value.search}
        onChange={e => onChange({ ...value, search: e.target.value })}
      />
    </Box>
  );
}

export default FindingsFilter;
