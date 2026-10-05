import { locale } from "./locale";

const rules = new Intl.PluralRules(locale);

// plural picks the form for n. Bulgarian only has "one" and "other"; locales with more forms
// (few, many) can pass them too.
export function plural(
	n: number,
	forms: Partial<Record<Intl.LDMLPluralRule, string>> & { other: string },
): string {
	return forms[rules.select(n)] ?? forms.other;
}
