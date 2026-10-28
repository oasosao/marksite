type Handler = () => void

const handlers = new Map<string, Set<Handler>>()

export function on(event: string, handler: Handler) {
  if (!handlers.has(event)) {
    handlers.set(event, new Set())
  }
  handlers.get(event)!.add(handler)
}

export function off(event: string, handler: Handler) {
  handlers.get(event)?.delete(handler)
}

export function emit(event: string) {
  handlers.get(event)?.forEach((h) => h())
}
