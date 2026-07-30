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
import {
  Background,
  Controls,
  Edge,
  MarkerType,
  MiniMap,
  Node,
  NodeMouseHandler,
  ReactFlow,
} from '@xyflow/react';
import '@xyflow/react/dist/base.css';
import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Divider from '@mui/material/Divider';
import FormControl from '@mui/material/FormControl';
import FormControlLabel from '@mui/material/FormControlLabel';
import IconButton from '@mui/material/IconButton';
import InputLabel from '@mui/material/InputLabel';
import MenuItem from '@mui/material/MenuItem';
import Paper from '@mui/material/Paper';
import Select from '@mui/material/Select';
import Switch from '@mui/material/Switch';
import Typography from '@mui/material/Typography';
import React from 'react';
import { Exposure, getNetworkMap, NetNode, NetworkMap } from '../../lib/cluster-doctor-network-api';
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
function useGraph(
  map: NetworkMap | null,
  showAllowed: boolean,
  showLive: boolean,
  showInferred: boolean
) {
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
      const label = n.database
        ? `🗄️ ${n.name}\n${n.dbEngine} · ${n.namespace}`
        : `${n.name}\n${n.namespace} · ${n.kind}`;

      return {
        id: n.id,
        position: { x: col * COL_WIDTH, y: row * ROW_HEIGHT },
        data: { label },
        style: {
          border: `2px solid ${color}`,
          // Databases get a distinct rounded, tinted look so the datastores
          // stand out from application workloads at a glance.
          borderRadius: n.database ? 20 : 8,
          padding: 6,
          fontSize: 11,
          whiteSpace: 'pre-line',
          width: COL_WIDTH - 60,
          background: n.database ? '#312e81' : 'var(--xy-node-background-color, #1e293b)',
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

    if (showInferred) {
      (map.inferred ?? []).forEach(e => {
        edges.push({
          id: e.id,
          source: e.source,
          target: e.target,
          // Dashed teal = "configured to talk to", distinct from policy/mesh.
          style: { stroke: '#14b8a6', strokeWidth: 1.5, strokeDasharray: '5 4' },
          markerEnd: { type: MarkerType.ArrowClosed, color: '#14b8a6' },
        });
      });
    }

    return { nodes, edges };
  }, [map, showAllowed, showLive, showInferred]);
}

/** Side panel shown when a workload node is clicked: its live and policy neighbours. */
function NodeDetail({ node, map, onClose }: { node: NetNode; map: NetworkMap; onClose: () => void }) {
  const nameOf = (id: string) => {
    const n = map.nodes.find(x => x.id === id);
    return n ? `${n.namespace}/${n.name}` : id;
  };

  const allowedFrom = map.edges.filter(e => e.target === node.id);
  const allowedTo = map.edges.filter(e => e.source === node.id);
  const liveFrom = map.traffic.filter(t => t.target === node.id);
  const liveTo = map.traffic.filter(t => t.source === node.id);
  const wiresTo = (map.inferred ?? []).filter(e => e.source === node.id);
  const wiredFrom = (map.inferred ?? []).filter(e => e.target === node.id);

  const section = (title: string, rows: React.ReactNode[]) => (
    <Box sx={{ mb: 1.5 }}>
      <Typography
        variant="caption"
        color="text.secondary"
        sx={{ textTransform: 'uppercase', letterSpacing: 0.5, fontWeight: 600 }}
      >
        {title}
      </Typography>
      {rows.length === 0 ? (
        <Typography variant="body2" color="text.disabled">
          —
        </Typography>
      ) : (
        rows
      )}
    </Box>
  );

  return (
    <Paper
      elevation={6}
      sx={{ position: 'absolute', top: 8, right: 8, width: 300, p: 2, zIndex: 5, maxHeight: '92%', overflow: 'auto' }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', mb: 0.5 }}>
        <Typography variant="subtitle1" sx={{ fontWeight: 700 }}>
          {node.name}
        </Typography>
        <IconButton size="small" onClick={onClose} aria-label="Close">
          <Icon icon="mdi:close" />
        </IconButton>
      </Box>
      <Typography variant="body2" color="text.secondary">
        {node.namespace} · {node.kind}
      </Typography>
      <Box sx={{ display: 'flex', gap: 0.75, flexWrap: 'wrap', mt: 0.75, mb: 1.5 }}>
        <Chip
          size="small"
          variant="outlined"
          label={node.exposure}
          sx={{ borderColor: EXPOSURE_COLOR[node.exposure], color: EXPOSURE_COLOR[node.exposure] }}
        />
        {node.database && (
          <Chip size="small" color="secondary" label={`🗄️ ${node.dbEngine || 'database'}`} />
        )}
      </Box>
      <Divider sx={{ mb: 1.5 }} />

      {map.mesh.enabled && (
        <>
          {section(
            'Live traffic in',
            liveFrom.map(t => (
              <Typography key={t.id} variant="body2">
                ← {nameOf(t.source)} · {t.rps.toFixed(1)} req/s
              </Typography>
            ))
          )}
          {section(
            'Live traffic out',
            liveTo.map(t => (
              <Typography key={t.id} variant="body2">
                → {nameOf(t.target)} · {t.rps.toFixed(1)} req/s
              </Typography>
            ))
          )}
          <Divider sx={{ mb: 1.5 }} />
        </>
      )}

      {section(
        'Allowed to reach it',
        allowedFrom.map(e => (
          <Typography key={e.id} variant="body2">
            ← {nameOf(e.source)}
            {e.ports ? ` · ${e.ports}` : ''}
          </Typography>
        ))
      )}
      {section(
        'It may reach',
        allowedTo.map(e => (
          <Typography key={e.id} variant="body2">
            → {nameOf(e.target)}
            {e.ports ? ` · ${e.ports}` : ''}
          </Typography>
        ))
      )}

      {(wiresTo.length > 0 || wiredFrom.length > 0) && <Divider sx={{ mb: 1.5 }} />}
      {wiresTo.length > 0 &&
        section(
          'Wires to (from config)',
          wiresTo.map(e => (
            <Typography key={e.id} variant="body2" sx={{ color: '#14b8a6' }}>
              ⇢ {nameOf(e.target)} · via {e.via}
            </Typography>
          ))
        )}
      {wiredFrom.length > 0 &&
        section(
          'Wired from (from config)',
          wiredFrom.map(e => (
            <Typography key={e.id} variant="body2" sx={{ color: '#14b8a6' }}>
              ⇠ {nameOf(e.source)}
            </Typography>
          ))
        )}
    </Paper>
  );
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
  const [showInferred, setShowInferred] = React.useState(true);
  const [selected, setSelected] = React.useState<NetNode | null>(null);

  const onNodeClick = React.useCallback<NodeMouseHandler>(
    (_, node) => setSelected(map?.nodes.find(n => n.id === node.id) ?? null),
    [map]
  );

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

  const { nodes, edges } = useGraph(map, showAllowed, showLive, showInferred);

  return (
    <Box sx={{ p: 3, height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Typography variant="h4">Network Map</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        How your workloads connect in <strong>{cluster}</strong>. Node colour shows ingress exposure;
        grey arrows are policy-allowed connections, and dashed teal arrows are connections inferred
        from app config (env / args / ConfigMaps).
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
        <FormControlLabel
          control={<Switch checked={showInferred} onChange={e => setShowInferred(e.target.checked)} />}
          label="Inferred (config)"
        />
        <Box sx={{ flex: 1 }} />
        <Box sx={{ display: 'flex', gap: 2, flexWrap: 'wrap' }}>
          {(['open', 'restricted', 'isolated', 'external'] as Exposure[]).map(k => (
            <LegendDot key={k} color={EXPOSURE_COLOR[k]} label={EXPOSURE_LABEL[k]} />
          ))}
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
            <Typography variant="caption">🗄️ Database</Typography>
          </Box>
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 0.75 }}>
            <Box sx={{ width: 18, borderTop: '2px dashed #14b8a6' }} />
            <Typography variant="caption">Inferred (from config)</Typography>
          </Box>
        </Box>
      </Box>

      <Box
        sx={{
          flex: 1,
          minHeight: 400,
          position: 'relative',
          border: theme => `1px solid ${theme.palette.divider}`,
        }}
      >
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
          <>
            <ReactFlow
              nodes={nodes}
              edges={edges}
              onNodeClick={onNodeClick}
              onPaneClick={() => setSelected(null)}
              fitView
              minZoom={0.1}
            >
              <Background />
              <Controls showInteractive={false} />
              <MiniMap pannable zoomable />
            </ReactFlow>
            {selected && map && (
              <NodeDetail node={selected} map={map} onClose={() => setSelected(null)} />
            )}
          </>
        )}
      </Box>

      {map && (
        <Typography variant="caption" color="text.secondary" sx={{ mt: 1 }}>
          <Chip size="small" label={`${map.nodes.length} workloads`} sx={{ mr: 1 }} />
          <Chip size="small" label={`${map.edges.length} allowed connections`} sx={{ mr: 1 }} />
          <Chip size="small" label={`${(map.inferred ?? []).length} inferred`} sx={{ mr: 1 }} />
          {map.mesh.enabled && <Chip size="small" label={`${map.traffic.length} live flows`} />}
        </Typography>
      )}
    </Box>
  );
}
