{{- define "clusterNamespacedName" }}
{{- if .useClusterNamespaces }}
name: member
namespace: {{ .clusterNamespace | default .name }}
{{- else }}
name: {{ .name }}
{{- end }}
{{- end }}
