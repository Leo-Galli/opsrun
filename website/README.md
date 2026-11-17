# OpsRun website

Marketing site for [OpsRun](https://github.com/Leo-Galli/opsrun). Astro static site
deployed to [opsrun.vercel.app](https://opsrun.vercel.app).

## Local development

```bash
cd website
npm install
npm run dev
```

```bash
npm run build
npm run preview
```

## Deploy on Vercel (you run this)

1. Import the `Leo-Galli/opsrun` GitHub repository in the Vercel dashboard.
2. Set **Root Directory** to `website`.
3. Framework Preset should detect Astro (`vercel.json` also sets `"framework": "astro"`).
4. Build command: `npm run build` (default). Output: `dist`.
5. Deploy. Attach the production domain `opsrun.vercel.app` (default for a project named `opsrun`, or add it under Domains).

CLI alternative from this folder:

```bash
npx vercel link
npx vercel --prod
```

Ensure the linked project Root Directory is `website`.

## Stack

- Astro
- Tailwind CSS v4 (`@tailwindcss/vite`)
- IBM Plex Sans + IBM Plex Mono
- English-only copy

## Routes

| Path | Page |
|---|---|
| `/` | Landing |
| `/docs` | Documentation hub |
| `/docs/getting-started` | Getting started |
| `/docs/cli` | CLI reference |
| `/docs/runbook-format` | Runbook grammar |
| `/docs/configuration` | opsrun.yaml |
| `/docs/architecture` | Architecture |
| `/docs/security` | Security model |
| `/docs/examples` | Bundled runbooks guide |
| `/runbooks` | Example runbook catalog |
| `/install` | Install + first commands |
| `/security` | Security overview |

Repository Markdown under `docs/` remains authoritative for deep diagrams.
