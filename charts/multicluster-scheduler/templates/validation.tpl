{{ range .Values.targets }}
  {{ if and .serviceAccountImportName .secretName }}
    {{ fail "a target cannot have both a secretName and a serviceAccountImportName" }}
  {{ end }}
{{ end }}
