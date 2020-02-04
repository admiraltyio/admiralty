{{ range .Values.clusters }}
  {{ if and .serviceAccountImportName .secretName }}
    {{ fail "a cluster cannot have both a secretName and a serviceAccountImportName" }}
  {{ end }}
{{ end }}
