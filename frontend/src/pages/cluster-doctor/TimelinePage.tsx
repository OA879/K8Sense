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
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import FormControl from '@mui/material/FormControl';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  getTimeline,
  TimelineEntry,
  TimelineLevel,
  TimelineType,
} from '../../lib/cluster-doctor-timeline-api';
import { useCluster } from '../../lib/k8s';

const LEVEL_COLOR: Record<TimelineLevel, string> = {
  critical: '#ef4444',
  warning: '#f59e0b',
  info: '#64748b',
};

const TYPE_META: Record<TimelineType, { icon: string; label: string }> = {
  action: { icon: 'mdi:gesture-tap-button', label: 'Action' },
  event: { icon: 'mdi:flash', label: 'Cluster event' },
  finding: { icon: 'mdi:stethoscope', label: 'Finding' },
};

const WINDOWS = [
  { label: 'Last 15 min', value: 15 },
  { label: 'Last hour', value: 60 },
  { label: 'Last 6 hours', value: 360 },
  { label: 'Last 24 hours', value: 1440 },
];

function relativeTime(unixSeconds: number): string {
  const secs = Math.max(0, Math.floor(Date.now() / 1000 - unixSeconds));
  if (secs < 60) return `${secs}s ago`;
  if (secs < 3600) return `${Math.floor(secs / 60)}m ago`;
  if (secs < 86400) return `${Math.floor(secs / 3600)}h ago`;
  return `${Math.floor(secs / 86400)}d ago`;
}

function TimelineRow({ entry }: { entry: TimelineEntry }) {
  const color = LEVEL_COLOR[entry.level];
  const meta = TYPE_META[entry.type];
  const resource = [entry.namespace, entry.resourceName].filter(Boolean).join('/');

  return (
    <Box sx={{ display: 'flex', gap: 1.5, alignItems: 'flex-start', py: 1 }}>
      {/* rail: icon dot + connecting line */}
      <Box sx={{ display: 'flex', flexDirection: 'column', alignItems: 'center', minWidth: 28 }}>
        <Box
          sx={{
            width: 28,
            height: 28,
            borderRadius: '50%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            border: `2px solid ${color}`,
            color,
          }}
        >
          <Icon icon={meta.icon} width={16} height={16} />
        </Box>
        <Box sx={{ flex: 1, width: 2, bgcolor: 'divider', mt: 0.5, minHeight: 8 }} />
      </Box>

      <Box sx={{ flex: 1, minWidth: 0 }}>
        <Box sx={{ display: 'flex', alignItems: 'baseline', gap: 1, flexWrap: 'wrap' }}>
          <Typography variant="subtitle2" sx={{ fontWeight: 600 }}>
            {entry.title}
          </Typography>
          <Typography variant="caption" color="text.secondary" title={new Date(entry.time * 1000).toLocaleString()}>
            {relativeTime(entry.time)}
          </Typography>
          <Chip size="small" variant="outlined" label={meta.label} sx={{ height: 18, fontSize: 10 }} />
        </Box>
        {entry.detail && (
          <Typography variant="body2" color="text.secondary" sx={{ whiteSpace: 'pre-wrap', mt: 0.25 }}>
            {entry.detail}
          </Typography>
        )}
        <Box sx={{ display: 'flex', gap: 1, mt: 0.25, flexWrap: 'wrap' }}>
          {resource && (
            <Typography variant="caption" color="text.secondary">
              {entry.resourceKind ? `${entry.resourceKind} ` : ''}
              {resource}
            </Typography>
          )}
          {entry.actor && (
            <Typography variant="caption" color="text.secondary">
              · by {entry.actor}
            </Typography>
          )}
        </Box>
      </Box>
    </Box>
  );
}

export default function TimelinePage() {
  const cluster = useCluster();
  const [entries, setEntries] = React.useState<TimelineEntry[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [namespace, setNamespace] = React.useState('');
  const [resource, setResource] = React.useState('');
  const [windowMin, setWindowMin] = React.useState(60);

  const load = React.useCallback(() => {
    if (!cluster) return;

    setLoading(true);
    setError(null);

    getTimeline(cluster, { namespace, resource, sinceMinutes: windowMin })
      .then(r => setEntries(r.entries ?? []))
      .catch(e => setError(e instanceof Error ? e.message : String(e)))
      .finally(() => setLoading(false));
  }, [cluster, namespace, resource, windowMin]);

  React.useEffect(() => {
    load();
  }, [load]);

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4">What changed?</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        A single timeline for <strong>{cluster}</strong> — human actions, cluster events, and scan
        findings merged newest-first, so you can see what happened around an incident.
      </Typography>

      <Box sx={{ display: 'flex', gap: 2, mb: 2, flexWrap: 'wrap', alignItems: 'center' }}>
        <FormControl size="small" sx={{ minWidth: 150 }}>
          <InputLabel>Window</InputLabel>
          <Select label="Window" value={windowMin} onChange={e => setWindowMin(Number(e.target.value))}>
            {WINDOWS.map(w => (
              <MenuItem key={w.value} value={w.value}>
                {w.label}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <TextField
          size="small"
          label="Namespace"
          value={namespace}
          onChange={e => setNamespace(e.target.value)}
          sx={{ width: 160 }}
        />
        <TextField
          size="small"
          label="Resource contains"
          value={resource}
          onChange={e => setResource(e.target.value)}
          sx={{ width: 200 }}
        />
        <Button size="small" variant="outlined" onClick={load} startIcon={<Icon icon="mdi:refresh" />}>
          Refresh
        </Button>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {loading && !entries && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {entries && entries.length === 0 && (
        <Alert severity="info">Nothing recorded in this window. Widen the window or run a scan.</Alert>
      )}

      {entries && entries.length > 0 && (
        <Box>
          {entries.map((e, i) => (
            <TimelineRow key={`${e.time}-${e.type}-${i}`} entry={e} />
          ))}
        </Box>
      )}
    </Box>
  );
}
