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

/**
 * White-label branding and in-app role. Branding is readable by everyone (the
 * app shell needs it on every load); changing either is an admin action and is
 * enforced server-side.
 */
import React from 'react';
import { apiFetch } from './cluster-doctor-api';

export interface Branding {
  productName: string;
  primaryColor: string;
  logoDataUri: string;
  hidePoweredBy: boolean;
}

export type Role = 'viewer' | 'operator' | 'admin';

const DEFAULT_BRANDING: Branding = {
  productName: 'K8sense',
  primaryColor: '',
  logoDataUri: '',
  hidePoweredBy: false,
};

export function getBranding(): Promise<Branding> {
  return apiFetch('/branding');
}

export function saveBranding(b: Partial<Branding>): Promise<{ result: string }> {
  return apiFetch('/branding', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(b),
  });
}

export function getRole(): Promise<{ role: Role }> {
  return apiFetch('/role');
}

export function saveRole(role: Role): Promise<{ result: string; role: Role }> {
  return apiFetch('/role', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ role }),
  });
}

/**
 * Reads the white-label branding, falling back to stock K8sense on any error
 * so a bad or unreachable config can never blank out the app shell.
 */
export function useBranding(): Branding {
  const [branding, setBranding] = React.useState<Branding>(DEFAULT_BRANDING);

  React.useEffect(() => {
    let cancelled = false;

    getBranding()
      .then(b => {
        if (!cancelled) setBranding({ ...DEFAULT_BRANDING, ...b });
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, []);

  return branding;
}

/** Reads the in-app role, defaulting to viewer until the real value loads. */
export function useRole(): Role {
  const [role, setRole] = React.useState<Role>('admin');

  React.useEffect(() => {
    let cancelled = false;

    getRole()
      .then(r => {
        if (!cancelled) setRole(r.role);
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, []);

  return role;
}

/** True when the current role meets or exceeds the required one. */
export function roleAtLeast(have: Role, required: Role): boolean {
  const rank: Record<Role, number> = { viewer: 0, operator: 1, admin: 2 };
  return rank[have] >= rank[required];
}
