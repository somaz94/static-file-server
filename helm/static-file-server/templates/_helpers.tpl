{{/*
Expand the name of the chart.
*/}}
{{- define "static-file-server.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Create a default fully qualified app name.
*/}}
{{- define "static-file-server.fullname" -}}
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
{{- define "static-file-server.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "static-file-server.labels" -}}
helm.sh/chart: {{ include "static-file-server.chart" . }}
{{ include "static-file-server.selectorLabels" . }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "static-file-server.selectorLabels" -}}
app.kubernetes.io/name: {{ include "static-file-server.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}

{{/*
Create the name of the service account to use
*/}}
{{- define "static-file-server.serviceAccountName" -}}
{{- if .Values.serviceAccount.create }}
{{- default (include "static-file-server.fullname" .) .Values.serviceAccount.name }}
{{- else }}
{{- default "default" .Values.serviceAccount.name }}
{{- end }}
{{- end }}

{{/*
S3 cache endpoint env vars. Fails when the cache would be reachable through the
file server: same claim as the served content without a hidden dot subPath.
*/}}
{{- define "static-file-server.s3CacheEnv" -}}
{{- $c := .Values.s3Cache -}}
{{- $secret := required "s3Cache.existingSecret is required when s3Cache.enabled=true" $c.existingSecret -}}
{{- if $c.existingClaim }}
{{- $served := list .Values.persistence.existingClaim -}}
{{- if .Values.persistentVolumeClaims.enabled }}
{{- range .Values.persistentVolumeClaims.items }}{{ $served = append $served .name }}{{ end }}
{{- end }}
{{- if and (has $c.existingClaim $served) (not (and (eq (toString .Values.config.hideDotFiles) "true") (hasPrefix "." (toString $c.subPath)))) }}
{{- fail "s3Cache.existingClaim is also served as content: set s3Cache.subPath to a dot directory (e.g. .runner-cache) and config.hideDotFiles=\"true\", or use a separate claim" -}}
{{- end }}
{{- end }}
- name: S3_ENABLED
  value: "true"
- name: S3_PORT
  value: {{ $c.port | quote }}
- name: S3_FOLDER
  value: {{ $c.folder | quote }}
- name: S3_ACCESS_KEY
  valueFrom:
    secretKeyRef:
      name: {{ $secret }}
      key: {{ $c.accessKeyKey }}
- name: S3_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: {{ $secret }}
      key: {{ $c.secretKeyKey }}
{{- with $c.buckets }}
- name: S3_BUCKETS
  value: {{ . | quote }}
{{- end }}
{{- with $c.ttl }}
- name: S3_TTL
  value: {{ . | quote }}
{{- end }}
{{- with $c.maxObjectSize }}
- name: S3_MAX_OBJECT_SIZE
  value: {{ . | quote }}
{{- end }}
{{- with $c.maxTotalSize }}
- name: S3_MAX_TOTAL_SIZE
  value: {{ . | quote }}
{{- end }}
{{- end }}
