import type { SpecField, SpecValue } from "#/lib/api/admin-specs";

// The admin's spec editor keeps one text draft per field. These turn values into drafts and
// drafts back into the values PATCH /admin/specs/{id} takes. They only convert: whether a value
// fits its field is the API's call, so a draft that isn't a number goes out as typed and comes
// back as a field error.

/** A field's admin value as the editor's text, "" when there is none. */
export function draftOf(f: SpecField): string {
	return f.admin === null ? "" : toText(f.admin);
}

/** A value as text: lists joined with commas, booleans as "true" / "false". */
export function toText(v: SpecValue): string {
	if (v === null || v === undefined) return "";
	if (Array.isArray(v)) return v.join(", ");
	return String(v);
}

/** A draft as a value of its kind. Blank is null: no admin value. */
export function fromDraft(kind: SpecField["kind"], draft: string): SpecValue {
	const text = draft.trim();
	if (text === "") return null;
	switch (kind) {
		case "integer": {
			const n = Number(text);
			return Number.isFinite(n) ? n : text;
		}
		case "boolean":
			return text === "true" ? true : text === "false" ? false : text;
		case "string_list":
			return text
				.split(",")
				.map((s) => s.trim())
				.filter(Boolean);
		default:
			return text;
	}
}

/** The fields whose draft differs from the stored admin value, as the PATCH body's values. */
export function changedValues(
	fields: SpecField[],
	drafts: Record<string, string>,
): Record<string, SpecValue> {
	const out: Record<string, SpecValue> = {};
	for (const f of fields) {
		const draft = drafts[f.name];
		if (draft === undefined || draft.trim() === draftOf(f)) continue;
		out[f.name] = fromDraft(f.kind, draft);
	}
	return out;
}
