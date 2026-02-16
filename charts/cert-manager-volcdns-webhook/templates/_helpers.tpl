{{/* vim: set filetype=mustache: */}}
{{/*
Expand the name of the chart.
*/}}
{{- define "cert-manager-volcdns-webhook.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create a default fully qualified app name.
We truncate at 63 chars because some Kubernetes name fields are limited to this (by the DNS naming spec).
If release name contains chart name it will be used as a full name.
*/}}
{{- define "cert-manager-volcdns-webhook.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- $name := default .Chart.Name .Values.nameOverride -}}
{{- if contains $name .Release.Name -}}
{{- .Release.Name | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- printf "%s-%s" .Release.Name $name | trunc 63 | trimSuffix "-" -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Create chart name and version as used by the chart label.
*/}}
{{- define "cert-manager-volcdns-webhook.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Create the name of the service account to use
*/}}
{{- define "cert-manager-volcdns-webhook.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
    {{ default (include "cert-manager-volcdns-webhook.fullname" .) .Values.serviceAccount.name }}
{{- else -}}
    {{ default "default" .Values.serviceAccount.name }}
{{- end -}}
{{- end -}}

{{- define "cert-manager-volcdns-webhook.selfSignedIssuer" -}}
{{ printf "%s-self-sign" (include "cert-manager-volcdns-webhook.fullname" .) }}
{{- end -}}

{{- define "cert-manager-volcdns-webhook.rootCAIssuer" -}}
{{ printf "%s-ca" (include "cert-manager-volcdns-webhook.fullname" .) }}
{{- end -}}

{{- define "cert-manager-volcdns-webhook.servingCertificate" -}}
{{ printf "%s-webhook-tls" (include "cert-manager-volcdns-webhook.fullname" .) }}
{{- end -}}
