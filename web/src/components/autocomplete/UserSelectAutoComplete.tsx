import { forwardRef } from 'react'
import { getUsersByIDs, searchUsers, type UserChoice } from '../../api.ts'
import AutoComplete, { type AutoCompleteHandle, type AutoCompleteProps } from './AutoComplete.tsx'

export const UserSelectAutoComplete = forwardRef<AutoCompleteHandle<UserChoice>, AutoCompleteProps<UserChoice>>((props, ref) => (
  <AutoComplete<UserChoice>
    ref={ref as never}
    cacheKey="user"
    queryItems={searchUsers}
    fetchItems={(ids) => getUsersByIDs(ids.map(Number))}
    itemText={(user) => user.username + (user.displayName ? ` (${user.displayName})` : '')}
    itemKey={(user) => String(user.id)}
    renderItem={(user) => (
      <div className="media">
        <div className="media__left medium">
          {user.avatar ? <img className="small user-profile-avatar" alt="" src={user.avatar} width="30" height="30" /> : null}
        </div>
        <div className="media__body medium">
          <div className="user-select__uname">{user.username}{user.displayName ? ` (${user.displayName})` : ''}</div>
          <div className="user-select__uid">UID = {user.id}</div>
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

UserSelectAutoComplete.displayName = 'UserSelectAutoComplete'
