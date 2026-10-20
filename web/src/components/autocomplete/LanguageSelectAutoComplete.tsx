import { forwardRef } from 'react'
import type { AutoCompleteHandle, AutoCompleteProps } from './AutoComplete.tsx'
import CustomSelectAutoComplete from './CustomSelectAutoComplete.tsx'

type LanguageItem = {
  _id: string
  name: string
}

export const languageOptions: LanguageItem[] = [
  { _id: 'cpp', name: 'C++' },
  { _id: 'c', name: 'C' },
  { _id: 'python3', name: 'Python 3' },
  { _id: 'java', name: 'Java' },
  { _id: 'go', name: 'Go' },
  { _id: 'rust', name: 'Rust' },
  { _id: 'pascal', name: 'Pascal' },
  { _id: 'nodejs', name: 'JavaScript' },
]

export const LanguageSelectAutoComplete = forwardRef<AutoCompleteHandle<LanguageItem>, AutoCompleteProps<LanguageItem>>((props, ref) => (
  <CustomSelectAutoComplete
    ref={ref as never}
    data={languageOptions}
    {...{
      width: '100%',
      height: 'auto',
      listStyle: {},
      multi: true,
      selectedKeys: [],
      allowEmptyQuery: true,
      freeSolo: false,
      freeSoloConverter: (input: string) => input,
      ...props,
    }}
  />
))

LanguageSelectAutoComplete.displayName = 'LanguageSelectAutoComplete'
