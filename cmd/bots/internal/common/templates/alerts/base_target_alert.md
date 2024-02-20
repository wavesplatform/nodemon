```yaml
❌ Base Target Alert
Base target is greater than the threshold value. The threshold value is {{ .Threshold }}
{{ with .BaseTargetValues }}{{ range . }}
Node: {{ .Node}}
Base Target: {{ .BaseTarget}}
{{end}}
{{end}}
```
