{{- define "avatar-service.name" -}}
{{- default .Chart.Name .Values.nameOverride | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{- define "avatar-service.fullname" -}}
{{- if .Values.fullnameOverride -}}
{{- .Values.fullnameOverride | trunc 63 | trimSuffix "-" -}}
{{- else -}}
{{- include "avatar-service.name" . -}}
{{- end -}}
{{- end -}}

{{- define "avatar-service.labels" -}}
app.kubernetes.io/name: {{ include "avatar-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
helm.sh/chart: {{ .Chart.Name }}-{{ .Chart.Version | replace "+" "_" }}
{{- end -}}

{{- define "avatar-service.selectorLabels" -}}
app.kubernetes.io/name: {{ include "avatar-service.name" . }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end -}}

{{- define "avatar-service.serviceAccountName" -}}
{{- if .Values.serviceAccount.create -}}
{{- default (include "avatar-service.fullname" .) .Values.serviceAccount.name -}}
{{- else -}}
{{- default "default" .Values.serviceAccount.name -}}
{{- end -}}
{{- end -}}

{{- define "avatar-service.postgresDsn" -}}
{{- if .Values.secret.postgresDsn -}}
{{- .Values.secret.postgresDsn -}}
{{- else -}}
postgres://{{ .Values.postgresql.username }}:{{ .Values.postgresql.password }}@{{ include "avatar-service.fullname" . }}-postgresql:5432/{{ .Values.postgresql.database }}?sslmode={{ .Values.postgresql.sslmode }}
{{- end -}}
{{- end -}}

{{- define "avatar-service.rabbitmqUrl" -}}
{{- if .Values.secret.rabbitmqUrl -}}
{{- .Values.secret.rabbitmqUrl -}}
{{- else -}}
amqp://{{ .Values.rabbitmq.username }}:{{ .Values.rabbitmq.password }}@{{ include "avatar-service.fullname" . }}-rabbitmq:5672/
{{- end -}}
{{- end -}}
