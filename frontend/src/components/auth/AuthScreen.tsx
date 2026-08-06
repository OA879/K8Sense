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
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import { bootstrap, login } from '../../lib/cluster-doctor-auth-api';

const ACCENT = '#3B82F6';
const BG = '#0f172a';

/**
 * Full-screen sign-in gate for the multi-user web deployment. In "bootstrap"
 * mode it creates the first admin account (first run); in "login" mode it signs
 * an existing user in. Fully offline — accounts live in K8sense's own database.
 */
export default function AuthScreen({
  mode,
  onDone,
}: {
  mode: 'login' | 'bootstrap';
  onDone: () => void;
}) {
  const [username, setUsername] = React.useState('');
  const [password, setPassword] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const isSetup = mode === 'bootstrap';

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    setBusy(true);
    setError(null);
    try {
      if (isSetup) {
        await bootstrap(username.trim(), password);
      } else {
        await login(username.trim(), password);
      }
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Box
      sx={{
        position: 'fixed',
        inset: 0,
        bgcolor: BG,
        color: '#e8eef8',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 2000,
        backgroundImage:
          'radial-gradient(900px 480px at 78% -10%, rgba(59,130,246,0.18), transparent 60%)',
      }}
    >
      <Box
        component="form"
        onSubmit={submit}
        sx={{
          width: 380,
          maxWidth: '90vw',
          p: 4,
          borderRadius: 3,
          bgcolor: '#111c31',
          border: '1px solid #22304a',
          boxShadow: '0 30px 80px rgba(0,0,0,0.45)',
        }}
      >
        <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.25, mb: 3 }}>
          <Box
            sx={{
              width: 34,
              height: 34,
              borderRadius: 2,
              background: `linear-gradient(150deg, ${ACCENT}, #1d4ed8)`,
              display: 'grid',
              placeItems: 'center',
            }}
          >
            <Icon icon="mdi:shield-check-outline" color="#fff" width={20} />
          </Box>
          <Typography sx={{ fontWeight: 800, fontSize: 20 }}>
            <span style={{ color: ACCENT }}>K8</span>sense
          </Typography>
        </Box>

        <Typography variant="h6" sx={{ fontWeight: 700, mb: 0.5 }}>
          {isSetup ? 'Create the admin account' : 'Sign in'}
        </Typography>
        <Typography variant="body2" sx={{ color: '#9fb0c9', mb: 2.5 }}>
          {isSetup
            ? 'First-time setup — this account administers K8sense. It stays on this server; nothing leaves your network.'
            : 'Enter your K8sense credentials.'}
        </Typography>

        {error && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {error}
          </Alert>
        )}

        <TextField
          fullWidth
          label="Username"
          value={username}
          onChange={e => setUsername(e.target.value)}
          autoFocus
          autoComplete="username"
          sx={{ mb: 2 }}
        />
        <TextField
          fullWidth
          type="password"
          label="Password"
          value={password}
          onChange={e => setPassword(e.target.value)}
          autoComplete={isSetup ? 'new-password' : 'current-password'}
          helperText={isSetup ? 'At least 8 characters.' : ' '}
          sx={{ mb: 2 }}
        />
        <Button
          type="submit"
          fullWidth
          variant="contained"
          disabled={busy || !username.trim() || !password}
          sx={{ py: 1.1, fontWeight: 700 }}
        >
          {busy ? 'Please wait…' : isSetup ? 'Create account & sign in' : 'Sign in'}
        </Button>
      </Box>
    </Box>
  );
}
