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

import Alert from '@mui/material/Alert';
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Checkbox from '@mui/material/Checkbox';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import FormControlLabel from '@mui/material/FormControlLabel';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import { applyGuidedFix,Finding, GuidedFixRequest } from '../../lib/cluster-doctor-api';

export interface GuidedFixModalProps {
  finding: Finding | null;
  cluster: string;
  open: boolean;
  onClose: () => void;
  onApplied: () => void;
}

// commandPreview renders the exact kubectl-equivalent of what the fix will do,
// so the operator sees precisely what they're authorising before confirming.
function commandPreview(f: Finding, force: boolean, replicas: number): string {
  const ns = f.namespace ? ` -n ${f.namespace}` : '';
  switch (f.guidedFixAction) {
    case 'delete_pod':
      return `kubectl delete pod ${f.resourceName}${ns}${force ? ' --force --grace-period=0' : ''}`;
    case 'delete_job':
      return `kubectl delete job ${f.resourceName}${ns}`;
    case 'uncordon_node':
      return `kubectl uncordon ${f.resourceName}`;
    case 'scale_deployment':
      return `kubectl scale deployment ${f.resourceName}${ns} --replicas=${replicas}`;
    case 'restart_deployment':
      return `kubectl rollout restart deployment ${f.resourceName}${ns}`;
    default:
      return f.guidedFixAction ?? '';
  }
}

export function GuidedFixModal({
  finding,
  cluster,
  open,
  onClose,
  onApplied,
}: GuidedFixModalProps) {
  const [force, setForce] = React.useState(false);
  const [replicas, setReplicas] = React.useState(1);
  const [applying, setApplying] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [done, setDone] = React.useState<string | null>(null);

  React.useEffect(() => {
    // Reset transient state whenever a new finding is opened.
    setForce(false);
    setReplicas(1);
    setApplying(false);
    setError(null);
    setDone(null);
  }, [finding]);

  if (!finding) return null;

  const isScale = finding.guidedFixAction === 'scale_deployment';

  async function handleConfirm() {
    if (!finding) return;

    setApplying(true);
    setError(null);

    const req: GuidedFixRequest = {
      cluster,
      action: finding.guidedFixAction!,
      namespace: finding.namespace,
      resourceName: finding.resourceName,
      confirmed: true,
    };
    if (force) req.force = true;
    if (isScale) req.replicas = replicas;

    try {
      const res = await applyGuidedFix(req);
      setDone(res.message);
      onApplied();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setApplying(false);
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Apply Fix — {finding.ruleName}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          This will run the following against <strong>{cluster}</strong>:
        </Typography>
        <Box
          component="pre"
          sx={{
            fontFamily: 'inherit',
            whiteSpace: 'pre-wrap',
            p: 1.5,
            borderRadius: 1,
            bgcolor: theme => theme.palette.background.default,
            mb: 2,
          }}
        >
          {commandPreview(finding, force, replicas)}
        </Box>

        {finding.guidedFixWarning && (
          <Alert severity="warning" sx={{ mb: 2 }}>
            {finding.guidedFixWarning}
          </Alert>
        )}

        {isScale && (
          <TextField
            type="number"
            label="Replicas"
            size="small"
            value={replicas}
            onChange={e => setReplicas(Math.max(0, Number(e.target.value)))}
            sx={{ mb: 2 }}
          />
        )}

        {finding.guidedFixAction === 'delete_pod' && (
          <FormControlLabel
            control={<Checkbox checked={force} onChange={e => setForce(e.target.checked)} />}
            label="Force delete (--grace-period=0)"
          />
        )}

        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {error}
          </Alert>
        )}
        {done && (
          <Alert severity="success" sx={{ mt: 2 }}>
            {done}
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{done ? 'Close' : 'Cancel'}</Button>
        {!done && (
          <Button variant="contained" onClick={handleConfirm} disabled={applying}>
            {applying ? 'Applying…' : 'Confirm & Apply'}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}

export default GuidedFixModal;
