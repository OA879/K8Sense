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

/** Typed client for the App Catalog (/cluster-doctor/catalog). One-click Helm
 *  install/uninstall of curated open-source tools onto the pointed cluster. */
import { apiFetch } from './cluster-doctor-api';

export interface CatalogApp {
  id: string;
  name: string;
  description: string;
  category: string;
  icon: string;
  namespace: string;
  installed: boolean;
  status?: string;
}

export interface CatalogActionResult {
  result: 'success' | 'failed';
  message: string;
}

/** Lists catalog apps with their install state on the given cluster. */
export function getCatalog(cluster: string): Promise<{ apps: CatalogApp[] }> {
  return apiFetch(`/catalog?cluster=${encodeURIComponent(cluster)}`);
}

/** Installs an app onto the cluster via Helm. */
export function installApp(cluster: string, appId: string): Promise<CatalogActionResult> {
  return apiFetch(
    '/catalog/install',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cluster, appId }),
    },
    cluster
  );
}

/** Uninstalls an app (Helm uninstall — removes the release and its pods). */
export function uninstallApp(cluster: string, appId: string): Promise<CatalogActionResult> {
  return apiFetch(
    '/catalog/uninstall',
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ cluster, appId }),
    },
    cluster
  );
}
