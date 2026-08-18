// Mention de paternité de l'éditeur.
//
// Le logo vient du binaire vérifié (voir internal/brand), pas d'un fichier
// d'interface : il ne peut pas être remplacé en éditant les ressources du
// frontend. Sa présence est une condition de la licence, article 3.

import { Session } from '../lib/api'
import { useAsync } from '../lib/useAsync'
import { useSession } from '../lib/session'

export function Attribution({ compact = false }: { compact?: boolean }) {
  const { state } = useSession()
  const brand = useAsync(() => Session.brand(), [])

  if (!brand.data) {
    // Repli textuel tant que l'image n'est pas là : la mention ne disparaît
    // jamais complètement, même le temps d'un chargement.
    return <div className="attribution-text muted">{state.notice}</div>
  }
  const b = brand.data

  return (
    <div className="attribution">
      <img src={b.logoDataUrl} alt={b.author} />
      <div className="attribution-text">
        <div className="attribution-author">{b.author}</div>
        <div className="muted">{compact ? b.notice : `${b.notice} (version ${state.appVersion})`}</div>
        {!b.intact && (
          <div style={{ color: 'var(--red)', marginTop: 3 }}>
            Identité visuelle modifiée, voir {b.licenseRef}.
          </div>
        )}
      </div>
    </div>
  )
}
