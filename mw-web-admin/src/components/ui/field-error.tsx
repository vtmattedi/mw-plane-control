export function FieldError({ id, message }: { id: string; message?: string }) {
  if (!message) return null

  return (
    <small className="field-error" id={id}>
      {message}
    </small>
  )
}
