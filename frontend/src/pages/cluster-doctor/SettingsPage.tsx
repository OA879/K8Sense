import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Divider from '@mui/material/Divider';
import Paper from '@mui/material/Paper';
import Typography from '@mui/material/Typography';
import React from 'react';
import { BrandingSettings } from '../../components/cluster-doctor/BrandingSettings';
import { LicenceStatus } from '../../components/cluster-doctor/LicenceStatus';
import { NotificationSettings } from '../../components/cluster-doctor/NotificationSettings';
import { useBranding } from '../../lib/cluster-doctor-branding-api';
import { useCluster } from '../../lib/k8s';
import {
  StorageStats,
  TestConnResult,
  getStorageStats,
  purgeScans,
  testConnection,
} from '../../lib/cluster-doctor-settings-api';

function formatBytes(n: number): string {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / (1024 * 1024)).toFixed(1)} MB`;
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <Paper sx={{ p: 3, mb: 3 }}>
      <Typography variant="h6" gutterBottom>
        {title}
      </Typography>
      <Divider sx={{ mb: 2 }} />
      {children}
    </Paper>
  );
}

export default function SettingsPage() {
  const cluster = useCluster();
  const branding = useBranding();
  const [storage, setStorage] = React.useState<StorageStats | null>(null);
  const [conn, setConn] = React.useState<TestConnResult | null>(null);
  const [busy, setBusy] = React.useState(false);
  const [msg, setMsg] = React.useState<string | null>(null);

  const refreshStorage = React.useCallback(() => {
    getStorageStats()
      .then(setStorage)
      .catch(() => undefined);
  }, []);

  React.useEffect(refreshStorage, [refreshStorage]);

  async function handlePurge() {
    setBusy(true);
    setMsg(null);
    try {
      const { pruned } = await purgeScans(10);
      setMsg(`Purged ${pruned} old scan${pruned === 1 ? '' : 's'}.`);
      refreshStorage();
    } catch (e) {
      setMsg(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function handleTestConn() {
    if (!cluster) return;
    setConn(null);
    setConn(await testConnection(cluster).catch(e => ({ reachable: false, error: String(e) })));
  }

  return (
    <Box sx={{ p: 3, maxWidth: 780 }}>
      <Typography variant="h4" gutterBottom>
        {branding.productName} Settings
      </Typography>

      <Section title="Licence">
        <LicenceStatus />
      </Section>

      <Section title="Cluster Connection">
        <Typography variant="body2" color="text.secondary" gutterBottom>
          Test connectivity to <strong>{cluster}</strong>.
        </Typography>
        <Button size="small" variant="outlined" onClick={handleTestConn}>
          Test Connection
        </Button>
        {conn && (
          <Alert severity={conn.reachable ? 'success' : 'error'} sx={{ mt: 2 }}>
            {conn.reachable
              ? `Reachable — Kubernetes ${conn.k8sVersion} (${conn.latencyMs} ms)`
              : `Unreachable: ${conn.error}`}
          </Alert>
        )}
      </Section>

      <Section title="Notifications & Scheduled Scans">
        {cluster && <NotificationSettings cluster={cluster} />}
      </Section>

      <Section title="Storage">
        {storage ? (
          <Typography variant="body2" color="text.secondary">
            Database {formatBytes(storage.dbSizeBytes)} · {storage.scanCount} scans ·{' '}
            {storage.findingCount} findings · {storage.auditCount} audit entries
          </Typography>
        ) : (
          <Typography variant="body2" color="text.secondary">
            Loading…
          </Typography>
        )}
        <Button size="small" variant="outlined" sx={{ mt: 2 }} onClick={handlePurge} disabled={busy}>
          Purge old scans (keep 10 per cluster)
        </Button>
        {msg && (
          <Alert severity="info" sx={{ mt: 2 }}>
            {msg}
          </Alert>
        )}
      </Section>

      <Section title="Branding & Access">
        <BrandingSettings />
      </Section>

      <Section title="About">
        <Typography variant="body2" color="text.secondary">
          {branding.productName} — Kubernetes operations platform with built-in Cluster Doctor
          diagnostics. Powered by open-source components (see NOTICE). All scans run locally; no
          cluster data leaves your machine.
        </Typography>
      </Section>
    </Box>
  );
}
