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
 * Typed client for K8sense's notification + scheduled-scan settings. Webhook
 * URLs are stored locally in the K8sense database and only ever contacted
 * directly by the backend.
 */
import { apiFetch } from './cluster-doctor-api';

export interface NotificationConfig {
  clusterId: string;
  slackWebhook?: string;
  teamsWebhook?: string;
  notifyCritical: boolean;
}

export interface ScanSchedule {
  clusterId: string;
  enabled: boolean;
  intervalMinutes: number;
  lastRunAt: number;
}

export interface NotifySettings {
  notifications: NotificationConfig;
  schedule: ScanSchedule;
}

/** Reads a cluster's webhook config and scan schedule. */
export function getNotifySettings(cluster: string): Promise<NotifySettings> {
  return apiFetch(`/notifications?cluster=${encodeURIComponent(cluster)}`);
}

export interface NotifyUpdate {
  cluster: string;
  slackWebhook: string;
  teamsWebhook: string;
  notifyCritical: boolean;
  scheduleEnabled: boolean;
  intervalMinutes: number;
}

/** Saves webhook config + scan schedule. Pro-gated server-side. */
export function saveNotifySettings(update: NotifyUpdate): Promise<{ result: string }> {
  return apiFetch('/notifications', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(update),
  });
}

/** Posts a sample alert to the configured webhooks. */
export function testNotification(cluster: string): Promise<{ result: string }> {
  return apiFetch(`/notifications/test?cluster=${encodeURIComponent(cluster)}`, { method: 'POST' });
}
