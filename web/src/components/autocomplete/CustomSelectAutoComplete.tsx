// @ts-nocheck -- copied from Hydro @hydrooj/components; upstream build uses looser TS settings.
import { defaults } from 'lodash';
import React from 'react';
import AutoComplete from './AutoComplete.tsx';

type DefaultProps = {
  _id: string;
  name: string;
} & string;

const CustomSelectAutoComplete = React.forwardRef<any, any>((props, ref) => {
  const def = defaults(props, {
    width: '100%',
    height: 'auto',
    listStyle: {},
    multi: false,
    selectedKeys: [],
    freeSolo: false,
    freeSoloConverter: (input) => input,
  });
  return <AutoComplete<DefaultProps>
    ref={ref as any}
    fetchItems={(keys) => def.data.filter((i) => (i._id ? keys.includes(i._id) : keys.includes(i)))}
    queryItems={(query) => def.data.filter((i) => (i.name || i).toString().toLowerCase().includes(query.toLowerCase()))}
    itemText={(item) => `${item.name || item}`}
    itemKey={(item) => `${item._id?.toString() || item.name || item}`}
    renderItem={(item) => `${item.name || item}`}
    allowEmptyQuery
    {...def}
  />;
});

CustomSelectAutoComplete.displayName = 'CustomSelectAutoComplete';

export default CustomSelectAutoComplete;
