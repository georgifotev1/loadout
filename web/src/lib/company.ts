// company is the seller the legal pages and the contacts page name. The order email
// (internal/notify, Seller) names the same.
// TODO(company): the registered company's details, once the company exists.
export const company = {
	name: "[Име на фирмата] ЕООД",
	id: "[ЕИК]",
	vatId: "[ДДС номер, ако фирмата е регистрирана по ДДС]",
	address: "[Седалище и адрес на управление]",
	email: "[имейл за връзка]",
	phone: "[телефон]",
	hours: "[работно време, напр. понеделник – петък, 9:00 – 18:00]",
};

// termsEffective is when the current terms took effect. Orders store it as the version their
// customer accepted (orders.TermsVersion): change both together.
export const termsEffective = "2026-10-12";
