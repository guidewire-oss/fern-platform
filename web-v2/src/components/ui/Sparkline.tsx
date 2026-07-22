// Tiny inline-SVG sparkline. No chart-lib dep; the math is small and
// the bundle stays slim. Values are normalized to the SVG viewBox so
// the caller's container sizes via Tailwind classes.

interface Props {
  values: number[];      // y values, in arbitrary units
  min?: number;          // override clipping; defaults to data min
  max?: number;          // override clipping; defaults to data max
  color?: string;        // line stroke
  fill?: string;         // optional area fill
  className?: string;    // container sizing via Tailwind
  showDots?: boolean;
  height?: number;       // viewBox height, default 32
  width?: number;        // viewBox width, default 100
}

export function Sparkline({
  values,
  min,
  max,
  color = 'currentColor',
  fill,
  className,
  showDots = false,
  height = 32,
  width = 100,
}: Props) {
  if (values.length === 0) {
    return (
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className={className}
        preserveAspectRatio="none"
        aria-hidden
      >
        <line x1={0} y1={height / 2} x2={width} y2={height / 2}
              stroke="currentColor" strokeWidth="1" opacity="0.2" />
      </svg>
    );
  }

  const lo = min ?? Math.min(...values);
  const hi = max ?? Math.max(...values);
  const span = hi - lo || 1;
  const padY = 2;
  const usable = height - padY * 2;
  const stepX = values.length > 1 ? width / (values.length - 1) : 0;

  const points = values.map((v, i) => {
    const x = i * stepX;
    const y = padY + (1 - (v - lo) / span) * usable;
    return { x, y };
  });

  const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(2)} ${p.y.toFixed(2)}`).join(' ');
  const area = `${line} L ${width} ${height} L 0 ${height} Z`;

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      className={className}
      preserveAspectRatio="none"
      role="img"
      aria-label="sparkline"
    >
      {fill && <path d={area} fill={fill} />}
      <path d={line} fill="none" stroke={color} strokeWidth="1.5"
            strokeLinecap="round" strokeLinejoin="round" />
      {showDots &&
        points.map((p, i) => (
          <circle key={i} cx={p.x} cy={p.y} r="1.5" fill={color} />
        ))}
    </svg>
  );
}
