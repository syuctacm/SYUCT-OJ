import { forwardRef } from 'react'
import { getProblems, getProblemsByDocIDs, type ProblemSummary } from '../../api.ts'
import AutoComplete, { type AutoCompleteHandle, type AutoCompleteProps } from './AutoComplete.tsx'

export const ProblemSelectAutoComplete = forwardRef<AutoCompleteHandle<ProblemSummary>, AutoCompleteProps<ProblemSummary>>((props, ref) => (
  <AutoComplete<ProblemSummary>
    ref={ref as never}
    cacheKey="problem"
    queryItems={async (query) => (await getProblems(query, 1, query ? 'default' : 'recent')).items}
    fetchItems={(ids) => getProblemsByDocIDs(ids.map(Number))}
    itemText={(problem) => `${problem.docId} ${problem.title}`}
    itemKey={(problem) => String(problem.docId)}
    renderItem={(problem) => (
      <div className="media">
        <div className="media__body medium">
          <div className="problem-select__name">{problem.pid ? `${problem.pid} ` : ''}{problem.title}</div>
          <div className="problem-select__id">ID = {problem.docId}</div>
        </div>
      </div>
    )}
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

ProblemSelectAutoComplete.displayName = 'ProblemSelectAutoComplete'
