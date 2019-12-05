{{ range .Values.remotes }}
  {{ if and .serviceAccountImportName .secretName }}
    {{ fail "a remote cannot have both a secretName and a serviceAccountImportName" }}
  {{ end }}
{{ end }}
