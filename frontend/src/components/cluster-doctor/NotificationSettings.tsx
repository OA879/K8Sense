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
import FormControlLabel from '@mui/material/FormControlLabel';
import MenuItem from '@mui/material/MenuItem';
import Switch from '@mui/material/Switch';
import TextField from '@mui/material/TextField';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  getNotifySettings,
  saveNotifySettings,
  testNotification,
} from '../../lib/cluster-doctor-notify-api';

const INTERVALS = [
  { value: 15, label: 'Every 15 minutes' },
  { value: 30, label: 'Every 30 minutes' },
  { value: 60, label: 'Hourly' },
  { value: 360, label: 'Every 6 hours' },
  { value: 1440, label: 'Daily' },
];

export interface NotificationSettingsProps {
  cluster: string;
}

/**
 * Webhook alerting + scheduled scan configuration for one cluster. Both are
 * Pro features; the backend returns 402 on save for Free licences and that
 * message is surfaced inline.
 */
export function NotificationSettings({ cluster }: NotificationSettingsProps) {
  const [slack, setSlack] = React.useState('');
  const [teams, setTeams] = React.useState('');
  const [notifyCritical, setNotifyCritical] = React.useState(true);
  const [scheduleEnabled, setScheduleEnabled] = React.useState(false);
  const [interval, setIntervalMinutes] = React.useState(60);
  const [lastRun, setLastRun] = React.useState(0);
  const [msg, setMsg] = React.useState<{ ok: boolean; text: string } | null>(null);
  const [busy, setBusy] = React.useState(false);

  React.useEffect(() => {
    if (!cluster) return;

    let cancelled = false;

    getNotifySettings(cluster)
      .then(s => {
        if (cancelled) return;
        setSlack(s.notifications.slackWebhook ?? '');
        setTeams(s.notifications.teamsWebhook ?? '');
        setNotifyCritical(s.notifications.notifyCritical);
        setScheduleEnabled(s.schedule.enabled);
        setIntervalMinutes(s.schedule.intervalMinutes);
        setLastRun(s.schedule.lastRunAt);
      })
      .catch(() => undefined);

    return () => {
      cancelled = true;
    };
  }, [cluster]);

  async function handleSave() {
    setBusy(true);
    setMsg(null);
    try {
      await saveNotifySettings({
        cluster,
        slackWebhook: slack,
        teamsWebhook: teams,
        notifyCritical,
        scheduleEnabled,
        intervalMinutes: interval,
      });
      setMsg({ ok: true, text: 'Settings saved.' });
    } catch (e) {
      setMsg({ ok: false, text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  async function handleTest() {
    setBusy(true);
    setMsg(null);
    try {
      await testNotification(cluster);
      setMsg({ ok: true, text: 'Test alert sent to the configured webhook(s).' });
    } catch (e) {
      setMsg({ ok: false, text: e instanceof Error ? e.message : String(e) });
    } finally {
      setBusy(false);
    }
  }

  return (
    <Box>
      <Typography variant="body2" color="text.secondary" gutterBottom>
        Alert on newly-appeared CRITICAL findings and run scans on a schedule for{' '}
        <strong>{cluster}</strong>. Webhook URLs are stored locally and only contacted by K8sense
        itself.
      </Typography>

      <TextField
        label="Slack webhook URL"
        size="small"
        fullWidth
        margin="dense"
        placeholder="https://hooks.slack.com/services/…"
        value={slack}
        onChange={e => setSlack(e.target.value)}
      />
      <TextField
        label="Microsoft Teams webhook URL"
        size="small"
        fullWidth
        margin="dense"
        placeholder="https://outlook.office.com/webhook/…"
        value={teams}
        onChange={e => setTeams(e.target.value)}
      />

      <FormControlLabel
        control={
          <Switch
            checked={notifyCritical}
            onChange={(_e, checked) => setNotifyCritical(checked)}
          />
        }
        label="Notify on new critical findings"
      />

      <Box sx={{ display: 'flex', alignItems: 'center', gap: 2, mt: 1 }}>
        <FormControlLabel
          control={
            <Switch
              checked={scheduleEnabled}
              onChange={(_e, checked) => setScheduleEnabled(checked)}
            />
          }
          label="Scheduled scans"
        />
        <TextField
          select
          size="small"
          label="Interval"
          value={interval}
          disabled={!scheduleEnabled}
          onChange={e => setIntervalMinutes(Number(e.target.value))}
          sx={{ minWidth: 180 }}
        >
          {INTERVALS.map(opt => (
            <MenuItem key={opt.value} value={opt.value}>
              {opt.label}
            </MenuItem>
          ))}
        </TextField>
      </Box>

      {scheduleEnabled && lastRun > 0 && (
        <Typography variant="caption" color="text.secondary" display="block" sx={{ mt: 1 }}>
          Last scheduled run: {new Date(lastRun * 1000).toLocaleString()}
        </Typography>
      )}

      <Box sx={{ display: 'flex', gap: 1, mt: 2 }}>
        <Button size="small" variant="contained" onClick={handleSave} disabled={busy}>
          Save
        </Button>
        <Button
          size="small"
          variant="outlined"
          onClick={handleTest}
          disabled={busy || (!slack && !teams)}
        >
          Send Test Alert
        </Button>
      </Box>

      {msg && (
        <Alert severity={msg.ok ? 'success' : 'error'} sx={{ mt: 2 }}>
          {msg.text}
        </Alert>
      )}
    </Box>
  );
}

export default NotificationSettings;
