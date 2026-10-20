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
"true" when the value is true or "true" (config.* booleans are often quoted), empty otherwise.
*/}}
{{- define "static-file-server.isTrue" -}}
{{- if eq (toString .) "true" }}true{{ end }}
{{- end }}

{{/*
HTTPRoute parentRefs entries for a list of refs.
*/}}
{{- define "static-file-server.parentRefs" -}}
{{- range . }}
- name: {{ required "parentRefs[].name is required (httproute.parentRefs or httproute.httpsRedirect.parentRefs)" .name }}
  {{- with .namespace }}
  namespace: {{ . }}
  {{- end }}
  {{- with .sectionName }}
  sectionName: {{ . }}
  {{- end }}
  {{- with .port }}
  port: {{ . }}
  {{- end }}
{{- end }}
{{- end }}

{{/*
HTTPRoute hostnames entries for a list of hostnames.
*/}}
{{- define "static-file-server.hostnames" -}}
{{- range . }}
- {{ . | quote }}
{{- end }}
{{- end }}

{{/*
"true" in cache-only mode, empty otherwise. Validates mode and the values it excludes.
*/}}
{{- define "static-file-server.cacheOnly" -}}
{{- $mode := .Values.mode | default "all" -}}
{{- if not (has $mode (list "all" "cache")) }}
{{- fail (printf "mode must be \"all\" or \"cache\", got %q" $mode) }}
{{- end }}
{{- if eq $mode "cache" }}
{{- if not .Values.s3Cache.enabled }}
{{- fail "mode: cache requires s3Cache.enabled=true" }}
{{- end }}
{{- if or .Values.httproute.enabled .Values.ingress.enabled }}
{{- fail "mode: cache serves no files: disable httproute and ingress" }}
{{- end -}}
true
{{- end }}
{{- end }}

{{/*
Name of the content claim used by persistence (chart-created or existing).
*/}}
{{- define "static-file-server.dataClaimName" -}}
{{- .Values.persistence.existingClaim | default (printf "%s-data" (include "static-file-server.fullname" .)) -}}
{{- end }}

{{/*
Claims mounted as served content, as a JSON list. Mirrors the volume selection in
deployment.yaml so the s3Cache guard checks exactly what is mounted.
*/}}
{{- define "static-file-server.servedClaims" -}}
{{- $served := list -}}
{{- if .Values.persistentVolumeClaims.enabled -}}
{{- range .Values.persistentVolumeClaims.items }}{{ $served = append $served .name }}{{ end -}}
{{- else if .Values.persistence.enabled -}}
{{- $served = append $served (include "static-file-server.dataClaimName" .) -}}
{{- end -}}
{{- toJson $served -}}
{{- end }}

{{/*
S3 cache endpoint env vars. Fails when the cache would be reachable through the
file server (same claim as the served content without a hidden dot subPath), or
when a ReadWriteOnce cache claim would be shared by several replicas.
*/}}
{{- define "static-file-server.s3CacheEnv" -}}
{{- $c := .Values.s3Cache -}}
{{- $secret := required "s3Cache.existingSecret is required when s3Cache.enabled=true" $c.existingSecret -}}
{{- if $c.existingClaim }}
{{- $served := include "static-file-server.servedClaims" . | fromJsonArray -}}
{{- /* The first subPath segment must be a dot directory: ".runner-cache", not "./cache". */ -}}
{{- $hidden := and (include "static-file-server.isTrue" .Values.config.hideDotFiles) (regexMatch "^\\.[^./]" (toString $c.subPath)) -}}
{{- if and (has $c.existingClaim $served) (not $hidden) }}
{{- fail "s3Cache.existingClaim is also served as content: set s3Cache.subPath to a dot directory (e.g. .runner-cache) and config.hideDotFiles=\"true\", or use a separate claim" -}}
{{- end }}
{{- else if and $c.persistence.enabled (gt (int .Values.replicaCount) 1) (eq $c.persistence.accessMode "ReadWriteOnce") }}
{{- fail "s3Cache.persistence.accessMode=ReadWriteOnce cannot be shared by replicaCount > 1: use ReadWriteMany" -}}
{{- end -}}
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
