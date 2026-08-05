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
import Accordion from '@mui/material/Accordion';
import AccordionDetails from '@mui/material/AccordionDetails';
import AccordionSummary from '@mui/material/AccordionSummary';
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
import Link from '@mui/material/Link';
import Paper from '@mui/material/Paper';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  getVulnConfig,
  ImageResult,
  runVulnScan,
  setVulnConfig,
  VulnConfig,
  VulnReport,
  vulnScanStatus,
} from '../../lib/cluster-doctor-vulnscan-api';
import { useCluster } from '../../lib/k8s';

const SEVERITIES = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'UNKNOWN'];
const SEV_COLOR: Record<string, string> = {
  CRITICAL: '#b91c1c',
  HIGH: '#ea580c',
  MEDIUM: '#ca8a04',
  LOW: '#2563eb',
  UNKNOWN: '#6b7280',
};

function SevChip({ sev, n }: { sev: string; n: number }) {
  if (!n) return null;
  return (
    <Box
      component="span"
      sx={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 0.5,
        px: 0.75,
        py: 0.25,
        mr: 0.5,
        borderRadius: 1,
        fontSize: 12,
        fontWeight: 600,
        color: '#fff',
        bgcolor: SEV_COLOR[sev],
      }}
    >
      {sev.charAt(0)} {n}
    </Box>
  );
}

function ConfigDialog({
  open,
  config,
  onClose,
  onSaved,
}: {
  open: boolean;
  config: VulnConfig | null;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [image, setImage] = React.useState('');
  const [dbRepo, setDbRepo] = React.useState('');
  const [saving, setSaving] = React.useState(false);
  const [err, setErr] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!open || !config) return;
    setImage(config.image);
    setDbRepo(config.dbRepository ?? '');
    setErr(null);
  }, [open, config]);

  async function save() {
    setSaving(true);
    setErr(null);
    try {
      await setVulnConfig({ image: image.trim(), dbRepository: dbRepo.trim() });
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
      <DialogTitle>Scanner settings</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>
          The Trivy image runs each scan as an in-cluster Job. For air-gapped clusters, point the{' '}
          <strong>DB repository</strong> at your internal mirror so Trivy can load its vulnerability
          database without internet access.
        </DialogContentText>
        {err && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {err}
          </Alert>
        )}
        <TextField
          fullWidth
          label="Trivy image"
          value={image}
          onChange={e => setImage(e.target.value)}
          placeholder="aquasec/trivy:latest"
          sx={{ mb: 2 }}
        />
        <TextField
          fullWidth
          label="DB repository (optional, for air-gap)"
          value={dbRepo}
          onChange={e => setDbRepo(e.target.value)}
          placeholder="registry.internal/aquasecurity/trivy-db"
          helperText="Sets TRIVY_DB_REPOSITORY in the scan Job."
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

function ImageRow({ img }: { img: ImageResult }) {
  const total = SEVERITIES.reduce((n, s) => n + (img.counts[s] || 0), 0);

  return (
    <Accordion disableGutters sx={{ '&:before': { display: 'none' } }}>
      <AccordionSummary expandIcon={img.vulns.length ? <Icon icon="mdi:chevron-down" /> : <Box sx={{ width: 24 }} />}>
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, width: '100%', flexWrap: 'wrap' }}>
          <Typography variant="body2" sx={{ fontFamily: 'monospace', flex: 1, wordBreak: 'break-all' }}>
            {img.image}
          </Typography>
          {img.error ? (
            <Chip size="small" color="default" variant="outlined" label="scan failed" />
          ) : total === 0 ? (
            <Chip size="small" color="success" variant="outlined" label="clean" />
          ) : (
            <Box>
              {SEVERITIES.map(s => (
                <SevChip key={s} sev={s} n={img.counts[s] || 0} />
              ))}
            </Box>
          )}
        </Box>
      </AccordionSummary>
      {(img.vulns.length > 0 || img.error) && (
        <AccordionDetails sx={{ pt: 0 }}>
          <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mb: 1 }}>
            Runs in: {img.namespaces.join(', ') || '—'}
          </Typography>
          {img.error ? (
            <Alert severity="warning">{img.error}</Alert>
          ) : (
            <Table size="small">
              <TableHead>
                <TableRow>
                  <TableCell>Severity</TableCell>
                  <TableCell>Vulnerability</TableCell>
                  <TableCell>Package</TableCell>
                  <TableCell>Installed → Fixed</TableCell>
                </TableRow>
              </TableHead>
              <TableBody>
                {img.vulns.map((v, i) => (
                  <TableRow key={`${v.vulnId}-${i}`}>
                    <TableCell>
                      <SevChip sev={v.severity} n={1} />
                    </TableCell>
                    <TableCell>
                      <Link
                        href={`https://nvd.nist.gov/vuln/detail/${v.vulnId}`}
                        target="_blank"
                        rel="noopener"
                      >
                        {v.vulnId}
                      </Link>
                      {v.title && (
                        <Typography variant="caption" color="text.secondary" sx={{ display: 'block' }}>
                          {v.title}
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>{v.pkgName}</TableCell>
                    <TableCell>
                      {v.installedVersion}
                      {v.fixedVersion ? ` → ${v.fixedVersion}` : ' (no fix)'}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </AccordionDetails>
      )}
    </Accordion>
  );
}

export default function VulnScanPage() {
  const cluster = useCluster();
  const [config, setConfig] = React.useState<VulnConfig | null>(null);
  const [configOpen, setConfigOpen] = React.useState(false);
  const [phase, setPhase] = React.useState<string | null>(null);
  const [report, setReport] = React.useState<VulnReport | null>(null);
  const [error, setError] = React.useState<string | null>(null);
  const [scanning, setScanning] = React.useState(false);

  const loadConfig = React.useCallback(() => {
    getVulnConfig()
      .then(setConfig)
      .catch(() => {});
  }, []);

  React.useEffect(() => {
    loadConfig();
    setReport(null);
    setPhase(null);
    setError(null);
  }, [loadConfig, cluster]);

  async function scan() {
    if (!cluster) return;
    setScanning(true);
    setReport(null);
    setError(null);
    setPhase('Starting');
    try {
      const { runId } = await runVulnScan(cluster);
      let stop = false;
      const poll = async () => {
        if (stop) return;
        try {
          const st = await vulnScanStatus(cluster, runId);
          setPhase(st.phase);
          if (st.finished) {
            setReport(st.report ?? { totals: {}, images: [] });
            setScanning(false);
            return;
          }
        } catch {
          // transient; keep polling
        }
        setTimeout(poll, 3000);
      };
      poll();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setScanning(false);
      setPhase(null);
    }
  }

  return (
    <Box sx={{ p: 3 }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 0.5 }}>
        <Icon icon="mdi:bug-outline" width={28} />
        <Typography variant="h4">Vulnerabilities</Typography>
        <Box sx={{ flex: 1 }} />
        <Tooltip title="Scanner settings">
          <IconButton onClick={() => setConfigOpen(true)}>
            <Icon icon="mdi:cog-outline" />
          </IconButton>
        </Tooltip>
      </Box>
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        Scan the container images running on <strong>{cluster}</strong> for known CVEs with Trivy —
        executed as an in-cluster Job, fully offline when the DB is mirrored.
      </Typography>

      {config?.airGapped && (
        <Alert
          severity="warning"
          sx={{ mb: 2 }}
          action={
            <Button color="inherit" size="small" onClick={() => setConfigOpen(true)}>
              Set mirror
            </Button>
          }
        >
          This K8sense is air-gapped. Set a mirrored <strong>DB repository</strong> and ensure the
          Trivy image <code>{config.image}</code> is pullable, or scans will fail to load the
          vulnerability database.
        </Alert>
      )}

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mb: 2 }}>
        <Button
          variant="contained"
          startIcon={scanning ? <CircularProgress size={16} color="inherit" /> : <Icon icon="mdi:magnify-scan" />}
          onClick={scan}
          disabled={scanning || !cluster}
        >
          {scanning ? `Scanning… (${phase})` : 'Scan running images'}
        </Button>
        {report && (
          <Box>
            {SEVERITIES.map(s => (
              <SevChip key={s} sev={s} n={report.totals[s] || 0} />
            ))}
            {SEVERITIES.every(s => !report.totals[s]) && (
              <Chip size="small" color="success" variant="outlined" label="No known CVEs" />
            )}
          </Box>
        )}
      </Box>

      {report && (
        <Paper variant="outlined">
          {report.images.length === 0 ? (
            <Box sx={{ p: 2 }}>
              <Typography color="text.secondary">No images scanned.</Typography>
            </Box>
          ) : (
            report.images.map(img => <ImageRow key={img.image} img={img} />)
          )}
        </Paper>
      )}

      {report && (
        <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 2 }}>
          Images are pulled and scanned by the in-cluster Job. Private registries may require the
          scan namespace to carry pull credentials; unreachable images show as “scan failed”.
        </Typography>
      )}

      <ConfigDialog
        open={configOpen}
        config={config}
        onClose={() => setConfigOpen(false)}
        onSaved={loadConfig}
      />
    </Box>
  );
}
