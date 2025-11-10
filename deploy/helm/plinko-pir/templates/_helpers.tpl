{{/*
Expand the name of the chart.
*/}}
{{- define "plinko-pir.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "plinko-pir.fullname" -}}
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
{{- define "plinko-pir.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "plinko-pir.labels" -}}
helm.sh/chart: {{ include "plinko-pir.chart" . }}
{{ include "plinko-pir.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "plinko-pir.selectorLabels" -}}
app.kubernetes.io/name: {{ include "plinko-pir.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Component-specific labels
*/}}
{{- define "plinko-pir.componentLabels" -}}
{{- $component := .component }}
{{- $root := .root }}
app.kubernetes.io/component: {{ $component }}
{{ include "plinko-pir.labels" $root }}
{{- end }}

{{/*
Component-specific selector labels
*/}}
{{- define "plinko-pir.componentSelectorLabels" -}}
{{- $component := .component }}
{{- $root := .root }}
app.kubernetes.io/component: {{ $component }}
{{ include "plinko-pir.selectorLabels" $root }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "plinko-pir.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "plinko-pir.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
Storage class name
*/}}
{{- define "plinko-pir.storageClass" -}}
{{- if .Values.global.storageClass }}
{{- .Values.global.storageClass }}
{{- else if .Values.persistence.storageClass }}
{{- .Values.persistence.storageClass }}
{{- else }}
{{- "vultr-block-storage" }}
{{- end }}
{{- end }}
