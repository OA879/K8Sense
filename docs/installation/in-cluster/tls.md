---
title: TLS and TLS pass through
sidebar_label: TLS
---

## TLS Termination at K8sense Backend

K8sense supports optional TLS termination at the backend server. This terminating TLS either at the ingress (default) or directly at the K8sense container, enabling use cases such as NGINX TLS passthrough and transport server.

### Enabling TLS at the Backend

To enable TLS termination at the K8sense backend, set the following environment variables in your deployment or container:

- `K8SENSE_CONFIG_TLS_CERT_PATH=/path/to/tls.crt` — Path to the TLS certificate file
- `K8SENSE_CONFIG_TLS_KEY_PATH=/path/to/tls.key` — Path to the TLS private key file

Instead of environment variables you could also add arguments `-tls-cert-path` and `-tls-key-path` to k8sense-server.


Example (Kubernetes manifest snippet):

```yaml
containers:
  - name: k8sense
    image: ...
    env:
      ...
    #   - name: K8SENSE_CONFIG_TLS_CERT_PATH
    #     value: "/certs/tls.crt"
    #   - name: K8SENSE_CONFIG_TLS_KEY_PATH
    #     value: "/certs/tls.key"
    args:
      ...
      - "-tls-cert-path=/certs/tls.crt"
      - "-tls-key-path=/certs/tls.key"
    volumeMounts:
      - name: certs
        mountPath: /certs
volumes:
  - name: certs
    secret:
      secretName: k8sense-tls
```

### K8sense Helm Chart Example

If you are using the k8sense helm chart, you can configure it like this:

```yaml
config:
  tlsCertPath: "/k8sense-cert/k8sense-ca.crt"
  tlsKeyPath: "/k8sense-cert/k8sense-tls.key"

volumes:
  - name: "k8sense-cert"
    secret:
      secretName: "k8sense-tls"
      items:
        - key: "tls.crt"
          path: "k8sense-ca.crt"
        - key: "tls.key"
          path: "k8sense-tls.key"

volumeMounts:
  - name: "k8sense-cert"
    mountPath: "/k8sense-cert"
```

### Notes

- If `K8SENSE_CONFIG_TLS_CERT_PATH` and `K8SENSE_CONFIG_TLS_KEY_PATH` are not set, K8sense will listen without TLS (default behavior).
- You can now use NGINX or other ingress controllers in TLS passthrough mode, letting K8sense terminate TLS.

### Optional Compatibility

- This feature is optional and fully backward compatible. If you do not set these variables, K8sense will continue to expect TLS termination at the ingress.

### See Also

- [In-cluster installation guide](https://k8sense.dev/docs/latest/installation/in-cluster/)
- [Kubernetes TLS Secrets](https://kubernetes.io/docs/concepts/configuration/secret/#tls-secrets)
