package store

import "database/sql"

// User mirrors the users table. hidden_friend_ids is decoded from its JSON
// text column into HiddenFriendIDs.
type User struct {
	ID                int64
	Name              string
	Email             string
	EmailVerified     sql.NullString
	PasswordHash      sql.NullString
	Image             sql.NullString
	Currency          string
	DefaultCurrency   string
	PreferredLanguage string
	ThemeColor        string
	Role              string
	DeactivatedAt     sql.NullString
	BankingID         sql.NullString
	HiddenFriendIDs   []int64
	CreatedAt         string
	// EmailExpenseNotify opts the user in to the expense email. In-app
	// notifications are the default channel, so this is off for everyone until
	// they ask for mail.
	EmailExpenseNotify bool
}

// IsAdmin reports whether the user has the ADMIN role.
func (u *User) IsAdmin() bool { return u.Role == "ADMIN" }

// IsActive reports whether the account is not deactivated.
func (u *User) IsActive() bool { return !u.DeactivatedAt.Valid }

// Notification mirrors the notifications table: one in-app event entry for one
// recipient. It is a historical record -- entity_type/entity_id link to its
// subject with no foreign key, so deleting that subject neither fails nor
// erases the notification, and title/Amount/Currency are a snapshot so the
// entry still reads correctly afterwards. No display text is stored; Kind is
// an i18n key stem rendered in the viewer's own language.
type Notification struct {
	ID         int64
	UserID     int64
	ActorID    int64
	Kind       string
	EntityType string
	EntityID   string
	Title      string
	Amount     int64 // the recipient's signed net share, minor units
	Currency   string
	ReadAt     sql.NullString
	CreatedAt  string
}

// IsRead reports whether the recipient has seen this notification.
func (n *Notification) IsRead() bool { return n.ReadAt.Valid }

// Session is a server-side session record.
type Session struct {
	Token   string
	UserID  int64
	Expires string
}

// VerificationToken backs magic-link, email verification, and password reset.
type VerificationToken struct {
	Identifier string
	Token      string
	Purpose    string // "magic" | "verify" | "reset"
	Expires    string
}

// Group mirrors the groups table.
type Group struct {
	ID               int64
	PublicID         string
	Name             string
	Image            sql.NullString
	CreatedBy        int64
	DefaultCurrency  string
	SimplifyDebts    bool
	ArchivedAt       sql.NullString
	SplitwiseGroupID sql.NullString
	CreatedAt        string
}

// IsArchived reports whether the group is archived.
func (g *Group) IsArchived() bool { return g.ArchivedAt.Valid }

// Expense mirrors the expenses table.
type Expense struct {
	ID             string
	Name           string
	Category       string
	Amount         int64
	SplitType      string
	ExpenseDate    string
	Currency       string
	PaidBy         int64
	AddedBy        int64
	UpdatedBy      sql.NullInt64
	GroupID        sql.NullInt64
	FileKey        sql.NullString
	TransactionID  sql.NullString
	RecurrenceID   sql.NullInt64
	ConversionToID sql.NullString
	MovedFromID    sql.NullString
	DeletedAt      sql.NullString
	DeletedBy      sql.NullInt64
	CreatedAt      string
	UpdatedAt      string
	Note           string
	// Version is the optimistic-concurrency token, bumped on every update. An
	// update asserts the version it read so a second editor cannot silently
	// overwrite the first (see Store.UpdateExpense).
	Version int64
}

// ExpenseParticipant is one signed row of an expense (rows sum to zero).
type ExpenseParticipant struct {
	ExpenseID string
	UserID    int64
	Amount    int64
}

// ExpenseRecurrence schedules generation of expenses from a template.
type ExpenseRecurrence struct {
	ID                int64
	CronExpression    string
	JobName           string
	TemplateExpenseID string
	Notified          bool
	CreatedBy         int64
	CreatedAt         string
	NextRunAt         sql.NullString
}

// Balance is one row of the balance_view: amount > 0 means friend owes user.
type Balance struct {
	UserID   int64
	FriendID int64
	GroupID  sql.NullInt64
	Currency string
	Amount   int64
}
