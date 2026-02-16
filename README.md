# cert-manager-volcdns-webhook

`cert-manager-volcdns-webhook` is a webhook that provides ACME DNS01 validation for cert-manager. It automatically creates and cleans up TXT records through Volcengine Public DNS services to complete the certificate issuance flow.

Key features:

- Implements the cert-manager ACME DNS01 webhook interface
- Uses Volcengine Public DNS APIs to manage DNS records
- Supports both AK/SK and IRSA authentication
- Allows specifying a Role TRN in the Issuer to perform dynamic IRSA authentication
- Automatically escapes special characters in TXT record values (such as double quotes or the `heritage=` prefix)
- Includes a Helm chart for convenient deployment

### Prerequisites

- A Kubernetes cluster with cert-manager already installed (version >= v1.15 is recommended)
- The cluster must be able to access Volcengine Public DNS APIs
- The environment where the Pods run must meet one of the following conditions:
  - Can access a Kubernetes Secret containing AK/SK credentials
  - IRSA is configured (ServiceAccount is bound to an appropriate Volcengine IAM role)

### Build

In the project root [cert-manager-volcdns-webhook](file:///home/maple/code/ai/trae/cert-manager-volcdns-webhook):

```bash
go build -o webhook .
```

The built binary will be output as a `webhook` file in the current directory.

### Docker image build

The project root provides a [Dockerfile](file:///home/maple/code/ai/trae/cert-manager-volcdns-webhook/Dockerfile) using multi-stage builds:

```bash
docker build -t your-registry/cert-manager-volcdns-webhook:latest .
```

The image entrypoint is `/webhook`. You must specify the webhook group name via the `GROUP_NAME` environment variable.

### Deployment (Helm Chart)

This project includes a Helm chart located in the `deploy/cert-manager-volcdns-webhook` directory.

1. **Install the chart**

   ```bash
   helm install cert-manager-volcdns-webhook ./deploy/cert-manager-volcdns-webhook \
     --namespace cert-manager \
     --set groupName=acme.yourcompany.com \
     --set image.repository=your-registry/cert-manager-volcdns-webhook \
     --set image.tag=latest
   ```

   **Note**: `groupName` must match the `groupName` referenced in the ClusterIssuer.

### Authentication methods

The authentication logic is implemented in [solver.go](file:///home/maple/code/ai/trae/cert-manager-volcdns-webhook/volcengine/solver.go):

1. **AK/SK (highest priority)**:
   - Use `accessKeySecretRef` and `secretKeySecretRef` to specify the Secret
   - Provide `access-key` and `secret-key` (or custom key names) in the Secret `data` fields

2. **Dynamic IRSA (recommended)**:
   - Specify `roleTrn` in `config`
   - The webhook uses the Pod's base OIDC credentials to assume this role
   - Suitable for multi-tenant scenarios where different Issuers can use different roles

3. **Static IRSA (default)**:
   - If neither a Secret nor `roleTrn` is configured
   - The webhook directly uses configuration from environment variables on the Pod (injected via ServiceAccount annotations)

### Issuer configuration example

The following is a simplified ClusterIssuer example showing how to use this webhook with cert-manager:

#### Option 1: use AK/SK

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-volcengine
spec:
  acme:
    email: you@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-account-key
    solvers:
      - dns01:
          webhook:
            groupName: acme.yourcompany.com
            solverName: volcdns-resolver
            config:
              region: cn-beijing
              zoneID: "123456789"
              accessKeySecretRef:
                name: volcengine-cred
                key: access-key
              secretKeySecretRef:
                name: volcengine-cred
                key: secret-key
```

#### Option 2: use dynamic IRSA (Role TRN)

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-volcengine-irsa
spec:
  acme:
    email: you@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: letsencrypt-account-key
    solvers:
      - dns01:
          webhook:
            groupName: acme.yourcompany.com
            solverName: volcdns-resolver
            config:
              region: cn-beijing
              zoneID: "123456789"
              # Role TRN to assume
              roleTrn: trn:iam::123456789:role/cert-manager-role
```

### DNS behavior

In a typical ACME DNS01 validation flow:

- When `Present` is called:
  - Parse `ResolvedFQDN`
  - Determine the zone ID:
    - If `zoneID` is configured on the Issuer, validate and use that zone (recommended)
    - Otherwise, use Volcengine DNS APIs to find the best-matching zone for the FQDN
  - Escape the TXT record value (handle special characters such as double quotes)
  - Create a `TXT` record in that zone
- When `CleanUp` is called:
  - Again find the best-matching zone
  - Query TXT records under that hostname
  - Delete records whose value matches the current challenge key (supports automatic unescape matching)

All DNS calls are wrapped by [client.go](file:///home/maple/code/ai/trae/cert-manager-volcdns-webhook/volcengine/client.go) and use the `dns` client from `volcengine-go-sdk`.

### Notes

- This project only supports Volcengine Public DNS zones (public zone)
- Make sure the region, AK/SK, and role permissions match the actual DNS resources
