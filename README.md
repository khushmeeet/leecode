# LeeCode

A small single-user web app for organizing job-hunt prep: an application kanban, a tagged reading list, a LeetCode revision list, and behavioral (STAR) answers. Go + SQLite + server-rendered Bootstrap. No frameworks, no build step.

## Run locally

```sh
go run .
```

Opens on http://localhost:8080. The database is created automatically as `interview.db` in the working directory (override with `DB_PATH`).

## Deploy to Fly.io

One-time setup:

```sh
fly launch --no-deploy        # creates the app; keep the generated app name in fly.toml
fly volumes create leecode_data --size 1
```

Then deploy (and redeploy after changes) with:

```sh
fly deploy
```

The SQLite file lives on the volume at `/data/interview.db`, so it survives restarts and deploys.

## Backup

Visit `/export` (or the "Download backup" button on the dashboard) to download the raw SQLite file.
