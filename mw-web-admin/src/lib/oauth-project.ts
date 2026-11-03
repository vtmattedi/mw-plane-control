import { queryOptions } from '@tanstack/react-query'
import { useOutletContext } from 'react-router-dom'
import { controlRequest } from './control-api'
import type { GoogleOAuthConfig } from './oauth-provider-config'

export function useOAuthProject() {
  return useOutletContext<{ project: Record<string, unknown> }>()
}

export function googleProviderQuery(projectID: string) {
  return queryOptions({
    queryKey: ['oauth-google-provider', projectID],
    queryFn: () =>
      controlRequest<GoogleOAuthConfig>(
        `/oauth/projects/${encodeURIComponent(projectID)}/oauth/google`,
      ),
    staleTime: 30_000,
    retry: 1,
    enabled: Boolean(projectID),
  })
}
