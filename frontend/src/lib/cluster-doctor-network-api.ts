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

/** Typed client for the Cluster Doctor Network Map (/cluster-doctor/network-map). */
import { apiFetch } from './cluster-doctor-api';

/** Ingress reachability of a workload. */
export type Exposure = 'open' | 'restricted' | 'isolated' | 'external';

export interface NetNode {
  id: string;
  namespace: string;
  name: string;
  kind: string;
  exposure: Exposure;
  protected: boolean;
  /** True when this workload / external service is a datastore. */
  database?: boolean;
  /** Detected engine (postgres, mysql, redis, …) when database is true. */
  dbEngine?: string;
}

/** A policy-allowed connection (source -> target). */
export interface NetEdge {
  id: string;
  source: string;
  target: string;
  ports?: string;
}

/** A live observed flow, present only when a service mesh is detected. */
export interface TrafficEdge {
  id: string;
  source: string;
  target: string;
  rps: number;
}

/** A configured (not observed) connection inferred from env/args/ConfigMap. */
export interface InferredEdge {
  id: string;
  source: string;
  target: string;
  via: string;
}

export interface MeshInfo {
  enabled: boolean;
  source?: string;
}

export interface NetworkMap {
  nodes: NetNode[];
  edges: NetEdge[];
  traffic: TrafficEdge[];
  inferred: InferredEdge[];
  mesh: MeshInfo;
  namespaces: string[];
}

/** Fetches the network + policy map for a cluster, optionally one namespace. */
export function getNetworkMap(cluster: string, namespace?: string): Promise<NetworkMap> {
  const q = new URLSearchParams({ cluster });
  if (namespace) {
    q.set('namespace', namespace);
  }

  return apiFetch(`/network-map?${q.toString()}`);
}
