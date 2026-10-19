---
name: cycle
---

## Step: a - First
depends_on: [b]

```bash opsrun:run target=web
echo a
```

## Step: b - Second
depends_on: [a]

```bash opsrun:run target=web
echo b
```
