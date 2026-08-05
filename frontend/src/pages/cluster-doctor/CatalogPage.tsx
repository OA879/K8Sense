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
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  CatalogApp,
  getCatalog,
  installApp,
  uninstallApp,
} from '../../lib/cluster-doctor-catalog-api';
import { useCluster } from '../../lib/k8s';

function groupByCategory(apps: CatalogApp[]): Record<string, CatalogApp[]> {
  const groups: Record<string, CatalogApp[]> = {};
  for (const a of apps) {
    (groups[a.category] ??= []).push(a);
  }
  return groups;
}

function AppCard({
  app,
  busy,
  onInstall,
  onUninstall,
}: {
  app: CatalogApp;
  busy: boolean;
  onInstall: () => void;
  onUninstall: () => void;
}) {
  return (
    <Paper
      variant="outlined"
      sx={{ p: 2, width: 320, display: 'flex', flexDirection: 'column', gap: 1 }}
    >
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5 }}>
        <Icon icon={app.icon} width={30} />
        <Box sx={{ flex: 1 }}>
          <Typography variant="subtitle1" sx={{ fontWeight: 600, lineHeight: 1.2 }}>
            {app.name}
          </Typography>
          <Typography variant="caption" color="text.secondary">
            ns: {app.namespace}
          </Typography>
        </Box>
        {app.installed && (
          <Chip size="small" color="success" variant="outlined" label={app.status || 'installed'} />
        )}
      </Box>

      <Typography variant="body2" color="text.secondary" sx={{ flex: 1, minHeight: 60 }}>
        {app.description}
      </Typography>

      <Box>
        {busy ? (
          <Button size="small" disabled startIcon={<CircularProgress size={14} />}>
            Working…
          </Button>
        ) : app.installed ? (
          <Button size="small" color="error" variant="outlined" onClick={onUninstall}>
            Uninstall
          </Button>
        ) : (
          <Button
            size="small"
            variant="contained"
            startIcon={<Icon icon="mdi:download" />}
            onClick={onInstall}
          >
            Install
          </Button>
        )}
      </Box>
    </Paper>
  );
}

export default function CatalogPage() {
  const cluster = useCluster();
  const [apps, setApps] = React.useState<CatalogApp[] | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [notice, setNotice] = React.useState<string | null>(null);
  const [busyId, setBusyId] = React.useState<string | null>(null);
  const [confirm, setConfirm] = React.useState<CatalogApp | null>(null);

  const load = React.useCallback(() => {
    if (!cluster) return;
    getCatalog(cluster)
      .then(r => setApps(r.apps))
      .catch(e => setError(e instanceof Error ? e.message : String(e)));
  }, [cluster]);

  React.useEffect(() => {
    setApps(null);
    setError(null);
    load();
  }, [load]);

  async function doInstall(app: CatalogApp) {
    if (!cluster) return;
    setBusyId(app.id);
    setError(null);
    setNotice(null);
    try {
      const res = await installApp(cluster, app.id);
      setNotice(res.message);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusyId(null);
    }
  }

  async function doUninstall(app: CatalogApp) {
    if (!cluster) return;
    setConfirm(null);
    setBusyId(app.id);
    setError(null);
    setNotice(null);
    try {
      const res = await uninstallApp(cluster, app.id);
      setNotice(res.message);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusyId(null);
    }
  }

  const groups = apps ? groupByCategory(apps) : {};

  return (
    <Box sx={{ p: 3 }}>
      <Typography variant="h4">App Catalog</Typography>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        One-click install of curated open-source tools onto <strong>{cluster}</strong>, via Helm.
        Uninstall removes the release and its pods.
      </Typography>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}
      {notice && (
        <Alert severity="info" sx={{ mb: 2 }} onClose={() => setNotice(null)}>
          {notice}
        </Alert>
      )}

      {!apps && !error && (
        <Box sx={{ display: 'flex', justifyContent: 'center', py: 6 }}>
          <CircularProgress />
        </Box>
      )}

      {apps &&
        Object.entries(groups).map(([category, list]) => (
          <Box key={category} sx={{ mb: 3 }}>
            <Typography variant="subtitle1" sx={{ fontWeight: 600, mb: 1 }}>
              {category}
            </Typography>
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 2 }}>
              {list.map(app => (
                <AppCard
                  key={app.id}
                  app={app}
                  busy={busyId === app.id}
                  onInstall={() => doInstall(app)}
                  onUninstall={() => setConfirm(app)}
                />
              ))}
            </Box>
          </Box>
        ))}

      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 1 }}>
        Charts install from each project&apos;s official repo; the cluster pulls the images. For
        air-gapped sites, point the repos and image registry at internal mirrors.
      </Typography>

      <Dialog open={!!confirm} onClose={() => setConfirm(null)}>
        <DialogTitle>Uninstall {confirm?.name}?</DialogTitle>
        <DialogContent>
          <DialogContentText>
            This runs <code>helm uninstall</code> on <strong>{cluster}</strong> — it deletes the{' '}
            {confirm?.name} release and all of its pods from namespace{' '}
            <code>{confirm?.namespace}</code>. This cannot be undone.
          </DialogContentText>
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setConfirm(null)}>Cancel</Button>
          <Button color="error" variant="contained" onClick={() => confirm && doUninstall(confirm)}>
            Uninstall
          </Button>
        </DialogActions>
      </Dialog>
    </Box>
  );
}
