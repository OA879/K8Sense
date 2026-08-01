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

/** Typed client for the offline K8sense Copilot (/cluster-doctor/ai/*). */
import { apiFetch } from './cluster-doctor-api';

export type ChatRole = 'user' | 'assistant';

export interface ChatMessage {
  role: ChatRole;
  content: string;
}

export interface AIStatus {
  enabled: boolean;
  reachable: boolean;
  endpoint: string;
  model: string;
}

/** Reports whether a local model is configured and answering. */
export function getAIStatus(): Promise<AIStatus> {
  return apiFetch(`/ai/status`);
}

/**
 * Sends the conversation to the local model, grounded in the cluster's latest
 * scan findings on the backend. Returns the assistant's reply.
 */
export async function aiChat(
  cluster: string,
  messages: ChatMessage[],
  scanId?: string
): Promise<string> {
  const body: { cluster: string; messages: ChatMessage[]; scanId?: string } = { cluster, messages };
  if (scanId) {
    body.scanId = scanId;
  }

  const res = await apiFetch(`/ai/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });

  return (res as { reply: string }).reply;
}
