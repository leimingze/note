# API and client reference

Run from the installed skill directory:

```bash
python3 scripts/query_interviews.py --company 字节跳动 --keyword "AI Agent" --days 90 --format markdown
```

Filters: `--company`, `--post`, `--role-group`, `--role-family`, `--keyword`, `--date-from`, and `--date-to`. Pagination uses `--limit` and `--offset`.

Common role families are `backend_software`, `ai_algorithm`, `ai_application`, `ai_infra`, `frontend_fullstack`, `data_engineering`, `client`, `testing_quality`, and `sre_devops`. Start with company plus keyword when unsure, then reuse the `role_family` visible in matching records.

Use `--adaptive-recent` for evidence summaries. It may make up to three successful requests, each counting against trial quota. Use `--days 90` for a single request.

Complete records can be long. Use `--limit 3` and paginate with `--offset 3`, `6`, and so on when the Agent context is tight; do not replace or truncate the original text in the API response.

The API returns:

- `data`: company, post, role classification, title, complete interview text, and edit time.
- `total`: matching record count before pagination.
- `access`: `trial` or `paid`.
- `trial_remaining`: remaining successful trial requests, or `null` for paid access.

Environment variables:

- `OFFERLENS_API_KEY`: purchased key. If absent, the client uses `offerlens` for trial access.
- `OFFERLENS_API_BASE`: API origin override, mainly for local development.

Do not put a purchased key in prompts, command arguments, source files, or chat output.
