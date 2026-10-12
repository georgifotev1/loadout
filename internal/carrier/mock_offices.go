package carrier

import "context"

// Offices returns a few invented offices and parcel lockers in the larger cities, enough to try
// checkout's picker.
func (m *Mock) Offices(ctx context.Context) ([]Office, error) {
	return mockOffices, nil
}

const (
	weekHours   = "пн–пт 09:00–19:00, сб 09:00–14:00"
	lockerHours = "денонощно"
)

var mockOffices = []Office{
	{ID: "101", Kind: KindOffice, Name: "София Център", City: "София", Postcode: "1000", Address: "ул. Позитано 5", Hours: weekHours},
	{ID: "102", Kind: KindOffice, Name: "София Лозенец", City: "София", Postcode: "1164", Address: "бул. Черни връх 32", Hours: weekHours},
	{ID: "103", Kind: KindOffice, Name: "София Младост", City: "София", Postcode: "1712", Address: "бул. Александър Малинов 51", Hours: weekHours},
	{ID: "104", Kind: KindOffice, Name: "София Люлин", City: "София", Postcode: "1324", Address: "бул. Царица Йоанна 15", Hours: weekHours},
	{ID: "105", Kind: KindAPT, Name: "Автомат София Студентски град", City: "София", Postcode: "1700", Address: "ул. Акад. Стефан Младенов 3", Hours: lockerHours},
	{ID: "106", Kind: KindAPT, Name: "Автомат София Изток", City: "София", Postcode: "1113", Address: "бул. Цариградско шосе 90", Hours: lockerHours},
	{ID: "201", Kind: KindOffice, Name: "Пловдив Център", City: "Пловдив", Postcode: "4000", Address: "ул. Иван Вазов 20", Hours: weekHours},
	{ID: "202", Kind: KindOffice, Name: "Пловдив Тракия", City: "Пловдив", Postcode: "4023", Address: "бул. Освобождение 44", Hours: weekHours},
	{ID: "203", Kind: KindAPT, Name: "Автомат Пловдив Кършияка", City: "Пловдив", Postcode: "4003", Address: "бул. България 120", Hours: lockerHours},
	{ID: "301", Kind: KindOffice, Name: "Варна Център", City: "Варна", Postcode: "9000", Address: "бул. Сливница 10", Hours: weekHours},
	{ID: "302", Kind: KindOffice, Name: "Варна Владиславово", City: "Варна", Postcode: "9023", Address: "бул. Трети март 60", Hours: weekHours},
	{ID: "303", Kind: KindAPT, Name: "Автомат Варна Чайка", City: "Варна", Postcode: "9010", Address: "ул. Студентска 1", Hours: lockerHours},
	{ID: "401", Kind: KindOffice, Name: "Бургас Център", City: "Бургас", Postcode: "8000", Address: "ул. Александровска 70", Hours: weekHours},
	{ID: "402", Kind: KindAPT, Name: "Автомат Бургас Меден рудник", City: "Бургас", Postcode: "8009", Address: "ж.к. Меден рудник, зона А 12", Hours: lockerHours},
	{ID: "501", Kind: KindOffice, Name: "Русе Център", City: "Русе", Postcode: "7000", Address: "ул. Александровска 50", Hours: weekHours},
	{ID: "601", Kind: KindOffice, Name: "Стара Загора Център", City: "Стара Загора", Postcode: "6000", Address: "бул. Цар Симеон Велики 100", Hours: weekHours},
	{ID: "701", Kind: KindOffice, Name: "Плевен Център", City: "Плевен", Postcode: "5800", Address: "ул. Данаил Попов 8", Hours: weekHours},
}
