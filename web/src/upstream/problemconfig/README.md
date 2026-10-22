# Problem config editor

Ported from `@hydrooj/ui-default@4.58.5` `components/problemconfig/` and
`components/monaco/schema/problemconfig.ts` (AGPL-3.0).

Changes from upstream:

- `vj/utils` and `vj/components/dialog` are replaced by `compat.ts`.
- The Monaco loader is reduced to a local React component backed by
  `monaco-editor` and `monaco-yaml`; Vite workers are registered in
  `ProblemConfigEditor.tsx`.
- Local autocomplete components replace the workspace-only
  `@hydrooj/components` imports.
- `@hydrooj/utils` and `@hydrooj/common` helpers are copied locally to keep the
  browser bundle independent of Node-oriented workspace packages.
- `CONFIG_TESTDATA_REPLACE` lets the page refresh the sidebar after uploads
  without discarding unsaved editor state.

The config still saves through the existing Go `PATCH /api/v1/problems/:pid`
API.
