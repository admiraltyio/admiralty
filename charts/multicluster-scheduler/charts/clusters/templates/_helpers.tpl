{{- define "clusterNamespacedName" }}
{{- if .useClusterNamespaces }}
name: member
namespace: {{ .name }}
{{- else }}
name: {{ .name }}
{{- end }}
{{- end }}
