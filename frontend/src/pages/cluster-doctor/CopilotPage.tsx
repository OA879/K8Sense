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
import Chip from '@mui/material/Chip';
import CircularProgress from '@mui/material/CircularProgress';
import Dialog from '@mui/material/Dialog';
import DialogActions from '@mui/material/DialogActions';
import DialogContent from '@mui/material/DialogContent';
import DialogContentText from '@mui/material/DialogContentText';
import DialogTitle from '@mui/material/DialogTitle';
import IconButton from '@mui/material/IconButton';
import Paper from '@mui/material/Paper';
import TextField from '@mui/material/TextField';
import Tooltip from '@mui/material/Tooltip';
import Typography from '@mui/material/Typography';
import React from 'react';
import {
  AIStatus,
  aiChat,
  ChatMessage,
  getAIConfig,
  getAIStatus,
  setAIConfig,
} from '../../lib/cluster-doctor-ai-api';
import { useCluster } from '../../lib/k8s';

const SUGGESTIONS = [
  'What is crashing or unhealthy right now?',
  'What are the most critical findings, and how do I fix the top one?',
  'Any recent warning events I should worry about?',
  'Is this cluster safe to upgrade?',
];

function StatusDot({ status }: { status: AIStatus | null }) {
  const ok = status?.reachable;
  const color = ok ? '#2e7d32' : status?.enabled ? '#ed6c02' : '#9e9e9e';
  const label = !status
    ? 'Checking…'
    : !status.enabled
    ? 'Disabled'
    : ok
    ? `Online · ${status.model}`
    : 'Model offline';

  return (
    <Chip
      size="small"
      variant="outlined"
      icon={<Box sx={{ width: 8, height: 8, borderRadius: '50%', bgcolor: color, ml: 1 }} />}
      label={label}
    />
  );
}

function OfflineHelp({ status }: { status: AIStatus }) {
  return (
    <Alert severity="info" icon={<Icon icon="mdi:server-network-off" />} sx={{ mb: 2 }}>
      <Typography variant="body2" sx={{ fontWeight: 600, mb: 0.5 }}>
        No local model is answering at <code>{status.endpoint}</code>.
      </Typography>
      <Typography variant="body2" sx={{ mb: 1 }}>
        The Copilot runs entirely on your own hardware — no API key, no internet, nothing leaves your
        network. To bring it online, run a local model on this machine (or point{' '}
        <code>K8SENSE_AI_ENDPOINT</code> at your own inference server):
      </Typography>
      <Box
        component="pre"
        sx={{
          bgcolor: 'action.hover',
          p: 1.5,
          borderRadius: 1,
          fontSize: 13,
          overflowX: 'auto',
          m: 0,
        }}
      >
        {`# one-time, on a machine with internet — then it runs fully offline
curl -fsSL https://ollama.com/install.sh | sh
ollama pull ${status.model}   # Apache-2.0, ~5 GB
ollama serve                  # exposes ${status.endpoint}`}
      </Box>
    </Alert>
  );
}

function SettingsDialog({
  open,
  onClose,
  onSaved,
}: {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
}) {
  const [endpoint, setEndpoint] = React.useState('');
  const [model, setModel] = React.useState('');
  const [saving, setSaving] = React.useState(false);
  const [err, setErr] = React.useState<string | null>(null);

  React.useEffect(() => {
    if (!open) return;
    setErr(null);
    getAIConfig()
      .then(c => {
        setEndpoint(c.endpoint);
        setModel(c.model);
      })
      .catch(() => {});
  }, [open]);

  async function save() {
    setSaving(true);
    setErr(null);
    try {
      await setAIConfig({ endpoint: endpoint.trim(), model: model.trim() });
      onSaved();
      onClose();
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
    } finally {
      setSaving(false);
    }
  }

  return (
    <Dialog open={open} onClose={onClose} maxWidth="sm" fullWidth>
      <DialogTitle>Copilot model</DialogTitle>
      <DialogContent>
        <DialogContentText sx={{ mb: 2 }}>
          Point the Copilot at any OpenAI-compatible model server — your local Ollama, a shared GPU
          box, or one running inside the cluster. It never needs an API key or internet.
        </DialogContentText>
        {err && (
          <Alert severity="error" sx={{ mb: 2 }}>
            {err}
          </Alert>
        )}
        <TextField
          fullWidth
          label="Endpoint"
          placeholder="http://localhost:11434/v1"
          value={endpoint}
          onChange={e => setEndpoint(e.target.value)}
          sx={{ mb: 2 }}
          helperText="Base URL of the OpenAI-compatible API (Ollama, vLLM, llama.cpp…)."
        />
        <TextField
          fullWidth
          label="Model"
          placeholder="qwen2.5"
          value={model}
          onChange={e => setModel(e.target.value)}
          helperText="Model name/tag to request."
        />
      </DialogContent>
      <DialogActions>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="contained" onClick={save} disabled={saving || !endpoint.trim()}>
          {saving ? 'Saving…' : 'Save'}
        </Button>
      </DialogActions>
    </Dialog>
  );
}

function Bubble({ message }: { message: ChatMessage }) {
  const isUser = message.role === 'user';

  return (
    <Box sx={{ display: 'flex', justifyContent: isUser ? 'flex-end' : 'flex-start', mb: 1.5 }}>
      <Paper
        elevation={0}
        sx={{
          p: 1.5,
          maxWidth: '80%',
          bgcolor: isUser ? 'primary.main' : 'action.hover',
          color: isUser ? 'primary.contrastText' : 'text.primary',
          borderRadius: 2,
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-word',
        }}
      >
        <Typography variant="body2">{message.content}</Typography>
      </Paper>
    </Box>
  );
}

export default function CopilotPage() {
  const cluster = useCluster();
  const [status, setStatus] = React.useState<AIStatus | null>(null);
  const [messages, setMessages] = React.useState<ChatMessage[]>([]);
  const [input, setInput] = React.useState('');
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [settingsOpen, setSettingsOpen] = React.useState(false);
  const endRef = React.useRef<HTMLDivElement>(null);

  const refreshStatus = React.useCallback(() => {
    let cancelled = false;
    getAIStatus()
      .then(s => !cancelled && setStatus(s))
      .catch(
        () => !cancelled && setStatus({ enabled: false, reachable: false, endpoint: '', model: '' })
      );
    return () => {
      cancelled = true;
    };
  }, []);

  React.useEffect(() => refreshStatus(), [refreshStatus]);

  React.useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, busy]);

  async function send(text: string) {
    const question = text.trim();
    if (!question || busy || !cluster) return;

    const next = [...messages, { role: 'user', content: question } as ChatMessage];
    setMessages(next);
    setInput('');
    setBusy(true);
    setError(null);

    try {
      const reply = await aiChat(cluster, next);
      setMessages([...next, { role: 'assistant', content: reply }]);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  const canChat = status?.reachable && !!cluster;

  return (
    <Box sx={{ p: 3, display: 'flex', flexDirection: 'column', height: 'calc(100vh - 64px)' }}>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1.5, mb: 0.5 }}>
        <Icon icon="mdi:robot-happy-outline" width={28} />
        <Typography variant="h4">Copilot</Typography>
        <StatusDot status={status} />
        <Box sx={{ flex: 1 }} />
        <Tooltip title="Configure model">
          <IconButton onClick={() => setSettingsOpen(true)}>
            <Icon icon="mdi:cog-outline" />
          </IconButton>
        </Tooltip>
      </Box>
      <SettingsDialog
        open={settingsOpen}
        onClose={() => setSettingsOpen(false)}
        onSaved={refreshStatus}
      />
      <Typography color="text.secondary" sx={{ mb: 2 }}>
        An offline AI assistant grounded in <strong>{cluster || 'your cluster'}</strong>&apos;s latest
        scan <em>and</em> its live state (unhealthy pods, warning events, node health) — runs on your
        own hardware, no data leaves your network.
      </Typography>

      {status && status.enabled && !status.reachable && <OfflineHelp status={status} />}
      {status && !status.enabled && (
        <Alert severity="warning" sx={{ mb: 2 }}>
          The Copilot has been disabled on this server (<code>K8SENSE_AI_DISABLED</code>).
        </Alert>
      )}
      {error && (
        <Alert severity="error" sx={{ mb: 2 }} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      <Paper
        variant="outlined"
        sx={{ flex: 1, overflowY: 'auto', p: 2, mb: 2, bgcolor: 'background.default' }}
      >
        {messages.length === 0 && (
          <Box sx={{ color: 'text.secondary' }}>
            <Typography variant="body2" sx={{ mb: 1.5 }}>
              Ask about this cluster&apos;s findings, an incident, or a fix. For example:
            </Typography>
            <Box sx={{ display: 'flex', flexWrap: 'wrap', gap: 1 }}>
              {SUGGESTIONS.map(s => (
                <Chip
                  key={s}
                  label={s}
                  variant="outlined"
                  clickable
                  disabled={!canChat}
                  onClick={() => send(s)}
                />
              ))}
            </Box>
          </Box>
        )}

        {messages.map((m, i) => (
          <Bubble key={i} message={m} />
        ))}

        {busy && (
          <Box sx={{ display: 'flex', alignItems: 'center', gap: 1, color: 'text.secondary' }}>
            <CircularProgress size={16} />
            <Typography variant="body2">Thinking…</Typography>
          </Box>
        )}
        <div ref={endRef} />
      </Paper>

      <Box sx={{ display: 'flex', gap: 1, alignItems: 'flex-end' }}>
        <TextField
          fullWidth
          multiline
          maxRows={4}
          size="small"
          placeholder={canChat ? 'Ask the Copilot…' : 'Start a local model to chat'}
          value={input}
          disabled={!canChat || busy}
          onChange={e => setInput(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter' && !e.shiftKey) {
              e.preventDefault();
              send(input);
            }
          }}
        />
        <Tooltip title="Send">
          <span>
            <IconButton
              color="primary"
              disabled={!canChat || busy || !input.trim()}
              onClick={() => send(input)}
            >
              <Icon icon="mdi:send" />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
    </Box>
  );
}
