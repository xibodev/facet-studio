import { useCallback, useEffect, useState } from "react"

import {
  type ModelRoute,
  type ProviderTarget,
  listModelRoutes,
  listProviderTargets,
} from "@/api/provider-instances"

export function useChatSelections(isConnected: boolean) {
  const [targets, setTargets] = useState<ProviderTarget[]>([])
  const [routes, setRoutes] = useState<ModelRoute[]>([])

  const refresh = useCallback(async () => {
    try {
      const [targetData, routeData] = await Promise.all([
        listProviderTargets(),
        listModelRoutes(),
      ])
      setTargets(targetData.targets || [])
      setRoutes(routeData.routes || [])
    } catch {
      setTargets([])
      setRoutes([])
    }
  }, [])

  useEffect(() => {
    if (isConnected) void refresh()
  }, [isConnected, refresh])

  return { targets, routes, refresh }
}
