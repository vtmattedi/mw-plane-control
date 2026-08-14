import { Switch } from '@/components/ui/switch'
import styles from './entity-active-toggle.module.scss'

export function EntityActiveToggle({
  active,
  disabled = false,
  entityName,
  onChange,
}: {
  active: boolean
  disabled?: boolean
  entityName: string
  onChange: (active: boolean) => void
}) {
  return (
    <div className={styles.toggle}>
      <Switch
        aria-label={`${active ? 'Desativar' : 'Ativar'} ${entityName}`}
        checked={active}
        disabled={disabled}
        onCheckedChange={onChange}
      />
      <span>{active ? 'Ativo' : 'Inativo'}</span>
    </div>
  )
}
