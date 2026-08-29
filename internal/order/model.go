package order

type Order struct {
	ID     int64
	Status string
	Items  []Item
}

type Item struct {
	ID           int64
	ProductID    int64
	Quantity     int
	UnitPrice    int64
	CurrencyCode string
}
