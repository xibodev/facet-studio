import * as React from "react"

import { listModules, type ModuleView } from "@/api/modules"
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

/**
 * Module selector — the connector gesture.
 *
 * Selecting a module points the agent at it for the turn, the way choosing a
 * connector in a chat box does. That selection is what loads the module's
 * agent overlay and skills; until a module is selected it costs one line of
 * capability summary and nothing more.
 *
 * The selector shows what a module can do rather than only its name, because
 * the choice a person is making is "use Facet for this", not "configure a
 * setting".
 */

export const NO_MODULE = "__none__"

interface ModuleSelectorProps {
  value: string
  disabled?: boolean
  onValueChange: (moduleId: string) => void
}

export function ModuleSelector({
  value,
  disabled = false,
  onValueChange,
}: ModuleSelectorProps) {
  const [modules, setModules] = React.useState<ModuleView[]>([])

  React.useEffect(() => {
    let cancelled = false
    listModules()
      .then((list) => {
        if (!cancelled) setModules(list.filter((m) => !m.error))
      })
      // A failure here must not break the composer: the user can still chat,
      // they simply cannot scope the turn to a module.
      .catch(() => {})
    return () => {
      cancelled = true
    }
  }, [])

  if (modules.length === 0) return null

  const selected = modules.find((m) => m.module === value)

  return (
    <Select value={value || NO_MODULE} onValueChange={onValueChange} disabled={disabled}>
      <SelectTrigger
        size="sm"
        className="h-7 gap-1 border-dashed text-xs"
        aria-label="Module"
        title={
          selected
            ? `${selected.name}: ${selected.capabilities.length} capabilities available to the agent`
            : "Point the agent at a module"
        }
      >
        <SelectValue placeholder="Module" />
      </SelectTrigger>
      <SelectContent>
        <SelectGroup>
          <SelectItem value={NO_MODULE}>
            <span className="text-muted-foreground">No module</span>
          </SelectItem>
        </SelectGroup>
        <SelectSeparator />
        <SelectGroup>
          <SelectLabel>Modules</SelectLabel>
          {modules.map((m) => (
            <SelectItem key={m.module} value={m.module}>
              <div className="flex flex-col gap-0.5">
                <span>{m.name}</span>
                <span className="text-muted-foreground text-[11px]">
                  {m.capabilities.length} capabilities
                  {m.capabilities.some((c) => !c.cost_known) ? " · some may bill" : ""}
                </span>
              </div>
            </SelectItem>
          ))}
        </SelectGroup>
      </SelectContent>
    </Select>
  )
}
