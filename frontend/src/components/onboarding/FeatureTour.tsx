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
import Box from '@mui/material/Box';
import Button from '@mui/material/Button';
import Typography from '@mui/material/Typography';
import React from 'react';

const ACCENT = '#3B82F6';
const BG = '#0f172a';

interface TourStep {
  icon: string;
  title: string;
  body: string;
}

const STEPS: TourStep[] = [
  {
    icon: 'mdi:stethoscope',
    title: 'Cluster Doctor',
    body: 'Scan any cluster against 65 built-in checks across 8 categories — nodes, pods, control plane, storage, network, resources, certificates, and workloads. Severity-sorted findings in seconds.',
  },
  {
    icon: 'mdi:auto-fix',
    title: 'Guided Fix — and Undo',
    body: 'Fix safe issues with one click, behind a plain-language "here\'s what will happen" prompt. Reversible fixes (scale, uncordon) can be undone from the audit log.',
  },
  {
    icon: 'mdi:map-outline',
    title: 'Network Map',
    body: 'See what can reach what — NetworkPolicy exposure, databases, and connections inferred from app config. Live traffic overlays automatically when a service mesh is present.',
  },
  {
    icon: 'mdi:clipboard-text-clock-outline',
    title: 'What changed?',
    body: 'One timeline that merges human actions, cluster events, and scan findings — so you can reconstruct exactly what happened around an incident on a single screen.',
  },
  {
    icon: 'mdi:cash-multiple',
    title: 'Cost & Waste',
    body: 'Find idle load balancers, unused volumes, and over-provisioned workloads — with an estimated monthly waste figure. Spend that\'s provably wasted, from the Kubernetes API alone.',
  },
  {
    icon: 'mdi:update',
    title: 'Upgrade Readiness',
    body: 'Know exactly what will break before a Kubernetes upgrade — deprecated and removed APIs, listed per resource with the replacement. Turns the scariest chore into a checklist.',
  },
  {
    icon: 'mdi:script-text-play-outline',
    title: 'Runbooks',
    body: 'Governed Ansible automation against the pointed cluster — onboard a namespace, apply a default-deny policy, restart or scale a workload. Each run executes as an in-cluster Job (nothing runs on your machine, works the same on Windows), with dry-run and a full audit trail.',
  },
  {
    icon: 'mdi:robot-happy-outline',
    title: 'Copilot (offline AI)',
    body: 'An AI assistant grounded in your cluster\'s findings and its live state — ask what\'s crashing right now, how to fix the top issue, or whether it\'s safe to upgrade. Runs entirely on your hardware with a local model: no API key, no cloud, nothing leaves your network.',
  },
  {
    icon: 'mdi:bug-outline',
    title: 'Vulnerability scanning',
    body: 'Scan the container images actually running in the cluster for known CVEs with Trivy — severity-ranked, per image, with the fix version. Runs as an in-cluster Job and works fully offline when the vulnerability DB is mirrored.',
  },
  {
    icon: 'mdi:shield-check-outline',
    title: 'Compliance & CIS Benchmark',
    body: 'One-click audit against the CIS Kubernetes Benchmark — Pod Security, RBAC wildcards, cluster-admin bindings, Network Policies. A pass/fail score with per-control findings, evaluated live and fully offline.',
  },
  {
    icon: 'mdi:clipboard-list-outline',
    title: 'Full audit trail',
    body: 'Every action is recorded — who did what, when, and the result. Exportable to CSV. Built for regulated, air-gapped environments where nothing leaves the perimeter.',
  },
];

export interface FeatureTourProps {
  onDone: () => void;
}

export default function FeatureTour({ onDone }: FeatureTourProps) {
  const [step, setStep] = React.useState(0);
  const current = STEPS[step];
  const isLast = step === STEPS.length - 1;

  return (
    <Box
      sx={{
        position: 'fixed',
        inset: 0,
        zIndex: 2100,
        background: 'rgba(2,6,16,0.72)',
        backdropFilter: 'blur(4px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        px: 3,
      }}
    >
      <Box
        sx={{
          width: 480,
          maxWidth: '100%',
          p: 4,
          borderRadius: 4,
          textAlign: 'center',
          color: '#e5edf7',
          background: `radial-gradient(600px 300px at 50% 0%, #16233f 0%, ${BG} 70%)`,
          border: '1px solid rgba(148,163,184,0.15)',
          boxShadow: '0 24px 80px rgba(0,0,0,0.5)',
        }}
      >
        {/* step counter + skip */}
        <Box sx={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', mb: 2 }}>
          <Typography variant="caption" sx={{ color: '#64748b' }}>
            {step + 1} of {STEPS.length}
          </Typography>
          <Button size="small" onClick={onDone} sx={{ color: '#64748b', textTransform: 'none' }}>
            Skip tour
          </Button>
        </Box>

        {/* icon */}
        <Box
          sx={{
            width: 84,
            height: 84,
            mx: 'auto',
            mb: 2.5,
            borderRadius: '24px',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: ACCENT,
            background: 'rgba(59,130,246,0.12)',
            border: '1px solid rgba(59,130,246,0.22)',
            boxShadow: '0 0 50px rgba(59,130,246,0.28)',
          }}
        >
          <Icon icon={current.icon} width={44} height={44} />
        </Box>

        <Typography variant="h5" sx={{ fontWeight: 800, mb: 1.5 }}>
          {current.title}
        </Typography>
        <Typography sx={{ color: '#94a3b8', lineHeight: 1.7, minHeight: 96 }}>
          {current.body}
        </Typography>

        {/* progress dots */}
        <Box sx={{ display: 'flex', justifyContent: 'center', gap: 1, my: 3 }}>
          {STEPS.map((_, i) => (
            <Box
              key={i}
              onClick={() => setStep(i)}
              sx={{
                width: i === step ? 22 : 8,
                height: 8,
                borderRadius: 4,
                cursor: 'pointer',
                transition: 'width 0.2s',
                background: i === step ? ACCENT : 'rgba(148,163,184,0.3)',
              }}
            />
          ))}
        </Box>

        {/* nav */}
        <Box sx={{ display: 'flex', gap: 1.5 }}>
          <Button
            variant="outlined"
            disabled={step === 0}
            onClick={() => setStep(s => Math.max(0, s - 1))}
            sx={{
              flex: 1,
              textTransform: 'none',
              color: '#cbd5e1',
              borderColor: 'rgba(148,163,184,0.3)',
            }}
          >
            Back
          </Button>
          <Button
            variant="contained"
            onClick={() => (isLast ? onDone() : setStep(s => s + 1))}
            sx={{
              flex: 1,
              fontWeight: 700,
              textTransform: 'none',
              background: ACCENT,
              '&:hover': { background: '#2f6fe0' },
            }}
          >
            {isLast ? "Let's go" : 'Next'}
          </Button>
        </Box>
      </Box>
    </Box>
  );
}
