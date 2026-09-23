# Evidence-based workflows

## High-frequency 八股 or algorithm questions

1. Query the company, role keyword, and adaptive recent window.
2. Extract only questions explicitly present in returned text.
3. Merge wording variants, count records rather than repeated mentions in one record, and rank by record count.
4. Report the window, matching sample count, returned sample count, and any pagination limitation.
5. Separate observed questions from preparation suggestions.

## Project deep dive

Compare the local resume project with real project questions. Group likely follow-ups by architecture, trade-offs, performance, reliability, debugging, and personal contribution. Label generated follow-ups as predictions, not observed questions.

## Mock interview

Use the local resume and JD for context, then draw questions from returned evidence. A normal sequence can include self-introduction, fundamentals, project deep dive, and coding. Do not force that sequence when the user asks for only one section. Give the full evaluation after the requested session unless the user asks for per-question feedback.

## JD source

If `offerlens-jobs` is installed, use it to select a current official campus JD. Otherwise accept a JD pasted by the user. OfferLens Interview Data does not fetch or upload resumes.
