// Ported from @hydrooj/ui-default components/problem/list.page.js (AGPL-3.0).
export function initHydroProblemList(root: ParentNode) {
  for (const anchor of root.querySelectorAll<HTMLAnchorElement>('.col--problem-name > a')) {
    anchor.target = '_blank'
  }
}
