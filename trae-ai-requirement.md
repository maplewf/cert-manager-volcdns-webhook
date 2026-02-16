Please create a project named cert-manager-volcdns-webhook that provides the following features:
- It implements the cert-manager ACME DNS01 webhook; see https://cert-manager.io/docs/configuration/acme/dns01/webhook/ for more details.
- Within the interfaces defined by the cert-manager webhook, it calls Volcengine DNS APIs to perform all resolver operations required by cert-manager ACME DNS01. The Volcengine DNS API implementation is at https://github.com/volcengine/volcengine-go-sdk/tree/master/service/dns.
- There is an existing project https://github.com/DEVmachine-fr/cert-manager-alidns-webhook that targets AliDNS and can be used as a reference, but we need to target Volcengine DNS (public zone) instead.
- Authentication must support both AK/SK and IRSA. For how AK/SK and IRSA are implemented on Volcengine, see https://github.com/volcengine/external-dns-volcengine-webhook.
- Remove all test code.
- If the conversation context length exceeds the limit, automatically compact it to reduce the context length.
