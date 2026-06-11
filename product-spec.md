# Interview Prep — Product Spec

## What it is
A small, single-user web app to organize my job-hunt prep. Temporary, low-traffic, single user (me). Keep everything simple. Do not over-engineer.

## Tech stack (fixed — do not substitute)
- **Backend:** Go, standard library `net/http` + `html/template`. No web framework.
- **DB:** SQLite via `modernc.org/sqlite` (pure Go, no CGO). WAL mode on.
- **Frontend:** Server-rendered HTML + Bootstrap 5 (via CDN). No SPA, no npm build step, no JS framework.
- **Interactions:** Plain HTML forms (POST → redirect-on-success). Add minimal vanilla JS only where it clearly helps.
- **Deploy:** Fly.io, single machine, DB file on a mounted volume at `/data/interview.db`.

## Architecture (keep flat)
```
main.go            // routes + all handlers
db.go              // sqlite open, schema migrate-on-start, queries
templates/*.html   // layout + one template per feature
go.mod
Dockerfile
fly.toml           // volume mounted at /data
```
Schema is created on startup if absent. No migration tooling.

## Features
Four sections, reachable from a left sidebar. Each is simple CRUD (list + add form + delete) unless noted.

### 1. Applications (kanban)
- Three fixed columns: **Applied**, **Rejected**, **Accepted**.
- Card fields: company (required), role, posting URL, notes, created date.
- Move a card between columns via a small status dropdown + button on the card (no drag-and-drop).
- Add via a form; delete via a button.

### 2. Links
- A tagged, read-anywhere reading list (I'll read these from any computer).
- Fields: title (required), URL (required), tag, read/unread flag, created date.
- List view with filter-by-tag and a toggle to mark read/unread.

### 3. Algorithms / LeetCode
- A revision list of problems.
- Fields: title (required), URL, difficulty (Easy/Medium/Hard), pattern (free text, e.g. two-pointers, DP, graph), notes, created date.
- List view, filterable by difficulty and pattern.

### 4. Behavioral
- My STAR stories and HR-question answers (e.g. "describe a hard problem you solved", "why this company", "biggest weakness").
- Fields: question (required), answer (long text), category (e.g. conflict, leadership, why-company), created date.
- List view, filterable by category. Answer field should render readable long text.

## Extras (only if trivial)
- `GET /export` — stream the raw `.db` file so I can back it up to my laptop.
- A simple dashboard landing page with counts (applications per status, unread links, total problems, behavioral answers).

## Design / aesthetic
Polished and calm — this is something I'll look at daily, so it should feel intentional, not a Bootstrap default dump.
- Clean, generous whitespace;
- One restrained accent color; neutral grays elsewhere. Subtle borders/shadows, rounded cards.
- A nicer font than default (Google Fonts: e.g. **Inter** for UI, optional mono like **JetBrains Mono** for code/URLs).
- Status columns and difficulty levels use small, muted color-coded badges (not loud).
- Consistent card styling across all four features. Mobile-friendly (I'll read links on the go).

## Navigation
Top navbar (Bootstrap 5 `navbar`), not a sidebar — four flat peer views need no hierarchy, and the navbar collapses to a hamburger on mobile for free.
- Left: brand/title "LeeCode", linking to the dashboard.
- Right: four links — Applications, Links, Algorithms, Behavioral.
- Highlight the current page with Bootstrap's `.active` class: pass the current section name into the layout template and compare per link.
- Use `navbar-expand-md` so it collapses on small screens (I'll use this on mobile).

## Out of scope (do not build)
Auth/login, multi-user,spaced-repetition scheduling, htmx, real-time anything, tests beyond a smoke check, Postgres, ORMs.

## Deliverables
Complete runnable repo: all Go files, templates, `Dockerfile`, `fly.toml` (with volume config), and a short README covering local run (`go run .`) and Fly deploy (volume creation + deploy).
