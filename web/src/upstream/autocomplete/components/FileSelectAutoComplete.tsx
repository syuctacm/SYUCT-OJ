// @ts-nocheck
import React from 'react';
import CustomSelectAutoComplete from '../../../components/autocomplete/CustomSelectAutoComplete.tsx';

const FileSelectAutoComplete = React.forwardRef<any, any>((props, ref) => (
  <CustomSelectAutoComplete
    ref={ref}
    {...props}
    multi={!!props.multi}
  />
));

FileSelectAutoComplete.displayName = 'FileSelectAutoComplete';
export default FileSelectAutoComplete;
