{{/*
Expand the name of the chart.
*/}}
{{- define "blockchain-node-operator.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
*/}}
{{- define "blockchain-node-operator.fullname" -}}
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
{{- define "blockchain-node-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "blockchain-node-operator.labels" -}}
helm.sh/chart: {{ include "blockchain-node-operator.chart" . }}
{{ include "blockchain-node-operator.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "blockchain-node-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ include "blockchain-node-operator.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
control-plane: controller-manager
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "blockchain-node-operator.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "blockchain-node-operator.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Operator image
*/}}
{{- define "blockchain-node-operator.image" -}}
{{- $tag := default .Chart.AppVersion .Values.image.tag -}}
{{- printf "%s:%s" .Values.image.repository $tag }}
{{- end }}

{{/*
Leader election role name
*/}}
{{- define "blockchain-node-operator.leaderElectionRoleName" -}}
{{- printf "%s-leader-election" (include "blockchain-node-operator.fullname" .) }}
{{- end }}

{{/*
Metrics service name
*/}}
{{- define "blockchain-node-operator.metricsServiceName" -}}
{{- printf "%s-metrics" (include "blockchain-node-operator.fullname" .) }}
{{- end }}

{{/*
Webhook service name
*/}}
{{- define "blockchain-node-operator.webhookServiceName" -}}
{{- printf "%s-webhook" (include "blockchain-node-operator.fullname" .) }}
{{- end }}
