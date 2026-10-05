import { createFileRoute } from "@tanstack/react-router";
import { Search, ShoppingCart, X } from "lucide-react";
import { Price } from "#/components/product/price";
import { Stock } from "#/components/product/stock";
import { Container } from "#/components/site/container";
import { Badge } from "#/components/ui/badge";
import { Button } from "#/components/ui/button";
import { Checkbox } from "#/components/ui/checkbox";
import { Input } from "#/components/ui/input";
import { Label } from "#/components/ui/label";
import { RadioGroup, RadioGroupItem } from "#/components/ui/radio-group";
import { cn } from "#/lib/utils";
import { PreviewProductCard } from "./-cart";
import { bySlug } from "./-data";

// Token specimen. Developer tooling, so labels are in English and not translated.
export const Route = createFileRoute("/preview/")({
	component: Specimen,
});

const neutrals = [
	{ name: "background", token: "bg-background", note: "page" },
	{ name: "card", token: "bg-card", note: "cards, header, images" },
	{ name: "muted", token: "bg-muted", note: "hover, wells" },
	{ name: "border", token: "bg-border", note: "hairlines" },
	{ name: "border-strong", token: "bg-border-strong", note: "inputs, hover" },
	{ name: "subtle-fg", token: "bg-subtle-foreground", note: "meta text" },
	{ name: "muted-fg", token: "bg-muted-foreground", note: "secondary text" },
	{ name: "foreground", token: "bg-foreground", note: "text" },
	{ name: "primary", token: "bg-primary", note: "main buttons" },
];

const semantic = [
	{ name: "success", token: "bg-success", note: "in stock" },
	{ name: "warning", token: "bg-warning", note: "low stock" },
	{ name: "destructive", token: "bg-destructive", note: "errors" },
];

const typeScale = [
	{ cls: "text-5xl font-semibold tracking-[-0.035em]", label: "48 / display" },
	{ cls: "text-3xl font-semibold tracking-[-0.03em]", label: "30 / h1" },
	{ cls: "text-2xl font-semibold tracking-[-0.02em]", label: "24 / h2" },
	{ cls: "text-xl font-medium tracking-[-0.01em]", label: "20 / h3" },
	{ cls: "text-base", label: "16 / reading" },
	{ cls: "text-sm", label: "14 / UI" },
	{ cls: "text-meta text-muted-foreground", label: "13 / meta" },
	{ cls: "text-xs text-muted-foreground", label: "12 / caption" },
];

const sampleHeading = "Видео карта ASRock Intel Arc B580";
const sampleBody =
	"Жълтата дюля беше щастлива, че пухът, който цъфна, замръзна като гьон. 12 GB GDDR6, 192 b шина, 2740 MHz.";

function Section({
	title,
	note,
	children,
}: {
	title: string;
	note?: string;
	children: React.ReactNode;
}) {
	return (
		<section className="border-t py-12">
			<div className="mb-8 flex flex-col gap-1">
				<h2 className="font-semibold text-xl tracking-[-0.01em]">{title}</h2>
				{note ? (
					<p className="max-w-2xl text-muted-foreground text-sm">{note}</p>
				) : null}
			</div>
			{children}
		</section>
	);
}

function Specimen() {
	const card = bySlug("asr-vc-b580-cl-12g");

	return (
		<Container className="pt-12 pb-24">
			<header className="pb-12">
				<p className="font-mono text-subtle-foreground text-xs">/preview</p>
				<h1 className="mt-2 font-semibold text-4xl tracking-[-0.03em]">
					Design tokens
				</h1>
				<p className="mt-3 max-w-2xl text-muted-foreground">
					One light theme, near-black primary buttons, one accent.
				</p>
			</header>

			<Section
				title="Accent: signal orange"
				note="oklch(0.65 0.2 42), with brand-ink oklch(0.56 0.19 40) for text on white. Only for focus, active filters, the cart count and small highlights; main buttons stay near-black."
			>
				<div className="grid gap-4 lg:grid-cols-[1fr_20rem]">
					<div className="flex flex-col gap-6 rounded-xl border bg-card p-6">
						<div className="flex items-center gap-3">
							<span className="size-10 rounded-lg bg-brand" />
							<span className="size-10 rounded-lg bg-brand-ink" />
							<p className="ml-1 font-mono text-subtle-foreground text-xs">
								brand · brand-ink
							</p>
						</div>
						<div className="flex flex-wrap items-center gap-2 text-sm">
							<Button>
								<ShoppingCart /> Добави в количката
							</Button>
							<span className="inline-flex h-7 items-center gap-1.5 rounded-full border border-brand bg-brand/10 px-3 font-medium text-brand-ink">
								NVIDIA <X className="size-3.5" />
							</span>
							<span className="inline-flex h-7 items-center rounded-full border px-3 text-muted-foreground">
								AMD
							</span>
							<span className="inline-flex h-5 items-center rounded-full bg-brand px-2 font-semibold text-[11px] text-brand-foreground">
								Ново
							</span>
							<span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-brand px-1.5 font-semibold text-[11px] text-brand-foreground">
								3
							</span>
						</div>
						<p className="text-sm">
							Текст с{" "}
							<a
								href="#top"
								className="font-medium text-brand-ink underline-offset-4 hover:underline"
							>
								връзка към категория
							</a>{" "}
							и акцент.
						</p>
						<div className="relative max-w-md">
							<Search className="-translate-y-1/2 pointer-events-none absolute top-1/2 left-3 size-4 text-subtle-foreground" />
							<input
								readOnly
								value="Ryzen 7 7800X3D"
								className="h-9 w-full rounded-md border border-brand bg-background pl-9 text-sm outline-none ring-[3px] ring-brand/20"
							/>
						</div>
					</div>
					{card ? <PreviewProductCard product={card} /> : null}
				</div>
			</Section>

			<Section
				title="Neutrals"
				note="Zinc. Depth comes from white cards on an off-white page and hairline borders, not shadows."
			>
				<div className="grid grid-cols-3 gap-3 sm:grid-cols-5 lg:grid-cols-9">
					{neutrals.map((c) => (
						<Swatch key={c.name} {...c} />
					))}
				</div>
				<div className="mt-6 grid grid-cols-3 gap-3 sm:w-1/2 lg:w-1/3">
					{semantic.map((c) => (
						<Swatch key={c.name} {...c} />
					))}
				</div>
			</Section>

			<Section
				title="Font: Geist"
				note="Geist for everything, Geist Mono only for SKUs, spec codes and keys. Self-hosted, with Cyrillic."
			>
				<div lang="bg" className="rounded-xl border bg-card p-6">
					<p className="font-semibold text-5xl tracking-[-0.035em]">
						Всичко за компютъра
					</p>
					<p className="mt-4 font-semibold text-2xl tracking-[-0.02em]">
						{sampleHeading}
					</p>
					<p className="mt-3 max-w-2xl text-muted-foreground">{sampleBody}</p>
					<div className="mt-5 flex items-baseline gap-4">
						<Price cents={36406} className="text-3xl" />
						<span className="text-muted-foreground text-sm tabular-nums">
							1 234 567,89 €
						</span>
					</div>
					<p className="mt-4 text-sm">
						<span className="font-normal">Обикновен 400</span> ·{" "}
						<span className="font-medium">Среден 500</span> ·{" "}
						<span className="font-semibold">Получер 600</span>
					</p>
					<p className="mt-2 text-sm">абвгдежзийклмнопрстуфхцчшщъьюя</p>
					<p className="text-sm">АБВГДЕЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЬЮЯ</p>
					<p className="mt-4 font-mono text-sm">
						ASR-VC-B580-CL-12G · 2740 MHz · 249 x 132 x 41 mm · ⌘K
					</p>
				</div>
			</Section>

			<Section
				title="Type scale"
				note="Weights 400 / 500 / 600 only. Tighter tracking from 20px up. Prices always tabular."
			>
				<div className="flex flex-col divide-y rounded-xl border bg-card">
					{typeScale.map((t) => (
						<div key={t.label} className="flex items-baseline gap-6 px-6 py-4">
							<span className="w-28 shrink-0 font-mono text-subtle-foreground text-xs">
								{t.label}
							</span>
							<span className={cn("truncate", t.cls)}>{sampleHeading}</span>
						</div>
					))}
				</div>
			</Section>

			<Section
				title="Spacing, radius, layout"
				note="Tailwind's 4px scale unchanged. Page width 1280, gutter 16 / 24 / 32, header 56, section gaps 64 / 96."
			>
				<div className="flex flex-wrap items-end gap-4">
					{[1, 2, 3, 4, 6, 8, 12, 16, 24].map((n) => (
						<div key={n} className="flex flex-col items-center gap-2">
							<div
								className="bg-brand/80"
								style={{ width: n * 4, height: n * 4 }}
							/>
							<span className="font-mono text-subtle-foreground text-xs">
								{n * 4}
							</span>
						</div>
					))}
				</div>
				<div className="mt-10 flex flex-wrap gap-4">
					{[
						{ cls: "rounded-sm", label: "sm 6 · checkbox" },
						{ cls: "rounded-md", label: "md 8 · buttons, inputs" },
						{ cls: "rounded-lg", label: "lg 10 · cards" },
						{ cls: "rounded-xl", label: "xl 14 · panels" },
					].map((r) => (
						<div key={r.label} className="flex flex-col gap-2">
							<div
								className={cn(
									"size-20 border-2 border-border-strong bg-card",
									r.cls,
								)}
							/>
							<span className="font-mono text-subtle-foreground text-xs">
								{r.label}
							</span>
						</div>
					))}
				</div>
			</Section>

			<Section title="Components">
				<div className="grid gap-8 lg:grid-cols-2">
					<div className="flex flex-col gap-6 rounded-xl border bg-card p-6">
						<div className="flex flex-wrap gap-2">
							<Button>Основен</Button>
							<Button variant="outline">Вторичен</Button>
							<Button variant="secondary">Тих</Button>
							<Button variant="ghost">Призрачен</Button>
							<Button variant="link">Връзка</Button>
						</div>
						<div className="flex flex-wrap items-center gap-2">
							<Button size="sm">Малък</Button>
							<Button>Стандартен</Button>
							<Button size="lg">Голям</Button>
							<Button disabled>Неактивен</Button>
						</div>
						<div className="flex flex-wrap items-center gap-2">
							<Badge>Черен</Badge>
							<Badge variant="secondary">Сив</Badge>
							<Badge variant="outline">Контур</Badge>
							<Stock inStock />
							<Stock inStock={false} />
						</div>
					</div>
					<div className="flex flex-col gap-5 rounded-xl border bg-card p-6">
						<div className="grid gap-2">
							<Label htmlFor="s-email">Имейл</Label>
							<Input id="s-email" placeholder="ivan@example.bg" />
						</div>
						<div className="grid gap-2">
							<Label htmlFor="s-err">Телефон</Label>
							<Input id="s-err" defaultValue="0888" aria-invalid />
							<p className="text-destructive text-xs">
								Невалиден телефонен номер
							</p>
						</div>
						<div className="flex items-center gap-2">
							<Checkbox id="s-cb" defaultChecked />
							<Label htmlFor="s-cb" className="font-normal">
								Само налични
							</Label>
						</div>
						<RadioGroup defaultValue="cod" className="flex gap-6">
							<div className="flex items-center gap-2">
								<RadioGroupItem value="cod" id="s-cod" />
								<Label htmlFor="s-cod" className="font-normal">
									Наложен платеж
								</Label>
							</div>
							<div className="flex items-center gap-2">
								<RadioGroupItem value="card" id="s-card" />
								<Label htmlFor="s-card" className="font-normal">
									Карта
								</Label>
							</div>
						</RadioGroup>
					</div>
				</div>
			</Section>
		</Container>
	);
}

function Swatch({
	name,
	token,
	note,
}: {
	name: string;
	token: string;
	note: string;
}) {
	return (
		<div className="flex flex-col gap-2">
			<div className={cn("h-16 rounded-lg border", token)} />
			<div>
				<p className="font-medium text-xs">{name}</p>
				<p className="text-subtle-foreground text-xs">{note}</p>
			</div>
		</div>
	);
}
