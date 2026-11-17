---
status: accepted
---
# Ready means built PDFs, and Resume only is explicit

Written means the documents have text; Ready means the resume PDF exists and, unless the Application is Resume only, the cover letter PDF too. Both are worked out from files, so no screen can show a half-built packet as finished.

Resume only is stored explicitly in `meta.yml`, never inferred from an empty letter: an empty letter without the flag is a problem to fix, while Resume only is a choice already made. The build gate reads the same flag and refuses an empty letter unless it is set.
