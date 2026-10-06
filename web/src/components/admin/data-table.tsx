import {
	type ColumnDef,
	createColumnHelper,
	type RowData,
	tableFeatures,
	useTable,
} from "@tanstack/react-table";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "#/components/ui/table";
import { cn } from "#/lib/utils";

// The admin's tables share one feature set. Sorting, filtering and paging happen in the API
// (the URL holds them), so no row models beyond the core one are registered. `meta.className`
// styles a column's header and cells.
const features = tableFeatures({
	columnMeta: {} as { className?: string },
});

type Features = typeof features;

/** A column helper for rows of type T, e.g. `const col = adminColumns<Item>()`. */
export function adminColumns<T extends RowData>() {
	return createColumnHelper<Features, T>();
}

// Each column has its own value type, so a list of them can't name one; TanStack's own
// `columns()` helper returns this type too.
// biome-ignore lint/suspicious/noExplicitAny: see above
export type AdminColumns<T extends RowData> = ColumnDef<Features, T, any>[];

// DataTable renders rows the API already filtered and paged. `empty` shows in place of the body
// when there are none.
export function DataTable<T extends RowData>({
	columns,
	data,
	getRowId,
	empty,
	className,
}: {
	/** Defined at module scope (or memoized), so the table doesn't rebuild every render. */
	columns: AdminColumns<T>;
	data: T[];
	getRowId: (row: T) => string;
	empty: React.ReactNode;
	className?: string;
}) {
	const table = useTable({ features, columns, data, getRowId });

	return (
		<div className={cn("rounded-lg border bg-card", className)}>
			<Table>
				<TableHeader>
					{table.getHeaderGroups().map((group) => (
						<TableRow key={group.id} className="hover:bg-transparent">
							{group.headers.map((header) => (
								<TableHead
									key={header.id}
									className={cn(
										"px-3 text-muted-foreground text-xs",
										header.column.columnDef.meta?.className,
									)}
								>
									{header.isPlaceholder ? null : (
										<table.FlexRender header={header} />
									)}
								</TableHead>
							))}
						</TableRow>
					))}
				</TableHeader>
				<TableBody>
					{table.getRowModel().rows.length === 0 ? (
						<TableRow className="hover:bg-transparent">
							<TableCell
								colSpan={columns.length}
								className="px-3 py-10 text-center text-muted-foreground whitespace-normal"
							>
								{empty}
							</TableCell>
						</TableRow>
					) : (
						table.getRowModel().rows.map((row) => (
							<TableRow key={row.id}>
								{row.getAllCells().map((cell) => (
									<TableCell
										key={cell.id}
										className={cn(
											"px-3 py-2.5",
											cell.column.columnDef.meta?.className,
										)}
									>
										<table.FlexRender cell={cell} />
									</TableCell>
								))}
							</TableRow>
						))
					)}
				</TableBody>
			</Table>
		</div>
	);
}
