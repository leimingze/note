---
name: offerlens-interview-data
description: Query complete real interview records for interview preparation, question-frequency analysis, project deep dives, coding-question summaries, and mock interviews. Use when the user asks what a company or role recently interviews for, wants evidence from real interview experiences, or wants resume/JD-aware preparation. Resume content always stays local.
---

# OfferLens Interview Data

Use the bundled client to retrieve real interview records. Keep resumes and all personal material local; never send them to OfferLens.

## Query

Run `python3 scripts/query_interviews.py` with the narrowest useful filters. Read [references/api.md](references/api.md) for all options.

- Default to the last 90 days for “recent” questions.
- For frequency summaries, use `--adaptive-recent`: it widens 90 → 180 → 365 days until the API reports at least 20 matching records. State the final time window and sample count.
- Prefer exact company and broad role keywords first. If no result is returned, relax one filter at a time.
- Preserve the returned interview text as evidence. Do not invent frequency, recency, interview rounds, or questions absent from the records.
- Never expose or infer internal source identifiers or original-source links.

## Combine evidence locally

When a resume is available, compare its local text with the returned records without uploading it. When a real JD is needed, either use an installed `offerlens-jobs` skill or ask the user to paste the JD.

Use interview records as evidence, JD as the target, and resume as local context. See [references/workflows.md](references/workflows.md) for concise patterns covering 八股、项目、算法题 and mock interviews.

## Access

The client uses the trial key `offerlens` when `OFFERLENS_API_KEY` is unset. Trial access allows 10 successful non-empty requests per IP and up to 10 records per request. Empty and failed requests do not consume quota. A purchased shared key removes the trial quota and allows up to 100 records per request; set it only through `OFFERLENS_API_KEY`.

If the API returns `trial_exhausted`, show the purchase message and URL exactly as returned. Do not retry automatically.
