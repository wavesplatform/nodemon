```yaml
Alert type: Base Target

Level: Error ❌

Details: Base target is greater than the threshold value. The threshold value is {{ .Threshold}}

{{ with .BaseTargetValues }}
{{range .}}
Node: <code>{{ .Node}}</code>
Base Target: <code>{{ .BaseTarget}}</code>
{{end}}
```
