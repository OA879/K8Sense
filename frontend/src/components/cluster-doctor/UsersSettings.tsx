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
import Chip from '@mui/material/Chip';
import MenuItem from '@mui/material/MenuItem';
import Select from '@mui/material/Select';
import Table from '@mui/material/Table';
import TableBody from '@mui/material/TableBody';
import TableCell from '@mui/material/TableCell';
import TableHead from '@mui/material/TableHead';
import TableRow from '@mui/material/TableRow';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  AuthUser,
  Role,
  createUser,
  getMe,
  listUsers,
  logout,
  updateUser,
} from '../../lib/cluster-doctor-auth-api';

const ROLES: Role[] = ['viewer', 'operator', 'admin'];

export function UsersSettings() {
  const [enabled, setEnabled] = React.useState<boolean | null>(null);
  const [me, setMe] = React.useState<AuthUser | null>(null);
  const [users, setUsers] = React.useState<AuthUser[]>([]);
  const [error, setError] = React.useState<string | null>(null);
  const [nu, setNu] = React.useState({ username: '', password: '', role: 'viewer' as Role });

  const isAdmin = me?.role === 'admin';

  const load = React.useCallback(() => {
    getMe()
      .then(r => {
        setEnabled(r.authEnabled);
        setMe(r.user);
        if (r.authEnabled && r.user.role === 'admin') {
          listUsers()
            .then(x => setUsers(x.users))
            .catch(e => setError(e instanceof Error ? e.message : String(e)));
        }
      })
      .catch(() => setEnabled(false));
  }, []);

  React.useEffect(load, [load]);

  async function addUser() {
    setError(null);
    try {
      await createUser(nu.username.trim(), nu.password, nu.role);
      setNu({ username: '', password: '', role: 'viewer' });
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  async function patch(u: AuthUser, p: { role?: Role; disabled?: boolean }) {
    setError(null);
    try {
      await updateUser(u.id, p);
      load();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }

  // Auth off (desktop) — nothing to manage.
  if (enabled === false) {
    return (
      <Typography variant="body2" color="text.secondary">
        Authentication is off — this is a single-user install. Set <code>K8SENSE_AUTH=local</code> on
        the server to enable per-user accounts.
      </Typography>
    );
  }
  if (enabled === null) return null;

  return (
    <Box>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 2 }}>
        <Typography variant="body2" color="text.secondary">
          Signed in as <strong>{me?.username}</strong>
        </Typography>
        <Chip size="small" label={me?.role} variant="outlined" />
        <Box sx={{ flex: 1 }} />
        <Button size="small" variant="outlined" onClick={() => logout().then(() => window.location.reload())}>
          Sign out
        </Button>
      </Box>

      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {!isAdmin ? (
        <Typography variant="body2" color="text.secondary">
          Only admins can manage accounts.
        </Typography>
      ) : (
        <>
          <Table size="small" sx={{ mb: 2 }}>
            <TableHead>
              <TableRow>
                <TableCell>User</TableCell>
                <TableCell>Role</TableCell>
                <TableCell>Status</TableCell>
                <TableCell align="right">Actions</TableCell>
              </TableRow>
            </TableHead>
            <TableBody>
              {users.map(u => {
                const self = u.id === me?.id;
                return (
                  <TableRow key={u.id}>
                    <TableCell>
                      {u.username}
                      {self && (
                        <Typography variant="caption" color="text.secondary" sx={{ ml: 0.75 }}>
                          (you)
                        </Typography>
                      )}
                    </TableCell>
                    <TableCell>
                      <Select
                        size="small"
                        value={u.role}
                        disabled={self}
                        onChange={e => patch(u, { role: e.target.value as Role })}
                        sx={{ minWidth: 120 }}
                      >
                        {ROLES.map(r => (
                          <MenuItem key={r} value={r}>
                            {r}
                          </MenuItem>
                        ))}
                      </Select>
                    </TableCell>
                    <TableCell>
                      <Chip
                        size="small"
                        color={u.disabled ? 'default' : 'success'}
                        variant="outlined"
                        label={u.disabled ? 'disabled' : 'active'}
                      />
                    </TableCell>
                    <TableCell align="right">
                      {!self && (
                        <Button size="small" onClick={() => patch(u, { disabled: !u.disabled })}>
                          {u.disabled ? 'Enable' : 'Disable'}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                );
              })}
            </TableBody>
          </Table>

          <Typography variant="subtitle2" sx={{ mb: 1 }}>
            Add a user
          </Typography>
          <Box sx={{ display: 'flex', gap: 1, flexWrap: 'wrap', alignItems: 'center' }}>
            <TextField
              size="small"
              label="Username"
              value={nu.username}
              onChange={e => setNu({ ...nu, username: e.target.value })}
            />
            <TextField
              size="small"
              type="password"
              label="Password (min 8)"
              value={nu.password}
              onChange={e => setNu({ ...nu, password: e.target.value })}
            />
            <Select
              size="small"
              value={nu.role}
              onChange={e => setNu({ ...nu, role: e.target.value as Role })}
            >
              {ROLES.map(r => (
                <MenuItem key={r} value={r}>
                  {r}
                </MenuItem>
              ))}
            </Select>
            <Button
              variant="contained"
              onClick={addUser}
              disabled={!nu.username.trim() || nu.password.length < 8}
            >
              Add
            </Button>
          </Box>
        </>
      )}
    </Box>
  );
}
