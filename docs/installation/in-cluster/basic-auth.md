---
title: Basic authentication with username and password
sidebar_label: Basic Auth
---

K8sense does not provide a built-in username/password authentication mechanism.
When running K8sense in-cluster and exposing it via a public or internal URL,
it is recommended to protect access using **ingress-level basic authentication**.

This approach adds a simple username/password gate **before the K8sense UI
loads**, while K8sense itself continues to rely on Kubernetes authentication
(tokens, certificates) and RBAC for authorization.

---

## When should this be used?

Ingress-level basic authentication is useful when:

- K8sense is exposed via an Ingress, LoadBalancer, or NodePort
- You want to prevent unauthenticated users from accessing the UI
- You do not want to configure OIDC or an external identity provider
- The dashboard is intended for internal, demo, or small-team use

This is commonly used for:

- In-cluster deployments
- Helm-based installations
- Local or internal Kubernetes dashboards

---

## How it works

With basic authentication enabled at the ingress level, the request flow looks
like this:

1. A user accesses the K8sense URL
2. The ingress controller prompts for a username and password
3. If authentication succeeds, the request is forwarded to K8sense
4. K8sense then performs its usual Kubernetes authentication and authorization

The username/password check is handled entirely by the ingress controller.
K8sense never sees or manages these credentials.

---

## Example: NGINX Ingress basic authentication

The following example shows how to protect an in-cluster K8sense deployment
using NGINX Ingress basic authentication.

### 1. Create a password file

Use `htpasswd` to create a username and password:
```bash
htpasswd -c auth admin
```

This creates a file named `auth` containing a hashed password.

> **ℹ️ Note:** Multiple users can be added by running `htpasswd auth <username>` again (without the `-c` flag).

### 2. Store credentials in a Kubernetes Secret

Create a secret from the password file:
```bash
kubectl create secret generic k8sense-basic-auth \
  --from-file=auth \
  -n <k8sense-namespace>
```

### 3. Configure Ingress with basic authentication

Add the following annotations to the ingress resource that exposes K8sense:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: k8sense-ingress
  namespace: <k8sense-namespace>
  annotations:
    nginx.ingress.kubernetes.io/auth-type: basic
    nginx.ingress.kubernetes.io/auth-secret: k8sense-basic-auth
    nginx.ingress.kubernetes.io/auth-realm: "Authentication Required"
spec:
  ingressClassName: nginx
  rules:
  - host: k8sense.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: k8sense
            port:
              number: 80
```

After applying this configuration, users accessing the K8sense URL will be
prompted by their browser to enter a username and password.

---

## Password management

- Passwords are managed by cluster administrators, not by K8sense
- Credentials are stored securely in Kubernetes Secrets
- Passwords can be rotated by updating the secret without restarting K8sense
- Multiple users can be supported by adding entries to the password file

To update a password:
```bash
# Update the password file
htpasswd auth admin

# Update the secret
kubectl create secret generic k8sense-basic-auth \
  --from-file=auth \
  -n <k8sense-namespace> \
  --dry-run=client -o yaml | kubectl apply -f -
```

---

## Important notes

- **Basic authentication only controls access to the UI**. K8sense will still require Kubernetes authentication (token or kubeconfig) after the UI loads.
- **Authorization within the UI is controlled entirely by Kubernetes RBAC**. The username/password does not determine what resources users can view or modify.
- This approach is intended for simple access control and is **not a replacement for full identity management solutions**.
- For environments requiring per-user identity, auditing, or single sign-on, [OIDC-based authentication](../oidc/) is recommended instead.

---

## Other ingress controllers

A similar setup can be achieved using other ingress controllers such as **Traefik**
by configuring their respective basic authentication middleware.

### Example: Traefik basic authentication

For Traefik, you can use a similar approach with middleware:
```yaml
apiVersion: traefik.io/v1alpha1
kind: Middleware
metadata:
  name: k8sense-basic-auth
  namespace: <k8sense-namespace>
spec:
  basicAuth:
    secret: k8sense-basic-auth
```

Then reference the middleware in your Ingress:
```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: k8sense-ingress
  namespace: <k8sense-namespace>
  annotations:
    traefik.ingress.kubernetes.io/router.middlewares: <k8sense-namespace>-k8sense-basic-auth@kubernetescrd
spec:
  rules:
  - host: k8sense.example.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: k8sense
            port:
              number: 80
```
