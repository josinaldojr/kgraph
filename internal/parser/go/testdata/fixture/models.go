package fixture

import "strconv"

// User is a customer account.
type User struct {
	ID    uint   `gorm:"column:id;primaryKey"`
	Name  string `gorm:"column:name"`
	Email string `db:"email"`
}

// Order is a purchase made by a User.
type Order struct {
	ID     uint `gorm:"column:id;primaryKey"`
	UserID uint `gorm:"column:user_id"`
	Total  int  `gorm:"column:total"`
	Owner  *User
}

// Notifier can send a notification to a user.
type Notifier interface {
	Notify(u *User, message string) error
}

// GreetingFor returns a greeting for the given user.
func GreetingFor(u *User) string {
	return "Hello, " + u.Name
}

// Summary returns a short human-readable summary of the order, including a
// greeting for its owner.
func (o *Order) Summary() string {
	return GreetingFor(o.Owner) + " — total: " + itoa(o.Total)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
