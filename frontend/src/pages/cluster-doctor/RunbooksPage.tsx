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
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import IconButton from '@mui/material/IconButton';
import Paper from '@mui/material/Paper';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  enableRunbooks,
  listRunbooks,
  RunbooksStatus,
  RunbookTemplate,
  runLogs,
  runRunbook,
  runbooksStatus,
  setRunnerImage,
} from '../../lib/cluster-doctor-runbooks-api';
import { useCluster } from '../../lib/k8s';

// ---- Runner image (air-gap) settings dialog ----
function RunnerImageDialog({
  open,
  current,
  onClose,
  onSaved,
}: {
  open: boolean;
  current: string;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [image, setImage] = React.useState(current);
  const [saving, setSaving] = React.useState(false);
  const [err, setErr] = React.useState<string | null>(null);

  React.useEffect(() => setImage(current), [current, open]);

  async function save() {
    setSaving(true);
    setErr(null);
    try {
      await setRunnerImage(image.trim());
      onSaved();
      onClose();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Runner image</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>
          The image used for each run. It must contain the <code>kubernetes.core</code> Ansible
          collection and the Python <code>kubernetes</code> library. For air-gapped clusters, point
          this at your internal registry mirror.
        </DialogContentText>
        {err && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {err}
          </Alert>
        )}
        <TextField
          fullWidth
          label="Image"
          value={image}
          onChange={e => setImage(e.target.value)}
          placeholder="quay.io/ansible/ansible-runner:latest"
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={save} disabled={saving || !image.trim()}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// ---- Run dialog: variables form + dry-run/run ----
function RunDialog({
  template,
  onClose,
  onStart,
}: {
  template: RunbookTemplate | null;
  onClose: () => void;
  onStart: (vars: Record<string, string>, check: boolean) => void;
}) {
  const [vars, setVars] = React.useState<Record<string, string>>({});

  React.useEffect(() => {
    if (!template) return;
    const initial: Record<string, string> = {};
    template.vars.forEach(v => (initial[v.name] = v.default ?? ''));
    setVars(initial);
  }, [template]);

  if (!template) return null;

  const missing = template.vars.some(v => v.required && !vars[v.name]?.trim());

  return (
    <Dialog open={!!template} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>{template.name}</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>{template.description}</DialogContentText>
        {template.vars.map(v => (
          <TextField
            key={v.name}
            fullWidth
            required={v.required}
            label={v.label}
            value={vars[v.name] ?? ''}
            onChange={e => setVars(prev => ({ ...prev, [v.name]: e.target.value }))}
            sx={{ mb: 2 }}
          />
        ))}
        <Typography variant="caption" color="text.secondary">
          Dry run executes the playbook in <code>--check</code> mode — it reports what would change
          without applying anything.
        </Typography>
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button
          startIcon={<Icon icon="mdi:eye-check-outline" />}
          disabled={missing}
          onClick={() => onStart(vars, true)}
        >
          Dry run
        </Button>
        <Button
          variant="contained"
          startIcon={<Icon icon="mdi:play" />}
          disabled={missing}
          onClick={() => onStart(vars, false)}
        >
          Run
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// ---- Live run output (polls logs until finished) ----
function RunOutput({
  cluster,
  runId,
  check,
  onClose,
}: {
  cluster: string;
  runId: string;
  check: boolean;
  onClose: () => void;
}) {
  const [phase, setPhase] = React.useState('Pending');
  const [logs, setLogs] = React.useState('');
  const [finished, setFinished] = React.useState(false);

  React.useEffect(() => {
    let stop = false;
    const tick = async () => {
      try {
        const r = await runLogs(cluster, runId);
        if (stop) return;
        setPhase(r.phase);
        setLogs(r.logs);
        if (r.finished) {
          setFinished(true);
          return;
        }
      } catch {
        // transient; keep polling
      }
      if (!stop) setTimeout(tick, 2000);
    };
    tick();
    return () => {
      stop = true;
    };
  }, [cluster, runId]);

  const color =
    phase === 'Succeeded' ? 'success' : phase === 'Failed' ? 'error' : 'default';

  return (
    <Dialog open maxWidth="md" fullWidth onClose={finished ? onClose : undefined}>
      <DialogTitle sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
        {check ? 'Dry run' : 'Run'} output
        <Chip size="small" color={color as any} label={phase} />
        {!finished && <CircularProgress size={16} />}
      </DialogTitle>
      <DialogContent>
        <Box
          component="pre"
          sx={{
            bgcolor: '#0f172a',
            color: '#e2e8f0',
            p: 2,
            borderRadius: 1,
            fontSize: 12.5,
            lineHeight: 1.5,
            minHeight: 240,
            maxHeight: 460,
            overflow: 'auto',
            whiteSpace: 'pre-wrap',
            m: 0,
          }}
        >
          {logs || 'Waiting for the runner pod to start…'}
        </Box>
      </DialogContent>
      <DialogActions>
        <Button variant="contained" disabled={!finished} onClick={onClose}>
          {finished ? 'Close' : 'Running…'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

// ---- Page ----
export default function RunbooksPage() {
  const cluster = useCluster();
  const [status, setStatus] = React.useState<RunbooksStatus | null>(null);
  const [templates, setTemplates] = React.useState<RunbookTemplate[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [enabling, setEnabling] = React.useState(false);
  const [imageDialog, setImageDialog] = React.useState(false);
  const [runTarget, setRunTarget] = React.useState<RunbookTemplate | null>(null);
  const [activeRun, setActiveRun] = React.useState<{ runId: string; check: boolean } | null>(null);

  const loadStatus = React.useCallback(() => {
    if (!cluster) return;
    runbooksStatus(cluster)
      .then(setStatus)
      .catch(e => setError(e instanceof Error ? e.message : String(e)));
  }, [cluster]);

  React.useEffect(() => {
    setStatus(null);
    setError(null);
    loadStatus();
    listRunbooks()
      .then(r => setTemplates(r.runbooks))
      .catch(() => {});
  }, [loadStatus]);

  async function doEnable() {
    if (!cluster) return;
    setEnabling(true);
    setError(null);
    try {
      const s = await enableRunbooks(cluster);
      setStatus(s);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setEnabling(false);
    }
  }

  async function startRun(vars: Record<string, string>, check: boolean) {
    if (!cluster || !runTarget) return;
    const template = runTarget;
    setRunTarget(null);
    setError(null);
    try {
      const { runId } = await runRunbook(cluster, template.id, vars, check);
      setActiveRun({ runId, check });
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 0.5 }}>
        <Icon icon="mdi:script-text-play-outline" width={28} />
        <Typography variant="h4">Runbooks</Typography>
        <Box sx={{ flex: 1 }} />
        <Tooltip title="Runner image">
          <IconButton onClick={() => setImageDialog(true)}>
            <Icon icon="mdi:cog-outline" />
          </IconButton>
        </Tooltip>
      </Box>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        Governed Ansible automation on <strong>{cluster}</strong>. Each run executes as a Kubernetes
        Job in the cluster — nothing runs on your machine — and every run is audited.
      </Typography>

      {status?.airGapped && (
        <Alert severity="warning" sx={{ mb: 2 }} action={
          <Button color="inherit" size="small" onClick={() => setImageDialog(true)}>
            Set image
          </Button>
        }>
          This K8sense is in air-gapped mode. The runner image <code>{status.runnerImage}</code>{' '}
          must be mirrored to your internal registry and set here, or runs will fail to pull it.
        </Alert>
      )}

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {!status && !error && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {status && !status.enabled && (
        <Paper variant="outlined" sx={{ p: 3, maxWidth: 640 }}>
          <Typography variant="h6" sx={{ mb: 1 }}>
            Enable Runbooks on this cluster
          </Typography>
          <Typography variant="body2" color="text.secondary" sx={{ mb: 2 }}>
            This creates a <code>{status.namespace}</code> namespace, a{' '}
            <code>k8sense-runner</code> ServiceAccount, and a scoped ClusterRole. Runbooks then
            execute as Jobs using that ServiceAccount — so what they can do is bounded by its RBAC.
            Your account needs permission to create namespaces and RBAC.
          </Typography>
          <Button variant="contained" onClick={doEnable} disabled={enabling || !cluster}>
            {enabling ? 'Enabling…' : 'Enable on this cluster'}
          </Button>
        </Paper>
      )}

      {status?.enabled && (
        <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 2 }}>
          {templates.map(t => (
            <Paper
              key={t.id}
              variant="outlined"
              sx={{ p: 2, width: 340, display: 'flex', flexDirection: 'column', gap: 1 }}
            >
              <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
                <Icon icon={t.icon} width={26} />
                <Typography variant="subtitle1" sx={{ fontWeight: 600 }}>
                  {t.name}
                </Typography>
              </Box>
              <Typography variant="body2" color="text.secondary" sx={{ flex: 1, minHeight: 56 }}>
                {t.description}
              </Typography>
              <Box>
                <Chip size="small" variant="outlined" label={t.category} sx={{ mr: 1 }} />
                <Button size="small" variant="contained" onClick={() => setRunTarget(t)}>
                  Configure & run
                </Button>
              </Box>
            </Paper>
          ))}
        </Box>
      )}

      <RunDialog template={runTarget} onClose={() => setRunTarget(null)} onStart={startRun} />

      {activeRun && cluster && (
        <RunOutput
          cluster={cluster}
          runId={activeRun.runId}
          check={activeRun.check}
          onClose={() => {
            setActiveRun(null);
            loadStatus();
          }}
        />
      )}

      <RunnerImageDialog
        open={imageDialog}
        current={status?.runnerImage ?? ''}
        onClose={() => setImageDialog(false)}
        onSaved={loadStatus}
      />
    </Box>
  );
}
