package cart

type Cart struct {
	ID     int64
	UserID int64
	Status string
	Items  []CartItem
}

type CartItem struct {
	CartID    int64
	ProductID int64
	Quantity  int
}
