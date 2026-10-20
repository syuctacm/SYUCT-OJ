import { Navigate, useLocation } from 'react-router-dom'
import { legacyDomainTarget } from '../legacy-url.ts'

export function LegacyDomainRedirect() {
  const location = useLocation()
  return <Navigate to={legacyDomainTarget(location.pathname, location.search, location.hash)} replace />
}
