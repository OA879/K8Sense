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

/** Typed client for local authentication (/cluster-doctor/auth, /users).
 *  Offline, self-contained user accounts for the multi-user web deployment. */
import { apiFetch } from './cluster-doctor-api';

const SESSION_KEY = 'k8sense.session';

export type Role = 'viewer' | 'operator' | 'admin';

export interface AuthUser {
  id: string;
  username: string;
  role: Role;
  disabled: boolean;
  createdAt: number;
}

export interface AuthStatus {
  authEnabled: boolean;
  /** 'local' | 'oidc' | '' (off). */
  mode: string;
  needsBootstrap: boolean;
}

export function getSessionToken(): string {
  return localStorage.getItem(SESSION_KEY) || '';
}
export function setSessionToken(token: string) {
  if (token) localStorage.setItem(SESSION_KEY, token);
  else localStorage.removeItem(SESSION_KEY);
}

/** Whether auth is on and whether the install still needs its first admin. */
export function getAuthStatus(): Promise<AuthStatus> {
  return apiFetch('/auth/status');
}

/** Creates the first admin account (only works before any user exists). */
export async function bootstrap(username: string, password: string): Promise<AuthUser> {
  const res = await apiFetch<{ token: string; user: AuthUser }>('/auth/bootstrap', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  setSessionToken(res.token);
  return res.user;
}

/** Signs in and stores the session token. */
export async function login(username: string, password: string): Promise<AuthUser> {
  const res = await apiFetch<{ token: string; user: AuthUser }>('/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  });
  setSessionToken(res.token);
  return res.user;
}

/** Signs out and clears the local session. */
export async function logout(): Promise<void> {
  try {
    await apiFetch('/auth/logout', { method: 'POST' });
  } finally {
    setSessionToken('');
  }
}

/** Returns the current user (or throws 401 if the session is invalid). */
export function getMe(): Promise<{ user: AuthUser; authEnabled: boolean }> {
  return apiFetch('/auth/me');
}

/** Lists all accounts (admin only). */
export function listUsers(): Promise<{ users: AuthUser[] }> {
  return apiFetch('/users');
}

/** Creates an account (admin only). */
export function createUser(username: string, password: string, role: Role): Promise<void> {
  return apiFetch('/users', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password, role }),
  });
}

/** Updates an account's role or enabled state (admin only). */
export function updateUser(id: string, patch: { role?: Role; disabled?: boolean }): Promise<void> {
  return apiFetch(`/users/${encodeURIComponent(id)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(patch),
  });
}
