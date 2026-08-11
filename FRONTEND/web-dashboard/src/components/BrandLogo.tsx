// BrandLogo — renders the VUKA logo mark.
//
// The source artwork is navy + emerald on a transparent canvas. Because the
// dashboard shell is a dark navy surface and document views are white, the
// component always sits the mark inside an opaque white badge so it stays
// legible on either background.
import vukaLogo from '../assets/vuka-logo.png'

interface BrandLogoProps {
  /** Pixel width of the badge (default 40 for nav/10h). */
  size?: number
  /** Optional wordmark shown beside the mark. */
  wordmark?: string
  /** Optional sub-label under the wordmark (uppercased micro-copy). */
  sublabel?: string
  className?: string
}

export default function BrandLogo({ size = 40, wordmark, sublabel, className }: BrandLogoProps) {
  return (
    <div className={`flex items-center gap-2 ${className ?? ''}`}>
      <span
        className="grid shrink-0 place-items-center rounded-full bg-white shadow-md shadow-black/20 ring-1 ring-slate-200"
        style={{ width: size, height: size }}
        aria-hidden="true"
      >
        <img src={vukaLogo} alt="" width={size * 0.82} height={size * 0.82} className="object-contain" />
      </span>
      {wordmark && (
        <span className="leading-none">
          <span className="block text-sm font-black tracking-tight text-white">{wordmark}</span>
          {sublabel && (
            <span className="mt-1 block text-[10px] uppercase tracking-[0.28em] text-slate-400">{sublabel}</span>
          )}
        </span>
      )}
    </div>
  )
}