import type { HTMLAttributes } from 'react'

export function Icon({ name, className = '', ...rest }: { name: string } & HTMLAttributes<HTMLSpanElement>) {
  return <span {...rest} className={`icon icon-${name}${className ? ` ${className}` : ''}`} />
}
