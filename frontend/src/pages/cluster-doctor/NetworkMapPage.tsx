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

import { Background, Controls, Edge, MarkerType, MiniMap, Node, ReactFlow } from '@xyflow/react';
import '@xyflow/react/dist/base.css';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import FormControl from '@mui/material/FormControl';
import FormControlLabel from '@mui/material/FormControlLabel';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Switch from '@mui/material/Switch';
import Typography from '@mui/material/Typography';
import React from 'react';
import { Exposure, getNetworkMap, NetworkMap } from '../../lib/cluster-doctor-network-api';
import { useCluster } from '../../lib/k8s';

// Exposure colours: red = reachable by anything (a security concern),
// green = restricted by policy, blue = deny-all, grey = external.
const EXPOSURE_COLOR: Record<Exposure, string> = {
  open: '#ef4444',
  restricted: '#22c55e',
  isolated: '#3b82f6',
  external: '#9ca3af',
};

const EXPOSURE_LABEL: Record<Exposure, string> = {
  open: 'Open — reachable by anything',
  restricted: 'Restricted by policy',
  isolated: 'Isolated (deny-all)',
  external: 'External / mesh-only',
};

const COL_WIDTH = 300;
const ROW_HEIGHT = 80;

/** Lays out nodes in one column per namespace so cross-namespace edges read left-to-right. */
function useGraph(map: NetworkMap | null, showAllowed: boolean, showLive: boolean) {
  return React.useMemo(() => {
    if (!map) {
      return { nodes: [] as Node[], edges: [] as Edge[] };
    }

    const columns = new Map<string, number>();
    map.namespaces.forEach((ns, i) => columns.set(ns, i));

    const rowOf = new Map<string, number>();

    const nodes: Node[] = map.nodes.map(n => {
      const col = columns.get(n.namespace) ?? 0;
      const row = rowOf.get(n.namespace) ?? 0;
      rowOf.set(n.namespace, row + 1);

      const color = EXPOSURE_COLOR[n.exposure];

      return {
        id: n.id,
        position: { x: col * COL_WIDTH, y: row * ROW_HEIGHT },
        data: { label: `${n.name}\n${n.namespace} · ${n.kind}` },
        style: {
          border: `2px solid ${color}`,
          borderRadius: 8,
          padding: 6,
          fontSize: 11,
          whiteSpace: 'pre-line',
          width: COL_WIDTH - 60,
          background: 'var(--xy-node-background-color, #1e293b)',
        },
      };
    });

    const edges: Edge[] = [];

    if (showAllowed) {
      map.edges.forEach(e => {
        edges.push({
          id: e.id,
          source: e.source,
          target: e.target,
          label: e.ports || undefined,
          style: { stroke: '#64748b', strokeWidth: 1.5 },
          markerEnd: { type: MarkerType.ArrowClosed, color: '#64748b' },
        });
      });
    }

    if (showLive && map.mesh.enabled) {
      map.traffic.forEach(t => {
        edges.push({
          id: t.id,
          source: t.source,
          target: t.target,
          label: `${t.rps.toFixed(1)} req/s`,
          animated: true,
          style: { stroke: '#a855f7', strokeWidth: 2 },
          markerEnd: { type: MarkerType.ArrowClosed, color: '#a855f7' },
        });
      });
    }

    return { nodes, edges };
  }, [map, showAllowed, showLive]);
}

function LegendDot({ color, label }: { color: string; label: string }) {
  return (
    <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
      <Box sx={{ width: 12, height: 12, borderRadius: '3px', border: `2px solid ${color}` }} />
      <Typography variant="caption">{label}</Typography>
    </Box>
  );
}

export default function NetworkMapPage() {
  const cluster = useCluster();
  const [namespace, setNamespace] = React.useState<string>('');
  const [map, setMap] = React.useState<NetworkMap | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [loading, setLoading] = React.useState(false);
  const [showAllowed, setShowAllowed] = React.useState(true);
  const [showLive, setShowLive] = React.useState(true);

  React.useEffect(() => {
    if (!cluster) return;

    let cancelled = false;
    setLoading(true);
    setError(null);

    getNetworkMap(cluster, namespace || undefined)
      .then(result => {
        if (!cancelled) setMap(result);
      })
      .catch(e => {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [cluster, namespace]);

  const { nodes, edges } = useGraph(map, showAllowed, showLive);

  return (
    <Box sx={{ p: 3, height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Typography variant="h4">Network Map</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        What can reach what in <strong>{cluster}</strong>, derived from NetworkPolicies. Node colour
        shows ingress exposure; grey arrows are policy-allowed connections.
      </Typography>

      {/* Mesh status banner */}
      {map &&
        (map.mesh.enabled ? (
          <Alert severity="success" sx={{ mb: 2 }}>
            Live traffic detected — {map.mesh.source}. Purple animated arrows show observed request
            rates.
          </Alert>
        ) : (
          <Alert severity="info" sx={{ mb: 2 }}>
            No service mesh detected. Showing the policy/topology view. Live request flow needs a
            mesh (Istio) with a reachable Prometheus.
          </Alert>
        ))}

      {error && (
        <Alert severity="error" sx={{ mb: 2 }}>
          {error}
        </Alert>
      )}

      {/* Controls */}
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2, flexWrap: 'wrap' }}>
        <FormControl size="small" sx={{ minWidth: 200 }}>
          <InputLabel>Namespace</InputLabel>
          <Select
            label="Namespace"
            value={namespace}
            onChange={e => setNamespace(e.target.value)}
          >
            <MenuItem value="">All namespaces</MenuItem>
            {(map?.namespaces ?? []).map(ns => (
              <MenuItem key={ns} value={ns}>
                {ns}
              </MenuItem>
            ))}
          </Select>
        </FormControl>
        <FormControlLabel
          control={<Switch checked={showAllowed} onChange={e => setShowAllowed(e.target.checked)} />}
          label="Allowed connections"
        />
        <FormControlLabel
          control={
            <Switch
              checked={showLive}
              disabled={!map?.mesh.enabled}
              onChange={e => setShowLive(e.target.checked)}
            />
          }
          label="Live traffic"
        />
        <Box sx={{ flex: 1 }} />
        <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
          {(['open', 'restricted', 'isolated', 'external'] as Exposure[]).map(k => (
            <LegendDot key={k} color={EXPOSURE_COLOR[k]} label={EXPOSURE_LABEL[k]} />
          ))}
        </Box>
      </Box>

      <Box sx={{ flex: 1, minHeight: 400, border: theme => `1px solid ${theme.palette.divider}` }}>
        {loading && !map ? (
          <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100%' }}>
            <CircularProgress />
          </Box>
        ) : nodes.length === 0 ? (
          <Box sx={{ p: 4, textAlign: 'center' }}>
            <Typography color="text.secondary">
              No workloads found for this scope. Try “All namespaces”.
            </Typography>
          </Box>
        ) : (
          <ReactFlow nodes={nodes} edges={edges} fitView minZoom={0.1}>
            <Background />
            <Controls showInteractive={false} />
            <MiniMap pannable zoomable />
          </ReactFlow>
        )}
      </Box>

      {map && (
        <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
          <Chip size="small" label={`${map.nodes.length} workloads`} sx={{ mr: 1 }} />
          <Chip size="small" label={`${map.edges.length} allowed connections`} sx={{ mr: 1 }} />
          {map.mesh.enabled && <Chip size="small" label={`${map.traffic.length} live flows`} />}
        </Typography>
      )}
    </Box>
  );
}
