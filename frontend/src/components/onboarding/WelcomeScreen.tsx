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
import Chip from '@mui/material/Chip';
import Typography from '@mui/material/Typography';
import React from 'react';

const ACCENT = '#3B82F6';
const BG = '#0f172a';

const FEATURES: { icon: string; label: string }[] = [
  { icon: 'mdi:stethoscope', label: 'Cluster Doctor' },
  { icon: 'mdi:auto-fix', label: 'Guided Fix' },
  { icon: 'mdi:map-outline', label: 'Network Map' },
  { icon: 'mdi:clipboard-text-clock-outline', label: 'Audit & Timeline' },
];

/** Decorative tile grid on the right, echoing the app's capabilities. */
function FeatureTiles() {
  return (
    <Box
      sx={{
        display: 'grid',
        gridTemplateColumns: 'repeat(2, 96px)',
        gridAutoRows: '96px',
        gap: 1.5,
      }}
    >
      {FEATURES.map((f, i) => (
        <Box
          key={f.label}
          title={f.label}
          sx={{
            borderRadius: 4,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: ACCENT,
            background: 'rgba(59,130,246,0.10)',
            border: '1px solid rgba(59,130,246,0.18)',
            boxShadow: i === 0 ? `0 0 40px rgba(59,130,246,0.25)` : 'none',
          }}
        >
          <Icon icon={f.icon} width={40} height={40} />
        </Box>
      ))}
    </Box>
  );
}

export interface WelcomeScreenProps {
  onGetStarted: () => void;
  airGapped?: boolean;
}

export default function WelcomeScreen({ onGetStarted, airGapped }: WelcomeScreenProps) {
  const [detectedAirGapped, setDetectedAirGapped] = React.useState(false);

  React.useEffect(() => {
    if (airGapped !== undefined) return;
    let cancelled = false;
    // Best-effort: read the backend's air-gapped flag for the badge. Silent on failure.
    fetch('/config')
      .then(r => (r.ok ? r.json() : null))
      .then(cfg => {
        if (!cancelled && cfg && typeof cfg.airGapped === 'boolean') {
          setDetectedAirGapped(cfg.airGapped);
        }
      })
      .catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [airGapped]);

  const isAirGapped = airGapped ?? detectedAirGapped;

  return (
    <Box
      sx={{
        position: 'fixed',
        inset: 0,
        zIndex: 2000,
        background: `radial-gradient(1200px 700px at 70% 40%, #14213b 0%, ${BG} 60%)`,
        color: '#e5edf7',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        gap: { xs: 0, md: 10 },
        px: 3,
      }}
    >
      {/* Left: welcome card */}
      <Box
        sx={{
          width: 460,
          maxWidth: '100%',
          p: 5,
          borderRadius: 4,
          background: 'rgba(17,26,46,0.55)',
          border: '1px solid rgba(148,163,184,0.12)',
          backdropFilter: 'blur(6px)',
        }}
      >
        <Typography variant="h4" sx={{ fontWeight: 800, mb: 1 }}>
          Welcome to K8sense
        </Typography>
        <Typography sx={{ color: '#94a3b8', mb: 3 }}>
          Diagnose, fix, and secure your Kubernetes clusters — from your machine.
        </Typography>

        {isAirGapped && (
          <Chip
            icon={<Icon icon="mdi:shield-lock-outline" />}
            label="Air-gapped — nothing leaves this machine"
            size="small"
            sx={{
              mb: 3,
              color: '#86efac',
              background: 'rgba(34,197,94,0.10)',
              border: '1px solid rgba(34,197,94,0.25)',
              '& .MuiChip-icon': { color: '#86efac' },
            }}
          />
        )}

        <Button
          fullWidth
          variant="contained"
          onClick={onGetStarted}
          sx={{
            py: 1.3,
            fontWeight: 700,
            textTransform: 'none',
            background: ACCENT,
            '&:hover': { background: '#2f6fe0' },
          }}
        >
          Get started
        </Button>

        <Typography variant="body2" sx={{ color: '#94a3b8', mt: 2, lineHeight: 1.6 }}>
          Connect with your <strong>kubeconfig</strong>, or your organization&apos;s{' '}
          <strong>SSO (OIDC)</strong> — configured per cluster. No account, no cloud, no email.
        </Typography>
      </Box>

      {/* Right: decorative tiles (hidden on small screens) */}
      <Box sx={{ display: { xs: 'none', md: 'block' } }}>
        <FeatureTiles />
      </Box>
    </Box>
  );
}
