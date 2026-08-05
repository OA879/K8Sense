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

/** Typed client for the global internal-registry setting (/cluster-doctor/registry).
 *  One registry that every feature's default images are rewritten onto — the
 *  single knob that makes the whole platform air-gapped. */
import { apiFetch } from './cluster-doctor-api';

export interface RegistrySetting {
  registry: string;
  airGapped: boolean;
}

export function getRegistry(): Promise<RegistrySetting> {
  return apiFetch('/registry');
}

export function setRegistry(registry: string): Promise<RegistrySetting> {
  return apiFetch('/registry', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ registry }),
  });
}
