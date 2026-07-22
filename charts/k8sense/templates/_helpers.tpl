{{/*
Expand the name of the chart.
*/}}
{{- define "k8sense.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "k8sense.fullname" -}}
{{- if .Values.fullnameOverride }}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- $name := default .Chart.Name .Values.nameOverride }}
{{- if contains $name .Release.Name }}
{{- .Release.Name | trunc 63 | trimSuffix "-" }}
{{- else }}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" }}
{{- end }}
{{- end }}
{{- end }}

{{/*
Expand the namespace of the release.
Allows overriding it for multi-namespace deployments in combined charts.
*/}}
{{- define "k8sense.namespace" -}}
  {{- if .Values.namespaceOverride }}
    {{- .Values.namespaceOverride | trunc 63 | trimSuffix "-" -}}
  {{- else if .Release.Namespace }}
    {{- .Release.Namespace | trunc 63 | trimSuffix "-" -}}
  {{- else -}}
    default
  {{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "k8sense.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "k8sense.labels" -}}
helm.sh/chart: {{ include "k8sense.chart" . }}
{{ include "k8sense.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "k8sense.selectorLabels" -}}
app.kubernetes.io/name: {{ include "k8sense.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "k8sense.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "k8sense.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}


{{/*
Check if readOnlyRootFilesystem is enabled, returns string "true" if enabled, otherwise returns "false".
*/}}
{{- define "k8sense.readOnlyRootFilesystem" -}}
{{- $securityContextReadOnly := and .securityContext (hasKey .securityContext "readOnlyRootFilesystem") .securityContext.readOnlyRootFilesystem -}}
{{- if $securityContextReadOnly -}}true{{- else -}}false{{- end -}}
{{- end }}

{{/*
Compute whether to auto-add a writable /tmp emptyDir for a container with
readOnlyRootFilesystem: true.

- addMount is false when the user already has a volumeMount at /tmp
  (avoids duplicate mountPath).
- addVolume is false when the user already has a /tmp mount (avoids an
  orphaned volume) OR when a volume with mountName already exists (allows
  users to supply their own k8sense-tmp with custom emptyDir settings
  such as sizeLimit, while the chart still wires up the /tmp mount).

Input (dict):
  volumeMounts - list of existing volumeMounts for this container
  volumes      - list of existing pod-level volumes
  readOnly     - bool: is readOnlyRootFilesystem active for this container
  mountName    - string: name for the auto-created volume (e.g. "k8sense-tmp")

Output (YAML dict, intended for use with fromYaml):
  addMount: bool
  addVolume: bool
*/}}
{{- define "k8sense.tmpVolumeContext" -}}
{{- $hasTmpMount := false -}}
{{- range .volumeMounts -}}
  {{- if eq .mountPath "/tmp" -}}{{- $hasTmpMount = true -}}{{- end -}}
{{- end -}}
{{- $hasTmpVolume := false -}}
{{- range .volumes -}}
  {{- if eq .name $.mountName -}}{{- $hasTmpVolume = true -}}{{- end -}}
{{- end -}}
addMount: {{ and .readOnly (not $hasTmpMount) }}
addVolume: {{ and .readOnly (not $hasTmpMount) (not $hasTmpVolume) }}
{{- end }}
