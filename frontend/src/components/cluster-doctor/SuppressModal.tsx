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
import Button from '@mui/material/Button';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogTitle from '@mui/material/DialogTitle';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import { Finding } from '../../lib/cluster-doctor-api';
import { suppressFinding } from '../../lib/cluster-doctor-suppress-api';

export interface SuppressModalProps {
  finding: Finding | null;
  cluster: string;
  open: boolean;
  onClose: () => void;
  onDone: () => void;
}

export function SuppressModal({ finding, cluster, open, onClose, onDone }: SuppressModalProps) {
  const [reason, setReason] = React.useState('');
  const [saving, setSaving] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [done, setDone] = React.useState(false);

  React.useEffect(() => {
    // Reset transient state whenever a new finding is opened.
    setReason('');
    setSaving(false);
    setError(null);
    setDone(false);
  }, [finding]);

  if (!finding) return null;

  async function handleConfirm() {
    if (!finding) return;

    setSaving(true);
    setError(null);

    try {
      await suppressFinding({
        cluster,
        ruleId: finding.ruleId,
        namespace: finding.namespace,
        resourceKind: finding.resourceKind,
        resourceName: finding.resourceName,
        reason,
      });
      setDone(true);
      onDone();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Suppress — {finding.ruleName}</DialogTitle>
      <DialogContent>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          This mutes the finding for{' '}
          <strong>
            {finding.resourceKind} {finding.namespace ? `${finding.namespace}/` : ''}
            {finding.resourceName}
          </strong>{' '}
          on <strong>{cluster}</strong> across future scans until you un-suppress it.
        </Typography>
        <TextField
          label="Reason"
          required
          multiline
          minRows={3}
          fullWidth
          value={reason}
          onChange={e => setReason(e.target.value)}
          sx={{ mt: 2 }}
        />

        {error && (
          <Alert severity="error" sx={{ mt: 2 }}>
            {error}
          </Alert>
        )}
        {done && (
          <Alert severity="success" sx={{ mt: 2 }}>
            Finding suppressed.
          </Alert>
        )}
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>{done ? 'Close' : 'Cancel'}</Button>
        {!done && (
          <Button
            variant="contained"
            onClick={handleConfirm}
            disabled={saving || reason.trim() === ''}
          >
            {saving ? 'Suppressing…' : 'Suppress'}
          </Button>
        )}
      </DialogActions>
    </Dialog>
  );
}

export default SuppressModal;
