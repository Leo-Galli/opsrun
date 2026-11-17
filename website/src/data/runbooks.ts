export type Runbook = {
  file: string
  name: string
  severity: 'critical' | 'high' | 'medium'
  service: string
  summary: string
  try: string
}

export const githubRepo = 'https://github.com/Leo-Galli/opsrun'
export const runbooksPath = `${githubRepo}/tree/main/examples/runbooks`

export const runbooks: Runbook[] = [
  {
    file: 'full-disk.md',
    name: 'full-disk-remediation',
    severity: 'high',
    service: 'web-frontend',
    summary: 'Check disk usage, clear a known-safe cache directory, restart the service with rollback.',
    try: 'opsrun validate examples/runbooks/full-disk.md',
  },
  {
    file: 'service-restart.md',
    name: 'service-restart-with-healthcheck',
    severity: 'medium',
    service: 'payments-api',
    summary: 'Confirm the service is unhealthy, restart it, verify healthz, roll back on failure.',
    try: 'opsrun plan examples/runbooks/service-restart.md',
  },
  {
    file: 'db-failover.md',
    name: 'database-failover',
    severity: 'critical',
    service: 'orders-db',
    summary: 'Lag-checked replica promote and application repoint with approval mode all.',
    try: 'opsrun validate examples/runbooks/db-failover.md',
  },
  {
    file: 'cert-renewal.md',
    name: 'tls-certificate-renewal',
    severity: 'high',
    service: 'edge-proxy',
    summary: 'Expiry diagnostic, renew TLS, backup-and-restore rollback path.',
    try: 'opsrun plan examples/runbooks/cert-renewal.md --var domain=example.com',
  },
  {
    file: 'oom-mitigation.md',
    name: 'oom-mitigation',
    severity: 'critical',
    service: 'api-worker',
    summary: 'Confirm memory pressure, then restart the worker safely under approval.',
    try: 'opsrun validate examples/runbooks/oom-mitigation.md',
  },
  {
    file: 'log-cleanup.md',
    name: 'log-disk-cleanup',
    severity: 'medium',
    service: 'api-gateway',
    summary: 'Compress and delete old logs under a retention policy to relieve disk pressure.',
    try: 'opsrun plan examples/runbooks/log-cleanup.md',
  },
  {
    file: 'lb-node-drain.md',
    name: 'lb-node-drain-for-maintenance',
    severity: 'medium',
    service: 'web-frontend',
    summary: 'Drain a host via LB API with a secret token, maintain, then restore rotation.',
    try: 'opsrun plan examples/runbooks/lb-node-drain.md --var lb_api_token=dummy',
  },
  {
    file: 'rolling-deploy.md',
    name: 'rolling-application-deploy',
    severity: 'medium',
    service: 'web-frontend',
    summary: 'Deploy a release tag with rolling or canary host strategy and health checks.',
    try: 'opsrun plan examples/runbooks/rolling-deploy.md --var release_tag=v1.4.2',
  },
  {
    file: 'runaway-process.md',
    name: 'runaway-process-mitigation',
    severity: 'high',
    service: 'batch-worker',
    summary: 'Identify the runaway process, stop gracefully, then force-kill if needed.',
    try: 'opsrun plan examples/runbooks/runaway-process.md --var process_name=worker',
  },
]

export const severityLabel: Record<Runbook['severity'], string> = {
  critical: 'critical',
  high: 'high',
  medium: 'medium',
}
