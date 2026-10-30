// Adapted from @hydrooj/ui-default components/autocomplete/FileSelectAutoComplete.tsx.
import { forwardRef } from 'react'

import AutoComplete, { type AutoCompleteHandle, type AutoCompleteProps } from './AutoComplete.tsx'

type FileChoice = { name: string; size?: number }

type Props = AutoCompleteProps<FileChoice> & {
  data: FileChoice[]
}

export const FileSelectAutoComplete = forwardRef<AutoCompleteHandle<FileChoice>, Props>((props, ref) => {
  const { data, ...rest } = props
  return (
    <AutoComplete<FileChoice>
      ref={ref}
      queryItems={(query) => data.filter((item) => item.name.toLowerCase().includes(query.toLowerCase()))}
      fetchItems={(names) => data.filter((item) => names.includes(item.name))}
      itemText={(item) => item.name}
      itemKey={(item) => item.name}
      renderItem={(item) => <div className="file-select__name">{item.name}</div>}
      {...{
        width: '100%',
        height: 'auto',
        listStyle: {},
        multi: false,
        selectedKeys: [],
        allowEmptyQuery: true,
        freeSolo: false,
        freeSoloConverter: (input: string) => input,
        ...rest,
      }}
    />
  )
})

FileSelectAutoComplete.displayName = 'FileSelectAutoComplete'
