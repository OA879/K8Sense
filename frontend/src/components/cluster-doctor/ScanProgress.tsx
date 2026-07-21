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
import Chip from '@mui/material/Chip';
import LinearProgress from '@mui/material/LinearProgress';
import Typography from '@mui/material/Typography';
import React from 'react';

export interface CategoryState {
  name: string;
  status: 'pending' | 'running' | 'done';
}

export interface ScanProgressProps {
  categories: CategoryState[];
  findingCount: number;
  complete: boolean;
}

export function ScanProgress({ categories, findingCount, complete }: ScanProgressProps) {
  return (
    <Box>
      {!complete && <LinearProgress sx={{ mb: 2 }} />}
      <Typography variant="body2" sx={{ mb: 1 }}>
        {complete
          ? `Scan complete — ${findingCount} finding${findingCount === 1 ? '' : 's'}`
          : `Scanning… ${findingCount} finding${findingCount === 1 ? '' : 's'} so far`}
      </Typography>
      <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap' }}>
        {categories.map(cat => (
          <Chip
            key={cat.name}
            size="small"
            label={cat.name}
            icon={
              cat.status === 'done' ? (
                <Icon icon="mdi:check-circle" width={16} />
              ) : cat.status === 'running' ? (
                <Icon icon="mdi:loading" width={16} className="spin" />
              ) : undefined
            }
            variant={cat.status === 'pending' ? 'outlined' : 'filled'}
            color={
              cat.status === 'done' ? 'success' : cat.status === 'running' ? 'primary' : 'default'
            }
          />
        ))}
      </Box>
    </Box>
  );
}

export default ScanProgress;
