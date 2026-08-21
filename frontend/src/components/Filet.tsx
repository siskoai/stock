// Filet de sécurité autour du contenu des pages.
//
// Sans lui, une exception levée pendant le rendu démonte tout l'arbre React :
// l'écran devient entièrement noir, sans message, sans bouton, sans indice.
// Un commerçant devant un écran noir n'a aucun moyen de s'en sortir, et rien à
// nous rapporter d'utile.
//
// Le filet retient l'erreur, garde la barre latérale utilisable, affiche ce qui
// s'est passé et propose de réessayer. Il ne répare rien : il transforme une
// panne muette en panne compréhensible et signalable.

import { Component, type ErrorInfo, type ReactNode } from 'react'

interface Props {
  /** Change de valeur à chaque navigation : remet le filet à zéro. */
  cle: string
  children: ReactNode
}

interface State {
  erreur: Error | null
  pile: string
}

export class Filet extends Component<Props, State> {
  state: State = { erreur: null, pile: '' }

  static getDerivedStateFromError(erreur: Error): Partial<State> {
    return { erreur }
  }

  componentDidCatch(erreur: Error, infos: ErrorInfo) {
    this.setState({ pile: infos.componentStack ?? '' })
    // Le journal du navigateur reste la trace la plus complète pour un
    // diagnostic ultérieur.
    console.error('écran en erreur', erreur, infos.componentStack)
  }

  componentDidUpdate(precedentes: Props) {
    if (precedentes.cle !== this.props.cle && this.state.erreur) {
      this.setState({ erreur: null, pile: '' })
    }
  }

  render() {
    const { erreur, pile } = this.state
    if (!erreur) return this.props.children

    return (
      <div className="filet">
        <div className="filet-carte">
          <h2 className="filet-titre">Cet écran n'a pas pu s'afficher</h2>
          <p className="filet-texte">
            Le reste de l'application continue de fonctionner : vos données ne
            sont pas touchées, et vous pouvez passer à un autre écran par le
            menu de gauche.
          </p>

          <div className="filet-detail">
            <div className="section-title">Détail technique</div>
            <p className="mono small" data-selectable>{erreur.message}</p>
            {pile && (
              <pre className="filet-pile" data-selectable>{pile.trim().split('\n').slice(0, 6).join('\n')}</pre>
            )}
          </div>

          <p className="filet-texte">
            Si le problème revient, signalez-le en recopiant le détail ci-dessus.
          </p>

          <div className="row" style={{ marginTop: 16 }}>
            <button
              className="btn btn-primary"
              onClick={() => this.setState({ erreur: null, pile: '' })}
            >Réessayer</button>
            <button className="btn" onClick={() => window.location.reload()}>
              Recharger l'application
            </button>
          </div>
        </div>
      </div>
    )
  }
}
