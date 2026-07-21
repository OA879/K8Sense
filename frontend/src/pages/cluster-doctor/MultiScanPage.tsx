import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import CircularProgress from '@mui/material/CircularProgress';
import FormControlLabel from '@mui/material/FormControlLabel';
import LinearProgress from '@mui/material/LinearProgress';
import Paper from '@mui/material/Paper';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import Typography from '@mui/material/Typography';
import React from 'react';
import { useHistory } from 'react-router';
import { MultiScanEntry, listHistory, startMultiScan } from '../../lib/cluster-doctor-api';
import { useClustersConf } from '../../lib/k8s';
import { createRouteURL } from '../../lib/router/createRouteURL';

interface ClusterResult {
  cluster: string;
  scanId?: string;
  error?: string;
  status: 'scanning' | 'done' | 'failed';
  total?: number;
  critical?: number;
}

export default function MultiScanPage() {
  const clustersConf = useClustersConf();
  const routerHistory = useHistory();
  const clusterNames = React.useMemo(() => Object.keys(clustersConf ?? {}), [clustersConf]);

  const [selected, setSelected] = React.useState<string[]>([]);
  const [results, setResults] = React.useState<ClusterResult[]>([]);
  const [scanning, setScanning] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    // Default to all clusters selected once the config loads.
    setSelected(clusterNames);
  }, [clusterNames]);

  function toggle(name: string) {
    setSelected(prev => (prev.includes(name) ? prev.filter(n => n !== name) : [...prev, name]));
  }

  // pollResult fetches a cluster's latest scan summary until it completes,
  // so the row can show final counts.
  async function pollResult(entry: MultiScanEntry) {
    if (!entry.scanId) {
      setResults(prev =>
        prev.map(r => (r.cluster === entry.cluster ? { ...r, status: 'failed', error: entry.error } : r))
      );
      return;
    }

    for (let i = 0; i < 40; i++) {
      // eslint-disable-next-line no-await-in-loop
      const history = await listHistory(entry.cluster).catch(() => []);
      const scan = history.find(s => s.id === entry.scanId);
      if (scan && (scan.status === 'completed' || scan.status === 'partial' || scan.status === 'failed')) {
        setResults(prev =>
          prev.map(r =>
            r.cluster === entry.cluster
              ? {
                  ...r,
                  status: scan.status === 'failed' ? 'failed' : 'done',
                  total: scan.totalFindings,
                  critical: scan.criticalCount,
                }
              : r
          )
        );
        return;
      }
      // eslint-disable-next-line no-await-in-loop
      await new Promise(res => setTimeout(res, 1500));
    }
  }

  async function handleScan() {
    if (selected.length === 0) return;

    setScanning(true);
    setError(null);
    setResults(selected.map(c => ({ cluster: c, status: 'scanning' })));

    try {
      const entries = await startMultiScan(selected);
      setResults(
        entries.map(e => ({
          cluster: e.cluster,
          scanId: e.scanId,
          error: e.error,
          status: e.scanId ? 'scanning' : 'failed',
        }))
      );
      await Promise.all(entries.map(pollResult));
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setScanning(false);
    }
  }

  return (
    <Box sx={{ p: 3, maxWidth: 820 }}>
      <Typography variant="h4" gutterBottom>
        Multi-Cluster Scan
      </Typography>
      <Typography color="text.secondary" sx={{ mb: 3 }}>
        Scan several clusters at once (up to 5 run in parallel). Each cluster produces its own
        findings report.
      </Typography>

      <Paper sx={{ p: 3, mb: 3 }}>
        <Typography variant="subtitle1" gutterBottom>
          Clusters
        </Typography>
        {clusterNames.length === 0 && (
          <Typography color="text.secondary">No clusters imported.</Typography>
        )}
        <Box sx={{ display: 'flex', flexDirection: 'column' }}>
          {clusterNames.map(name => (
            <FormControlLabel
              key={name}
              control={<Checkbox checked={selected.includes(name)} onChange={() => toggle(name)} />}
              label={name}
            />
          ))}
        </Box>
        <Button
          variant="contained"
          sx={{ mt: 2 }}
          disabled={scanning || selected.length === 0}
          onClick={handleScan}
        >
          {scanning ? 'Scanning…' : `Scan ${selected.length} cluster${selected.length === 1 ? '' : 's'}`}
        </Button>
        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {error}
          </Alert>
        )}
      </Paper>

      {results.length > 0 && (
        <Paper sx={{ p: 2 }}>
          {scanning && <LinearProgress sx={{ mb: 2 }} />}
          <Table size="small">
            <TableHead>
              <TableRow>
                <TableCell>Cluster</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Critical</TableCell>
                <TableCell align="right">Total</TableCell>
                <TableCell />
              </TableRow>
            </TableHead>
            <TableBody>
              {results.map(r => (
                <TableRow key={r.cluster} hover>
                  <TableCell>{r.cluster}</TableCell>
                  <TableCell>
                    {r.status === 'scanning' ? (
                      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
                        <CircularProgress size={14} /> scanning
                      </Box>
                    ) : (
                      r.status
                    )}
                  </TableCell>
                  <TableCell align="right">{r.critical ?? '—'}</TableCell>
                  <TableCell align="right">{r.total ?? '—'}</TableCell>
                  <TableCell align="right">
                    {r.scanId && r.status === 'done' && (
                      <Button
                        size="small"
                        onClick={() =>
                          routerHistory.push(
                            createRouteURL('clusterDoctorFindings', { scanId: r.scanId })
                          )
                        }
                      >
                        View
                      </Button>
                    )}
                    {r.error && (
                      <Typography variant="caption" color="error">
                        {r.error}
                      </Typography>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </Paper>
      )}
    </Box>
  );
}
