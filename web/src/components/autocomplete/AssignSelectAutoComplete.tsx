import { forwardRef } from 'react'
import {
  getGroupsByNames,
  getUsersByIDs,
  searchGroups,
  searchUsers,
  type GroupChoice,
  type UserChoice,
} from '../../api.ts'
import AutoComplete, { type AutoCompleteHandle, type AutoCompleteProps } from './AutoComplete.tsx'

export type AssignItem = {
  type: 'user' | 'group'
  key: string
  name: string
  displayName?: string
  avatar?: string
  uids?: number[]
  invalid?: boolean
}

function userItem(user: UserChoice): AssignItem {
  return { type: 'user', key: String(user.id), name: user.username, displayName: user.displayName, avatar: user.avatar }
}

function groupItem(group: GroupChoice): AssignItem {
  return { type: 'group', key: group.name, name: group.name, uids: group.uids }
}

export const AssignSelectAutoComplete = forwardRef<AutoCompleteHandle<AssignItem>, AutoCompleteProps<AssignItem>>((props, ref) => (
  <AutoComplete<AssignItem>
    ref={ref as never}
    cacheKey="assign"
    queryItems={async (query) => {
      const [users, groups] = await Promise.all([searchUsers(query), searchGroups(query)])
      return [...groups.map(groupItem), ...users.map(userItem)]
    }}
    fetchItems={async (keys) => {
      const numeric = keys.filter((key) => /^-?\d+$/.test(key)).map(Number)
      const names = keys.filter((key) => !/^-?\d+$/.test(key))
      const [users, groups] = await Promise.all([getUsersByIDs(numeric), getGroupsByNames(names)])
      const groupMap = new Map(groups.map((group) => [group.name, group]))
      const groupItems = names.map((name) => {
        const group = groupMap.get(name)
        return group ? groupItem(group) : { type: 'group' as const, key: name, name, invalid: true }
      })
      return [...groupItems, ...users.map(userItem)]
    }}
    itemText={(item) => {
      if (item.type === 'group') return item.invalid ? `${item.name} (invalid)` : `${item.name} (${item.uids?.length || 0} users)`
      return item.name + (item.displayName ? ` (${item.displayName})` : '')
    }}
    itemKey={(item) => item.key}
    renderItem={(item) => (
      <div className="media">
        {item.type === 'user' && item.avatar ? (
          <div className="media__left medium">
            <img className="small user-profile-avatar" alt="" src={item.avatar} width="30" height="30" />
          </div>
        ) : null}
        <div className="media__body medium">
          <div className="assign-select__name">
            {item.name}{item.type === 'user' && item.displayName ? ` (${item.displayName})` : ''}
          </div>
          <div className="assign-select__desc">
            {item.type === 'group' ? `Group • ${item.uids?.length || 0} users` : `User • UID = ${item.key}`}
          </div>
        </div>
      </div>
    )}
    {...{
      width: '100%',
      height: 'auto',
      listStyle: {},
      multi: true,
      selectedKeys: [],
      allowEmptyQuery: false,
      freeSolo: false,
      freeSoloConverter: (input: string) => input,
      ...props,
    }}
  />
))

AssignSelectAutoComplete.displayName = 'AssignSelectAutoComplete'
