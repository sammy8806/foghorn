# Resolvers

[Back to README](../README.md) · [Configuration](configuration.md)

### Resolver subprocesses

Resolvers may call a local process to replace a field value for display,
grouping, sorting, and filtering. The executable, arguments, and environment are
static. Alert data goes to the process on standard input with an explicit
`stdin: value` or `stdin: json` setting. The JSON form has this shape:

```json
{
  "version": 1,
  "ref": "label:cluster",
  "kind": "label",
  "name": "cluster",
  "value": "cluster-01",
  "alert": {
    "id": "8f4d",
    "source": "production",
    "sourceType": "alertmanager",
    "name": "HighCPU",
    "severity": "warning",
    "state": "firing",
    "labels": {"alertname": "HighCPU", "cluster": "cluster-01"},
    "annotations": {"summary": "CPU usage is high"},
    "startsAt": "2026-08-24T12:00:00Z",
    "updatedAt": "2026-08-24T12:05:00Z",
    "generatorURL": "https://prometheus.example/graph",
    "silencedBy": [],
    "inhibitedBy": [],
    "receivers": ["on-call"]
  },
  "labels": {"alertname": "HighCPU", "cluster": "cluster-01"},
  "annotations": {"summary": "CPU usage is high"}
}
```

Foghorn rejects templates in resolver `command`, `args`, and `env`. Migrate an
old resolver that passed `"{{.Value}}"` as an argument by removing that argument,
setting `stdin: value`, and updating the resolver program to read standard input.
The program must treat that input as data, not shell or interpreter source code.
