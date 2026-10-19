---
name: tls-certificate-renewal
version: "1.0"
service: edge-proxy
severity: high
author: sre-team
targets:
  - group: edge
vars:
  domain:
    type: string
    required: true
  min_seconds_remaining:
    type: int
    default: 1209600 # 14 days; openssl checkend takes seconds, not days
approval:
  mode: per_step
  timeout: 10m
---

## Step: check-expiry - Check certificate expiry
read_only: true

Read-only diagnostic: reports how many days remain before the certificate for
`{{ .Vars.domain }}` expires, so the operator can confirm the incident before renewing
anything.

```bash opsrun:check expect_exit=0
echo | openssl s_client -servername {{ .Vars.domain }} -connect {{ .Vars.domain }}:443 2>/dev/null | openssl x509 -noout -checkend {{ .Vars.min_seconds_remaining }}
```

## Step: backup-current-cert - Back up the current certificate and key
depends_on: [check-expiry]

Copies the current certificate and key aside before renewal, so a failed renewal can be
rolled back to a known-good state immediately.

```bash opsrun:run target=edge timeout=30s
cp -a /etc/letsencrypt/live/{{ .Vars.domain }} /etc/letsencrypt/live/{{ .Vars.domain }}.bak
```

```bash opsrun:rollback
rm -rf /etc/letsencrypt/live/{{ .Vars.domain }}.bak
```

## Step: renew-certificate - Renew the certificate
depends_on: [backup-current-cert]
confirm: true
retries: 2

Requests a renewed certificate from the ACME provider.

```bash opsrun:run target=edge timeout=2m
certbot renew --cert-name {{ .Vars.domain }} --non-interactive
```

```bash opsrun:check expect_exit=0
openssl x509 -in /etc/letsencrypt/live/{{ .Vars.domain }}/cert.pem -noout -checkend 0
```

```bash opsrun:rollback
rm -rf /etc/letsencrypt/live/{{ .Vars.domain }} && mv /etc/letsencrypt/live/{{ .Vars.domain }}.bak /etc/letsencrypt/live/{{ .Vars.domain }}
```

## Step: reload-proxy - Reload the edge proxy
depends_on: [renew-certificate]

Reloads nginx so it picks up the renewed certificate without dropping connections.

```bash opsrun:run target=edge timeout=30s
nginx -t && systemctl reload nginx
```

```bash opsrun:check expect_exit=0
systemctl is-active nginx
```

```bash opsrun:rollback
systemctl reload nginx
```
