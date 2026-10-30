// Shared contest/homework sidebar routes used by problem and submit pages.

type RouteArgs = Record<string, unknown>

function queryString(query: Record<string, unknown>) {
  const params = new URLSearchParams()
  for (const [key, value] of Object.entries(query)) {
    if (value !== undefined && value !== null && value !== '') params.set(key, String(value))
  }
  return params.toString()
}

export function activityRoute(name: string, tid: string, args: RouteArgs = {}) {
  const id = encodeURIComponent(String(args.tid ?? tid))
  const query = (args.query ?? {}) as Record<string, unknown>
  switch (name) {
    case 'contest_detail': return `/contest/${id}`
    case 'contest_problemlist': return `/contest/${id}/problems`
    case 'contest_scoreboard': return `/contest/${id}/scoreboard`
    case 'contest_edit': return `/contest/${id}/edit`
    case 'contest_manage': return `/contest/${id}/management`
    case 'contest_clarification': return `/contest/${id}/clarification`
    case 'contest_user': return `/contest/${id}/user`
    case 'contest_balloon': return `/contest/${id}/balloon`
    case 'contest_print': return `/contest/${id}/print`
    case 'contest_team': return `/contest/${id}`
    case 'contest_code': return `/api/v1/contests/${id}/code`
    case 'homework_detail': return `/homework/${id}`
    case 'homework_edit': return `/homework/${id}/edit`
    case 'homework_files': return `/homework/${id}/file`
    case 'homework_scoreboard': return `/homework/${id}/scoreboard`
    case 'homework_code': return `/api/v1/homeworks/${id}/code`
    case 'record_main': {
      const qs = queryString(query)
      return `/record${qs ? `?${qs}` : ''}`
    }
    case 'discussion_create': {
      const type = encodeURIComponent(String(args.type ?? 'contest'))
      const target = encodeURIComponent(String(args.name ?? tid))
      return `/discuss/${type}/${target}/create`
    }
    case 'discussion_node': {
      const type = encodeURIComponent(String(args.type ?? 'contest'))
      const target = encodeURIComponent(String(args.name ?? tid))
      return `/discuss/${type}/${target}`
    }
    case 'wiki':
    case 'wiki_help':
      return '/discuss'
    default:
      return undefined
  }
}
