// Logo du commerçant, tel qu'il l'a téléversé dans les paramètres.
//
// L'image n'accompagne pas l'état de session : elle pèse jusqu'à 380 Ko, et cet
// état est relu toutes les minutes. Elle est donc demandée séparément, et
// redemandée seulement quand son empreinte change, c'est-à-dire quand le
// commerçant remplace son logo.

import { useEffect, useState } from 'react'
import { Session } from '../lib/api'
import { useSession } from '../lib/session'

export function useCompanyLogo(): string {
  const { state } = useSession()
  const [logo, setLogo] = useState('')
  const empreinte = state.companyLogoFingerprint

  useEffect(() => {
    if (!state.authenticated || empreinte === '') {
      setLogo('')
      return
    }
    let annule = false
    Session.companyLogo()
      .then((data) => { if (!annule) setLogo(data) })
      .catch(() => { if (!annule) setLogo('') })
    return () => { annule = true }
  }, [empreinte, state.authenticated])

  return logo
}

/** CompanyLogo affiche le logo, ou rien du tout si aucun n'est configuré. */
export function CompanyLogo({ className, alt }: { className?: string; alt?: string }) {
  const { state } = useSession()
  const logo = useCompanyLogo()
  if (!logo) return null
  return <img src={logo} alt={alt ?? state.companyName} className={className} />
}
