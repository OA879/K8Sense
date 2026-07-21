import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Chip from '@mui/material/Chip';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  LicenceInfo,
  activateLicence,
  getLicence,
  startTrial,
} from '../../lib/cluster-doctor-settings-api';

const TIER_COLOR: Record<LicenceInfo['tier'], 'default' | 'primary' | 'success'> = {
  free: 'default',
  pro: 'primary',
  enterprise: 'success',
};

export function LicenceStatus() {
  const [licence, setLicence] = React.useState<LicenceInfo | null>(null);
  const [pasteOpen, setPasteOpen] = React.useState(false);
  const [pasteValue, setPasteValue] = React.useState('');
  const [error, setError] = React.useState<string | null>(null);
  const [busy, setBusy] = React.useState(false);

  const refresh = React.useCallback(() => {
    getLicence()
      .then(setLicence)
      .catch(e => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  React.useEffect(refresh, [refresh]);

  async function handleActivate() {
    setBusy(true);
    setError(null);
    try {
      const info = await activateLicence(pasteValue);
      setLicence(info);
      setPasteOpen(false);
      setPasteValue('');
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  async function handleTrial() {
    setBusy(true);
    setError(null);
    try {
      setLicence(await startTrial());
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  if (!licence) {
    return <Typography color="text.secondary">Loading licence…</Typography>;
  }

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, mb: 1 }}>
        <Typography variant="subtitle1">Licence</Typography>
        <Chip
          size="small"
          label={licence.tier.toUpperCase()}
          color={TIER_COLOR[licence.tier]}
        />
        {licence.isTrial && <Chip size="small" label="TRIAL" variant="outlined" />}
        {licence.inGrace && <Chip size="small" label="GRACE" color="warning" />}
      </Box>

      {licence.customerName && (
        <Typography variant="body2">Licensed to {licence.customerName}</Typography>
      )}
      {licence.expiresAt && (
        <Typography variant="body2" color="text.secondary">
          Expires {licence.expiresAt} · up to {licence.maxClusters} clusters · {licence.seatCount}{' '}
          seat{licence.seatCount === 1 ? '' : 's'}
        </Typography>
      )}
      {licence.message && (
        <Alert severity={licence.valid ? 'info' : 'warning'} sx={{ mt: 1 }}>
          {licence.message}
        </Alert>
      )}

      {error && (
        <Alert severity="error" sx={{ mt: 1 }}>
          {error}
        </Alert>
      )}

      <Box sx={{ display: 'flex', gap: 1, mt: 2, flexWrap: 'wrap' }}>
        {licence.tier === 'free' && (
          <>
            <Button size="small" variant="contained" onClick={handleTrial} disabled={busy}>
              Start 14-day Pro Trial
            </Button>
            <Button size="small" variant="outlined" onClick={() => setPasteOpen(o => !o)}>
              Activate Licence
            </Button>
          </>
        )}
        {licence.tier !== 'free' && (
          <Button size="small" variant="outlined" onClick={() => setPasteOpen(o => !o)}>
            Replace Licence
          </Button>
        )}
      </Box>

      {pasteOpen && (
        <Box sx={{ mt: 2 }}>
          <TextField
            label="Paste .k8sense-licence file contents"
            multiline
            minRows={4}
            fullWidth
            value={pasteValue}
            onChange={e => setPasteValue(e.target.value)}
          />
          <Button
            size="small"
            variant="contained"
            sx={{ mt: 1 }}
            onClick={handleActivate}
            disabled={busy || !pasteValue.trim()}
          >
            Activate
          </Button>
        </Box>
      )}
    </Box>
  );
}

export default LicenceStatus;
