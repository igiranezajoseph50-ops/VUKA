// CorridorScene — two-node visual of the only live route (Rwanda -> Kenya).
// Shared by the landing page and any marketing surface.
export default function CorridorScene() {
  const nodes = [
    ['Rwanda', 32, 62],
    ['Kenya', 72, 42],
  ] as const

  return (
    <svg className="my-6 h-64 w-full rounded-3xl bg-slate-50" viewBox="0 0 100 100" aria-label="Rwanda to Kenya payment corridor">
      <path d="M32 62 C45 45, 58 40, 72 42" fill="none" stroke="#16a34a" strokeDasharray="4 4" strokeWidth="1.6" />
      <path d="M22 72 C32 55, 40 42, 50 34 C62 25, 78 31, 82 46 C87 63, 76 82, 60 86 C45 91, 28 86, 22 72Z" fill="#ecfdf5" opacity=".95" />
      {nodes.map(([label, x, y]) => (
        <g key={label}>
          <circle cx={x} cy={y} r="8" fill="#bbf7d0" />
          <circle cx={x} cy={y} r="3.2" fill="#16a34a" />
          <text x={x} y={y + 13} textAnchor="middle" className="fill-slate-500 text-[5px]">{label}</text>
        </g>
      ))}
    </svg>
  )
}