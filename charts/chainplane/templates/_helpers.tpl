{{/*
Expand the name of the chart.
*/}}
{{- define "chainplane.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "chainplane.fullname" -}}
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
Create chart name and version as used by the chart label.
*/}}
{{- define "chainplane.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "chainplane.labels" -}}
helm.sh/chart: {{ include "chainplane.chart" . }}
{{ include "chainplane.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "chainplane.selectorLabels" -}}
app.kubernetes.io/name: {{ include "chainplane.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "chainplane.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "chainplane.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Operator image
*/}}
{{- define "chainplane.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Leader election role name
*/}}
{{- define "chainplane.leaderElectionRoleName" -}}
{{- printf "%s-leader-election" (include "chainplane.fullname" .) }}
{{- end }}

{{/*
Metrics service name
*/}}
{{- define "chainplane.metricsServiceName" -}}
{{- printf "%s-metrics" (include "chainplane.fullname" .) }}
{{- end }}

{{/*
Webhook service name
*/}}
{{- define "chainplane.webhookServiceName" -}}
{{- printf "%s-webhook" (include "chainplane.fullname" .) }}
{{- end }}
