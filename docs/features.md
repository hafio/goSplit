# Features

What GoSplit does, screen by screen. For users and for anyone deciding whether it fits.
Operators looking for switches should read [configuration.md](configuration.md).

GoSplit is a Go rebuild of [SplitPro](https://github.com/oss-apps/split-pro), itself an
open-source Splitwise alternative. It keeps SplitPro's feature set except OAuth/OIDC
sign-in, and adds expense filtering, moving expenses between groups, archiving group
history, whole-instance backups and a few other things noted below.

## Signing in

- **Magic link**: enter your email, click the link you receive. Without a mail server
  configured, the link is printed to the server log instead, which is fine for a
  household instance where the operator can read it.
- **Password**: set one from your profile at any time, then sign in with it. Forgot-password
  and reset flows work by email. Changing your password signs you out everywhere; sign in
  again with the new one.
- **Registration** is open by default. The operator can close it
  (`DISABLE_EMAIL_SIGNUP`), in which case new users are created from the admin console.
- Addresses listed in `ADMIN_EMAILS` become **admins** automatically on first sign-in.

## Friends

Add a friend by email. If they already have an account you are linked immediately; if not,
a pending account is created for them and they are emailed an invitation. With
`ENABLE_SENDING_INVITES=false` that is refused outright: the address needs an account
first, either by registering or by an admin creating it.

A friend's page shows your balance with them per currency, netted across every group you
share, and a filterable history that includes the expenses you share in groups. From here
you can settle up, convert a balance to another currency, or collapse old history (see
[Collapsing history](#collapsing-history)). A friend can be hidden, which greys the row out
but keeps it listed, or removed outright.

## Groups

Create a group and add members either by picking an existing friend or by email. Every
group also has a **join link** you can share. Opening it requires signing in and shows
the recipient an invitation message; it does not add them by itself, so a member still
adds them with the group's *Add member* form.

The group page shows your own net position in the group, per currency, and the list of who
owes whom. A **simplify debts** toggle switches the suggestions between the minimal set of transfers that settles
everyone and the raw pairwise balances. Debt simplification uses a min-cash-flow algorithm
and never changes what anyone owes in total.

**Archiving a group** hides it from the normal lists and from your aggregate balances. It
moves into a collapsed "Archived" section that still shows its own unsettled debt, and its
expenses can still be seen in Activity with the archived filter switched on. Archive is
reversible.

## Expenses

Add an expense to a group or directly with one or more friends; the target selector at the
top of the form switches between them. Opened from a friend's page, a direct expense is
just the two of you; opened from elsewhere, it starts with every friend included.
Amounts are stored as integer minor units, never floats, and negative amounts are allowed.
Each expense has a category from a built-in emoji-labelled list (its emoji is what the
feed shows next to the date) and can carry a free-text note.

Five split methods:

| Method | You enter |
| --- | --- |
| Equal | who is included |
| Percentage | a percentage per person |
| Exact | an amount per person |
| Shares | a weight per person |
| Adjustment | an extra amount per person on top of an equal split |

Editing an expense restores the method and the values exactly as you typed them, not a
reconstruction from the computed amounts. Editing and deleting are limited to the people
involved: the payer, the creator and the participants. Deletion is a soft delete; the
detail page still shows the row marked as deleted.

**Moving an expense** to a different group, into a group, or out to a direct expense goes
through a guided re-split, because the set of people changes.

**Concurrent edits are refused, not merged.** Every expense carries a version. The edit
form and the delete button both send the version they were rendered from, and if someone
else changed the expense in between, the save is refused with a conflict message instead
of silently overwriting their change.

## Settlements

A settlement is recorded like an expense and reversed by deleting it.

- **With a friend**: settle what you owe or record what you are owed. The direction is
  worked out from the current balance, you only confirm the amount. Settling from a
  friend's page clears your net balance with them across every group.
- **Inside a group**: settle with one member, which clears that pair's balance within the
  group only, or **settle the whole group** in one action, which records the minimal set
  of transfers that nets every member to zero.
- A settlement edits as a settlement: amount, date, note and group can change; the pair,
  direction and currency are fixed. To change those, delete it and record a new one.

The difference between the two scopes matters when you share several groups with someone.
A group settlement fixes that group's books; a friend settlement fixes the overall picture.

## Balances and activity

**Balances** shows what you are owed and what you owe, per currency. Balances are never
stored; they are computed from expenses every time, so they cannot drift.

**Activity** is a feed of every expense you are part of, grouped by month, with a "you
lent / you borrowed" column per row. The same feed appears on friend and group pages
scoped to them. Each feed shows the 50 most recent matching expenses, with a "show all"
link when there are more.

### Filtering

Every feed can be filtered by description (use `*` as a wildcard), amount range and date
range. Activity adds a groups/direct scope switch and an **archived** toggle, off by
default, that brings in expenses from archived groups. Filters combine with AND, live in
the URL so they can be bookmarked, and never change any balance.

## Collapsing history

"Collapse" takes your direct two-person transactions with a friend, or a group's own
transactions, dated before a day you choose, and replaces them with one "Historical
Transactions" entry per currency that carries the same balance. Currency-conversion pairs
and expenses used as recurrence templates are left alone. The originals move to a
separate archived-expenses table and a CSV listing of them is kept in the entry's note, so
nothing is lost, but the feed gets short again. Expenses you share with a friend inside a
group still appear on the friend's page; they are collapsed from the group, not from the
friend. Use it for long-running groups and for friends you have settled with for years.

## Currency conversion

A balance with a friend in one currency can be converted to another. The conversion page
fetches a rate (Frankfurter by default, or Open Exchange Rates), lets you adjust it, and
lets you pick the direction. Both amounts and the rate stay in sync as you type, and the
amounts you confirm are stored exactly as shown. A conversion is recorded as a linked pair
of expenses: one cancels the balance in the old currency, the other recreates it in the
new one.

The operator can switch the rate provider or disable live rates entirely
(`CURRENCY_RATE_PROVIDER`).

## Recurring expenses

From the Recurring page, pick the template from your 20 most recent expenses (conversions
and deleted expenses excluded) and give it a standard 5-field cron rule (`0 9 1 * *` for
09:00 UTC on the first of every month; rules are evaluated in UTC). The server creates
each due expense exactly once, even when several replicas run. Recurrences can be deleted
from the same page.

## Notifications

Five events notify everyone involved except the person who did it: an expense added,
edited or deleted, and a settlement recorded or edited. There are three channels, and
they all read from the same record, so nothing is ever delivered that the app cannot
also show you.

- **In app** (always on): a bell in the top bar carries the unread count, opens a
  dropdown of the latest few, and links to the full list at `/notifications`. Clicking an
  entry marks it read and takes you to what it is about; "Mark all read" clears the rest.
  Entries say what changed and what it does to your own balance -- "you owe 12.50" -- and
  are written in your own language, not the language of whoever made the change.
- **Web Push** (opt in per browser): when the operator has configured VAPID keys, each
  browser can be enrolled from your profile page, with a test button. Notifications open
  the relevant page and nudge already-open tabs to refresh. Settling up a whole group
  sends one notification rather than one per transfer, though the list still records each
  settlement separately so every entry links to its own.
- **Email** (opt in per account, **off by default**): tick "Also email me about expense
  activity" on your profile page. Nothing is emailed until you do. Sign-in links,
  password resets and invitations are unaffected -- those reach you whether or not you
  have an account, so they are not notifications and cannot be turned off here.

Push and email are optional and disable cleanly when unconfigured; the in-app record does
not depend on either. An entry outlives what it refers to: deleting an expense does not
erase the notice that it was deleted, and an entry whose target is gone still leads
somewhere sensible. Read entries are kept for 90 days and unread ones for a year.

## Languages and appearance

GoSplit ships in nine languages: English, German, Spanish, French, Italian, Japanese,
Korean, Simplified Chinese and Traditional Chinese. Your profile setting wins; otherwise
the browser's `Accept-Language` decides among the seven single-tag languages. The two
Chinese variants are currently only reachable through the profile setting, because their
script-tagged codes are not matched from the header. Adding a language is a matter of
dropping a new JSON catalogue into the source tree; see
[development.md](development.md#adding-a-locale).

The interface is a hand-written design in the Splitwise style: icon buttons, initials
avatars, month-grouped feeds, designed empty states. Pick one of six accent themes on your
profile page (burgundy, the default, plus teal, indigo, forest, amber and slate). Each has
light and dark variants that follow your system setting, and the favicon recolours to
match. You can also upload an avatar.

## Profile export

Your profile page offers a JSON export of your own data. It is a takeout for your records,
not a backup, and cannot be imported. Whole-instance backups are the operator's job; see
[backup-restore.md](backup-restore.md).

## Progressive web app

GoSplit is installable on phones and desktops. A service worker caches the shell so the
app opens offline to a clear "you are offline" page rather than a browser error. The core
flows (signing in, expenses, settlements, filters, archiving) work with JavaScript
disabled; scripting makes navigation smoother. Three features need it: the conversion
page's live rate lookup, bank sync (Plaid Link), and enrolling a browser for push.

## Import from Splitwise

The Import page takes your Splitwise API key and imports your friends and groups. It does
not import expenses (matching the upstream project) and is safe to run twice: groups
already imported are skipped. The key is used for the import request and not stored.

## Bank sync (optional)

When the operator has configured Plaid, the Bank page lets you connect an account through
Plaid Link, pull recent transactions, and turn any of them into a prefilled expense with
one click. Without Plaid credentials the page simply reports the feature as off.

## Admin console

Admins get `/admin`, where they can create users, edit anyone's name, email, role, currency
and language, set or reset a password (which signs that user out everywhere), deactivate
and reactivate accounts, and mint a one-time sign-in link for a user who cannot receive
mail. `/admin/backup` handles backups and restores; see
[backup-restore.md](backup-restore.md).

## See also

- [configuration.md](configuration.md) -- switching features on and off
- [architecture.md](architecture.md) -- how balances, splits and the UI work underneath
- [backup-restore.md](backup-restore.md) -- keeping all of this safe
