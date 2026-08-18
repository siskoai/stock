// Jeu d'icônes minimal, dessiné en traits de 1,6 px pour rester lisible à
// 17 px. Aucune dépendance : quelques chemins suffisent.

interface Props { size?: number }

const base = (size: number) => ({
  width: size, height: size, viewBox: '0 0 24 24',
  fill: 'none', stroke: 'currentColor',
  strokeWidth: 1.6, strokeLinecap: 'round' as const, strokeLinejoin: 'round' as const,
})

export const IconDashboard = ({ size = 17 }: Props) => (
  <svg {...base(size)}><rect x="3" y="3" width="7" height="8" rx="1.5" /><rect x="14" y="3" width="7" height="5" rx="1.5" /><rect x="14" y="11" width="7" height="10" rx="1.5" /><rect x="3" y="14" width="7" height="7" rx="1.5" /></svg>
)
export const IconBox = ({ size = 17 }: Props) => (
  <svg {...base(size)}><path d="M21 8v8l-9 5-9-5V8l9-5 9 5Z" /><path d="M3 8l9 5 9-5M12 13v8" /></svg>
)
export const IconLayers = ({ size = 17 }: Props) => (
  <svg {...base(size)}><path d="M12 3 3 8l9 5 9-5-9-5Z" /><path d="m3 13 9 5 9-5" /></svg>
)
export const IconCart = ({ size = 17 }: Props) => (
  <svg {...base(size)}><circle cx="9" cy="20" r="1.4" /><circle cx="18" cy="20" r="1.4" /><path d="M2 3h2.5l2.2 11.2a1.5 1.5 0 0 0 1.5 1.2h8.4a1.5 1.5 0 0 0 1.5-1.2L20 7H6" /></svg>
)
export const IconTruck = ({ size = 17 }: Props) => (
  <svg {...base(size)}><path d="M2 6h11v10H2zM13 10h4.5l3.5 3.5V16h-8z" /><circle cx="7" cy="18" r="1.6" /><circle cx="17" cy="18" r="1.6" /></svg>
)
export const IconUsers = ({ size = 17 }: Props) => (
  <svg {...base(size)}><circle cx="9" cy="8" r="3.2" /><path d="M2.5 20a6.5 6.5 0 0 1 13 0" /><path d="M16 5.3a3.2 3.2 0 0 1 0 5.4M17.5 14.2A6.5 6.5 0 0 1 21.5 20" /></svg>
)
export const IconWallet = ({ size = 17 }: Props) => (
  <svg {...base(size)}><rect x="3" y="6" width="18" height="13" rx="2" /><path d="M3 10h18M16.5 14.5h.01" /></svg>
)
export const IconChart = ({ size = 17 }: Props) => (
  <svg {...base(size)}><path d="M3 3v18h18" /><path d="m7 14 3.5-4 3 2.5L20 6" /></svg>
)
export const IconSettings = ({ size = 17 }: Props) => (
  <svg {...base(size)}><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.6 1.6 0 0 0 .3 1.8l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.6 1.6 0 0 0-2.7 1.1V21a2 2 0 1 1-4 0v-.1A1.6 1.6 0 0 0 7.9 19.4l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1A1.6 1.6 0 0 0 3 15.9a2 2 0 1 1 0-4 1.6 1.6 0 0 0 1.1-2.7l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1A1.6 1.6 0 0 0 9.6 4.6H10a2 2 0 1 1 4 0 1.6 1.6 0 0 0 2.7 1.1l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.6 1.6 0 0 0 1.1 2.7 2 2 0 1 1 0 4 1.6 1.6 0 0 0-1.2.8Z" /></svg>
)
export const IconShield = ({ size = 17 }: Props) => (
  <svg {...base(size)}><path d="M12 3 4.5 6v6c0 4.5 3.1 8.2 7.5 9 4.4-.8 7.5-4.5 7.5-9V6L12 3Z" /><path d="m9 12 2 2 4-4" /></svg>
)
export const IconSearch = ({ size = 15 }: Props) => (
  <svg {...base(size)}><circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5" /></svg>
)
export const IconPlus = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M12 5v14M5 12h14" /></svg>
)
export const IconTrash = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M4 7h16M9 7V5a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2M6 7l1 13a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1l1-13" /></svg>
)
export const IconEdit = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L8 18l-4 1 1-4 11.5-11.5Z" /></svg>
)
export const IconPrint = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M7 8V3h10v5M7 18H5a2 2 0 0 1-2-2v-4a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2v4a2 2 0 0 1-2 2h-2" /><rect x="7" y="14" width="10" height="7" rx="1" /></svg>
)
export const IconDownload = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M12 3v12m0 0 4-4m-4 4-4-4M4 19h16" /></svg>
)
export const IconClose = ({ size = 16 }: Props) => (
  <svg {...base(size)}><path d="M6 6l12 12M18 6 6 18" /></svg>
)
export const IconArrowUp = ({ size = 12 }: Props) => (
  <svg {...base(size)}><path d="M12 19V5m0 0-6 6m6-6 6 6" /></svg>
)
export const IconArrowDown = ({ size = 12 }: Props) => (
  <svg {...base(size)}><path d="M12 5v14m0 0 6-6m-6 6-6-6" /></svg>
)
export const IconAlert = ({ size = 16 }: Props) => (
  <svg {...base(size)}><path d="M12 3 2 20h20L12 3Z" /><path d="M12 10v4M12 17.5h.01" /></svg>
)
export const IconCheck = ({ size = 16 }: Props) => (
  <svg {...base(size)}><path d="m5 12.5 4.5 4.5L19 7" /></svg>
)
export const IconLogout = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M15 4h3a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-3M10 16l-4-4 4-4M6 12h12" /></svg>
)
export const IconFolder = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M3 7a2 2 0 0 1 2-2h4l2 2.5h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V7Z" /></svg>
)
export const IconRefresh = ({ size = 15 }: Props) => (
  <svg {...base(size)}><path d="M20 11A8 8 0 0 0 6.3 6.3L4 8.5M4 5v3.5h3.5M4 13a8 8 0 0 0 13.7 4.7L20 15.5M20 19v-3.5h-3.5" /></svg>
)
