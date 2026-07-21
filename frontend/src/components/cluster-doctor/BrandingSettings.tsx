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
import MenuItem from '@mui/material/MenuItem';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  getBranding,
  getRole,
  Role,
  saveBranding,
  saveRole,
} from '../../lib/cluster-doctor-branding-api';

const ROLES: { value: Role; label: string; help: string }[] = [
  { value: 'viewer', label: 'Viewer', help: 'Scan and read findings. Cannot change anything.' },
  {
    value: 'operator',
    label: 'Operator',
    help: 'Also apply Guided Fixes and suppress findings.',
  },
  {
    value: 'admin',
    label: 'Admin',
    help: 'Also manage rules, notifications, branding and licence.',
  },
];

/**
 * White-label branding and in-app role configuration. Both are admin-only and
 * enforced server-side; this UI is a convenience, not the security boundary.
 */
export function BrandingSettings() {
  const [productName, setProductName] = React.useState('');
  const [primaryColor, setPrimaryColor] = React.useState('');
  const [role, setRole] = React.useState<Role>('admin');
  const [msg, setMsg] = React.useState<{ ok: boolean; text: string } | null>(null);
  const [busy, setBusy] = React.useState(false);

  React.useEffect(() => {
    let cancelled = false;

    getBranding()
      .then(b => {
        if (cancelled) return;
        setProductName(b.productName === 'K8sense' ? '' : b.productName);
        setPrimaryColor(b.primaryColor ?? '');
      })
      .catch(() => undefined);

    getRole()
      .then(r => {
        if (!cancelled) setRole(r.role);
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, []);

  async function handleSaveBranding() {
    setBusy(true);
    setMsg(null);
    try {
      await saveBranding({ productName, primaryColor });
      setMsg({ ok: true, text: 'Branding saved. Reload to see it applied everywhere.' });
    } catch (e) {
      setMsg({ ok: false, text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  async function handleSaveRole(next: Role) {
    setBusy(true);
    setMsg(null);
    try {
      await saveRole(next);
      setRole(next);
      setMsg({
        ok: true,
        text:
          next === 'admin'
            ? 'Role set to Admin.'
            : `Role set to ${next}. Only an admin can change it back — edit role.json in the K8sense config directory if you lock yourself out.`,
      });
    } catch (e) {
      setMsg({ ok: false, text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Box>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        Rebrand the app for your organisation and choose what this install is allowed to change.
      </Typography>

      <TextField
        label="Product name"
        size="small"
        fullWidth
        margin="dense"
        placeholder="K8sense"
        value={productName}
        onChange={e => setProductName(e.target.value)}
        helperText="Leave blank to keep K8sense branding."
      />
      <TextField
        label="Primary colour"
        size="small"
        margin="dense"
        placeholder="#3B82F6"
        value={primaryColor}
        onChange={e => setPrimaryColor(e.target.value)}
        helperText="Hex colour, e.g. #10B981"
      />

      <Box sx={{ mt: 1 }}>
        <Button size="small" variant="contained" onClick={handleSaveBranding} disabled={busy}>
          Save Branding
        </Button>
      </Box>

      <TextField
        select
        label="This install's role"
        size="small"
        margin="normal"
        value={role}
        onChange={e => handleSaveRole(e.target.value as Role)}
        disabled={busy}
        sx={{ minWidth: 260 }}
        helperText={ROLES.find(r => r.value === role)?.help}
      >
        {ROLES.map(r => (
          <MenuItem key={r.value} value={r.value}>
            {r.label}
          </MenuItem>
        ))}
      </TextField>

      <Alert severity="info" sx={{ mt: 1 }}>
        The role is a local operational guardrail — it prevents accidental changes and enables a
        read-only install. It is not identity-based access control; your cluster's own RBAC still
        governs everything K8sense does.
      </Alert>

      {msg && (
        <Alert severity={msg.ok ? 'success' : 'error'} sx={{ mt: 2 }}>
          {msg.text}
        </Alert>
      )}
    </Box>
  );
}

export default BrandingSettings;
