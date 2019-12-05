{{- define "globalname" -}}
  {{- default .Values.global.chart.name .Values.global.nameOverride -}}
{{- end -}}

{{- define "globalfullname" -}}
  {{- if .Values.global.fullnameOverride -}}
    {{- .Values.global.fullnameOverride -}}
  {{- else -}}
    {{- $name := (include "globalname" .) -}}
    {{- if contains $name .Release.Name -}}
      {{- .Release.Name -}}
    {{- else -}}
      {{- printf "%s-%s" .Release.Name $name -}}
    {{- end -}}
  {{- end -}}
{{- end -}}

{{/*
Expand the name of the chart.
*/}}
{{- define "name" -}}
  {{- $truncName := include "globalname" . | trunc (.Chart.Name | len | sub 63 | int) | trimSuffix "-" -}}
  {{- printf "%s-%s" $truncName .Chart.Name -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "fullname" -}}
  {{- $truncFullName := include "globalfullname" . | trunc (.Chart.Name | len | sub 63 | int) | trimSuffix "-" -}}
  {{- printf "%s-%s" $truncFullName .Chart.Name -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "chart" -}}
{{- printf "%s-%s" .Values.global.chart.name .Values.global.chart.version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common labels
*/}}
{{- define "labels" -}}
helm.sh/chart: {{ include "chart" . }}
{{ include "selectorLabels" . }}
{{- if .Values.global.chart.appVersion }}
app.kubernetes.io/version: {{ .Values.global.chart.appVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: {{ include "globalname" . }}
{{- end -}}

{{/*
Selector labels
*/}}
{{- define "selectorLabels" -}}
app.kubernetes.io/name: {{ include "name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}
