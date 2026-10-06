import { useMemo } from "react";
import { encode } from "uqr";

// QrCode draws `value` as an SVG QR code, one path for all dark modules. It keeps the 4-module
// quiet zone that scanners need and stays black on white whatever the theme.
export function QrCode({
	value,
	label,
	className,
}: {
	value: string;
	label: string;
	className?: string;
}) {
	const { size, path } = useMemo(() => {
		const qr = encode(value, { border: 4 });
		let d = "";
		qr.data.forEach((row, y) => {
			row.forEach((dark, x) => {
				if (dark) d += `M${x} ${y}h1v1h-1z`;
			});
		});
		return { size: qr.size, path: d };
	}, [value]);

	return (
		<svg
			role="img"
			aria-label={label}
			viewBox={`0 0 ${size} ${size}`}
			shapeRendering="crispEdges"
			className={className}
		>
			<rect width={size} height={size} fill="white" />
			<path d={path} fill="black" />
		</svg>
	);
}
