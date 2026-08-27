import { RadialBar, RadialBarChart } from 'recharts'

export function Gauge({
  label,
  percent,
  detail,
}: {
  label: string
  percent: number
  detail: string
}) {
  // A legitimately-zero value (e.g. 0% CPU usage) is dropped entirely by the
  // backend's protobuf-JSON encoding (omitempty), arriving here as
  // undefined rather than 0 — guard against that turning into NaN.
  const safePercent = Number.isFinite(percent) ? percent : 0
  const clamped = Math.min(100, Math.max(0, safePercent))
  const data = [{ value: clamped }]

  return (
    <div className="flex flex-col items-center gap-1">
      <div className="relative">
        <RadialBarChart
          width={140}
          height={140}
          cx="50%"
          cy="50%"
          innerRadius="70%"
          outerRadius="100%"
          barSize={12}
          data={data}
          startAngle={90}
          endAngle={-270}
        >
          <RadialBar background dataKey="value" cornerRadius={6} fill="var(--color-primary)" />
        </RadialBarChart>
        <div className="absolute inset-0 flex items-center justify-center text-lg font-semibold">
          {Math.round(clamped)}%
        </div>
      </div>
      <div className="text-sm font-medium">{label}</div>
      <div className="text-xs opacity-60">{detail}</div>
    </div>
  )
}
