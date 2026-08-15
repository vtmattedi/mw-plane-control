export function trimPastedValue(value: string) {
  return value.replace(/^[\s\u200B\uFEFF]+|[\s\u200B\uFEFF]+$/gu, '')
}

export function splitFormValues(value: string) {
  return value
    .split(/[\n,]/)
    .map(trimPastedValue)
    .filter(Boolean)
}
