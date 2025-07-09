# TemplateSet

TemplateSet acts like DaemonSet with super powers.

* Control pod names with templates
* Set pod labels with templates
* Set pod environment variables with templates
* Optionally create a service per pod

## Example

```yaml
apiVersion: templateset.keen.land/v1
kind: TemplateSet
metadata:
  name: example
  labels:
    app: example
spec:
  selector:
    matchLabels:
      app: example
  template:
    metadata:
      name: "example-{{index .Node.Labels \"topology.kubernetes.io/zone\"}}"
      labels:
        app: example
        zone: "{{index .Node.Labels \"topology.kubernetes.io/zone\"}}"
    spec:
      containers:
        - name: example
          image: "hello-world"
          env:
            - name: HELLO
              value: "{{index .Node.Labels \"topology.kubernetes.io/zone\"}}"
  servicePerPod:
    enabled: true
```

This will create one `hello-world` pod on every node in the cluster named, eg, `example-us-east1a-fzfx7`.
It will also create a service for each pod, eg, `example-us-east1a-fzfx7-service`.

Note: the `name` supplied in the pod template metadata will always have a deterministic suffix applied.

Pods will always have a `templateset.keen.land/podname` label attached which will be used as the selector for the created service.
`TemplateSet` will attempt to detect non-unique service names and apply a deterministic suffix.
