# Agent CRM (India, life insurance)

React + TypeScript · Go · PostgreSQL · REST `/api/v1` · Docker Compose

## Run
    cp .env.example .env     # optional; defaults work for local use
    docker compose up --build
- Web: http://localhost:3000   API: http://localhost:8080/api/v1   Health: /healthz

## API (`/api/v1`)
| Method | Path | Notes |
|---|---|---|
| GET/POST | /leads | GET supports `?q=` and `?stage=` |
| PUT/DELETE | /leads/{id} | |
| GET/POST | /policies | |
| DELETE | /policies/{id} | |
| GET/POST | /followups | |
| PATCH | /followups/{id}/done | |
| GET | /dashboard | pipeline counts, renewals due in 30 days, premium book |

Phone is validated as a 10-digit Indian mobile (starts 6-9). Money is whole rupees (bigint), shown in en-IN format.

## Not included yet
Login/multi-agent isolation, IRDAI/consent (DPDP Act) tracking, reminders via SMS/WhatsApp, migrations tool, tests.
