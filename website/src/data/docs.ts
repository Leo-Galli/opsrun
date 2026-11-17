export type DocLink = {
  href: string
  label: string
}

export const docsNav: DocLink[] = [
  { href: '/docs', label: 'Overview' },
  { href: '/docs/getting-started', label: 'Getting started' },
  { href: '/docs/cli', label: 'CLI reference' },
  { href: '/docs/runbook-format', label: 'Runbook format' },
  { href: '/docs/configuration', label: 'Configuration' },
  { href: '/docs/architecture', label: 'Architecture' },
  { href: '/docs/security', label: 'Security' },
  { href: '/docs/examples', label: 'Examples' },
]
