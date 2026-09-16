import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import type { ModelRoute, ProviderTarget } from "@/api/provider-instances"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectLabel,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"

import { chatSelectionValues } from "./model-selector.utils"

const CONFIGURED_DEFAULT = "__configured_default__"

interface ModelSelectorProps {
  selection: string
  targets: ProviderTarget[]
  routes: ModelRoute[]
  disabled?: boolean
  onValueChange: (selection: string) => void
}

export function ModelSelector({
  selection,
  targets,
  routes,
  disabled = false,
  onValueChange,
}: ModelSelectorProps) {
  const { t } = useTranslation()
  const values = chatSelectionValues(targets, routes)

  const targetsByInstance = useMemo(() => {
    const map = new Map<string, ProviderTarget[]>()
    for (const target of targets) {
      const list = map.get(target.instance_id) || []
      list.push(target)
      map.set(target.instance_id, list)
    }
    return Array.from(map.entries())
  }, [targets])

  return (
    <Select
      value={selection || CONFIGURED_DEFAULT}
      onValueChange={(value) =>
        onValueChange(value === CONFIGURED_DEFAULT ? "" : value)
      }
      disabled={disabled}
    >
      <SelectTrigger
        size="sm"
        className="h-8 max-w-[240px] min-w-[130px] bg-transparent shadow-none"
      >
        <SelectValue />
      </SelectTrigger>
      <SelectContent position="popper" align="start" className="max-h-80">
        <SelectItem value={CONFIGURED_DEFAULT}>
          {t("chat.selection.configuredDefault")}
        </SelectItem>
        {routes.length > 0 && <SelectSeparator />}
        {routes.length > 0 && (
          <SelectGroup>
            <SelectLabel>{t("chat.selection.routes")}</SelectLabel>
            {values.routes.map((route) => (
              <SelectItem key={route} value={route}>
                {route}
              </SelectItem>
            ))}
          </SelectGroup>
        )}
        {targetsByInstance.map(([instanceId, instanceTargets]) => (
          <div key={instanceId}>
            <SelectSeparator />
            <SelectGroup>
              <SelectLabel className="font-semibold text-xs tracking-wider text-muted-foreground uppercase">
                {instanceId} ({instanceTargets.length})
              </SelectLabel>
              {instanceTargets.map((item) => (
                <SelectItem key={item.target} value={item.target}>
                  {item.model_id}
                </SelectItem>
              ))}
            </SelectGroup>
          </div>
        ))}
      </SelectContent>
    </Select>
  )
}
