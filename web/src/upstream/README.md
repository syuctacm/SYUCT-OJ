# Hydro upstream UI

The edit pages shown by `HydroTemplate` are rendered from the unmodified
Nunjucks sources in the pinned `@hydrooj/ui-default` package (AGPL-3.0):

- `templates/**/*.html` is loaded by `render.ts` via Vite `?raw` imports.
- `locales/zh.yaml` is the upstream Chinese translation table.
- `hydro-editor/mdeditor.ts` is generated from the package's
  `components/editor/mdeditor.ts` by `pnpm --dir web sync:hydro-ui`; only the
  import paths are rewritten for this repository.
- `render.ts` provides the small server-side context shims (`_`, `url`,
  `urlForFile`, `size`, `handler`, `form` macros) expected by those templates.
- `hydrate.tsx` mounts only the interactive islands (autocomplete, date/time
  picker, Markdown editor) into the already-rendered upstream markup.
- `problemconfig/` is the pinned upstream Monaco/problem-config editor ported
  from `components/problemconfig/`; only imports and the Monaco/Vite worker
  loader are adapted. Its schema and reducers remain upstream-derived.
- `monaco/textarea.ts` replaces upstream `textarea[data-json]` and
  `textarea[data-yaml]` autoload behavior with a small reusable Monaco mount.
  It is dynamically imported so Monaco stays out of the initial route bundle.
- The same mount also backs the submit-page code editor and switches Monaco
  language modes when the submission language changes.

Do not duplicate a migrated page's markup in TSX or add handwritten CSS to
approximate it. Page fixes belong in the adapter context or in an upstream
template version bump.
