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
- Addresses listed in `ADMIN_EMAILS` become **admins** automatically, once the address is
  verified: at the first sign-in with a magic link, or the first sign-in after a password
  reset. Registering an address, or being invited under it, does not verify it.
- **Signing out** also clears what the browser kept of the account: the offline copies of
  its pages and the service worker, which the next page load registers again.

### Roles

Every account has one of three roles:

- **USER**: a full account. Everyone who signs up themselves (the register page, or a
  first magic-link request for a new address) gets this role.
- **LIMITED**: everything a user can do except create groups, which also rules out the
  Splitwise import (it creates groups). A limited user can still be added to groups, add
  and settle expenses, and invite people. This is the default
  for every account created on someone else's behalf: by an admin in the console, by a
  friend or group invite to a new address, and by a Splitwise import.
- **ADMIN**: a full account plus the [admin console](#admin-console).

An address in `ADMIN_EMAILS` ends up an admin once its owner verifies it (see
[Signing in](#signing-in)), limited or not; until then it keeps the role it was created
with, so whoever registers or invites the address first gains nothing. Only an admin
changes roles;
nothing demotes anyone automatically. While self-registration is open, LIMITED is advisory
rather than a hard boundary: anyone, a limited user included, can register a second
address and get a full account. Close registration (`DISABLE_EMAIL_SIGNUP=true`) when the
restriction matters.

## Friends

Add a friend by email. If they already have an account you are linked immediately; if not,
a pending **limited** account is created for them (see [Roles](#roles)) and they are
emailed an invitation. An invited address cannot use the register page, since the account
already exists: the invitee signs in with a magic link or the forgot-password flow, and
stays limited until an admin changes the role. With `ENABLE_SENDING_INVITES=false` an
invite is refused outright: the address needs an account first, either by registering or
by an admin creating it.

A friend's page shows your balance with them per currency, summed across every group you
share, and a filterable history that includes the expenses you share in groups. From here
you can settle up, convert a balance to another currency, or collapse old history (see
[Collapsing history](#collapsing-history)). A friend can be hidden, which greys the row out
but keeps it listed, or removed outright.

These pages exist for anyone you share something with -- a friendship, a group, or any
expense, past ones included -- which is everyone the Balances page lists. For anyone else
they answer "not found", like an unknown id, so they cannot be used to look up an
account's name or email. The same rule decides whom you can put money against outside a
group: a direct expense, a friend settlement or a currency conversion with someone you
share nothing with is refused.

## Groups

Create a group and add members either by picking an existing friend or by email (limited
accounts cannot create groups; they can be added to any). Every
group also has a **join link** you can share. Opening it requires signing in and shows
the recipient an invitation message; it does not add them by itself, so a member still
adds them with the group's *Add member* form.

The group page shows your own net position in the group, per currency, and the list of who
owes whom. A **simplify debts** toggle switches the suggestions between the minimal set of transfers that settles
everyone and the pairwise balances as recorded. Debt simplification uses a min-cash-flow algorithm
and never changes what anyone owes in total. Settling up rewires the recorded pairs to the
simplified debts (see [Settlements](#settlements)), so after a settlement the two views
agree; a settled group with leftover pairs offers **Tidy up balances**.

**Archiving a group** hides it from the normal lists and from your aggregate balances. It
moves into a collapsed "Archived" section that still shows its own unsettled debt, and its
expenses can still be seen in Activity with the archived filter switched on. Archive is
reversible.

Only members can act on a group: archiving, the simplify toggle, adding members, settling up
and collapsing history all answer 403 to anyone else, and the expense forms never list a
group's members to someone outside it. An admin can still open any group's page to read it
(see [Admin console](#admin-console)), but gets none of these actions there.

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
involved: the payer, the creator and the participants -- and admins, who can edit, move or
delete any transaction (see [Admin console](#admin-console)). Automatic "Group debts
simplified" entries (see [Settlements](#settlements)) cannot be edited or deleted by
anyone. Someone else opening the edit
form gets a refusal. An expense's own page opens for the people involved, for members of
its group and for admins; for anyone else it is "not found". Deletion is a soft delete;
the detail page still shows the row marked as deleted, with who deleted it and when.

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
A friend settlement is direct: paying a friend from their page leaves any group's own
debts where they are.

**Group debts simplified.** Paying a group's simplified debts settles every member's net,
but on its own it would leave the pairs uneven: if Alice owed Bob and Bob owed Carol, and
Alice paid Carol, the pairwise books would read "Alice owes Bob, Bob owes Carol, Carol
owes Alice". So after a settlement inside a group -- always for *settle the whole group*,
and for a single payment or a settlement edit or delete when the group's **simplify
debts** is on -- the app adds automatic **"Group debts simplified"** entries that rewire
each pair to match the group's simplified debts. After a full settle-up every pair reads
zero, so the Balances page and friend pages read settled too. The entries never change
anyone's total, appear read-only in feeds and friend histories, send no notifications, and
cannot be edited or deleted by anyone; deleting or editing the settlement they came from
rewires the pairs again. A pair can show a debt between two people who never shared an
expense (Alice owing Carol above): that is the simplified debt.

A group settled before this existed can still show leftover pairs on Balances while its
own page reads settled; its members get a **Tidy up balances** action on the group page,
which records the same kind of entries to clear them.

## Balances and activity

**Balances** shows what you are owed and what you owe, per currency. Balances are never
stored; they are computed from expenses every time, so they cannot drift. The list is one
row per person and currency, each pair's direct expenses and shared groups summed together.
It is not simplified across people (A owes B and B owes C stays two rows), and the page says
so above the list; the minimal set of payments is on each group's page (see
[Groups](#groups)). Once a group is settled, its pairs are rewired to match (see
[Settlements](#settlements)), so they drop off this page.

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
and expenses used as recurrence templates are left alone; "Group debts simplified"
entries collapse like any other row, so nets stay exact. The originals move to a
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

From the Recurring page, pick the template from your 20 most recent expenses (conversions,
history collapses, "Group debts simplified" entries and deleted expenses excluded) and give it a standard 5-field cron rule (`0 9 1 * *` for
09:00 UTC on the first of every month; rules are evaluated in UTC). The server creates
each due expense exactly once, even when several replicas run. A recurring group
settlement rewires the group's pairs like a manual one (see [Settlements](#settlements)).
Only an expense you are part of can be made to recur. Recurrences can be deleted from the
same page.

## Notifications

Five events notify everyone involved except the person who did it: an expense added,
edited or deleted, and a settlement recorded or edited. Automatic "Group debts simplified"
entries notify no one. There are three channels, and
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
not a backup, and cannot be imported. It holds your account details (name, email, currencies,
language, theme, role), each friend's name and email, your groups (name, currency, simplify
setting, archived date) and the expenses your Activity page shows, with their splits. It
never contains password hashes, bank links or group join links. Whole-instance backups are the operator's job; see
[backup-restore.md](backup-restore.md).

## Progressive web app

GoSplit is installable on phones and desktops. A service worker caches the shell so the
app opens offline to a clear "you are offline" page rather than a browser error. It never
keeps a copy of the admin pages, or of other people's data an admin opens, so none of it
stays readable offline on the admin's device. The core
flows (signing in, expenses, settlements, filters, archiving) work with JavaScript
disabled; scripting makes navigation smoother. Three features need it: the conversion
page's live rate lookup, bank sync (Plaid Link), and enrolling a browser for push.

## Import from Splitwise

The Import page takes your Splitwise API key and imports your friends and groups. It does
not import expenses (matching the upstream project) and is safe to run twice: groups
already imported are skipped. The key is used for the import request and not stored.
Accounts the import creates for your Splitwise friends and group members are limited; an
address in `ADMIN_EMAILS` is promoted once its owner verifies it. Because it creates groups,
the import is not available
to limited accounts: the menu entry is hidden and the page answers with an explanation.

## Bank sync (optional)

When the operator has configured Plaid, the Bank page lets you connect an account through
Plaid Link, pull recent transactions, and turn any of them into a prefilled expense with
one click. Without Plaid credentials the page simply reports the feature as off.

## Admin console

Admins get `/admin`, where they can create users, edit anyone's name, email, role (LIMITED,
USER or ADMIN, see [Roles](#roles); the Add user form starts at LIMITED, so pick USER for
someone who should create groups), currency and language, set or reset a password (which signs that user out everywhere), deactivate
and reactivate accounts, and mint a one-time sign-in link for a user who cannot receive
mail. `/admin/backup` handles backups and restores; see
[backup-restore.md](backup-restore.md).

**All activity** (`/admin/activity`) lists every user's transactions, soft-deleted ones
included (dimmed, tagged and read-only), with each row's total rather than a "you lent"
column. Besides the usual filters it can narrow to one user (the transactions they paid,
created or are part of; each user's card in the console links there) or one group, and it
always includes archived groups. Opening a transaction gives an admin the same powers a
participant has: edit it, move it to another group, or delete it. When moving into a
group, the payer and everyone in the split must belong to it; only the admin does not need
to. Participants are notified as for any other edit, with the admin named, and the detail
page tells the admin they are acting on someone else's transaction. Admins cannot add new
expenses to, settle, or otherwise act on a group they are not in.

**All groups** (`/admin/groups`) lists every group with its member count, creator and a
link into the activity list. An admin can open any group's page read-only: its balances,
simplified debts and feed are all there, but none of the member controls (settle, add
member, archive, the simplify toggle, collapse), and never its join link.

## See also

- [configuration.md](configuration.md) -- switching features on and off
- [architecture.md](architecture.md) -- how balances, splits and the UI work underneath
- [backup-restore.md](backup-restore.md) -- keeping all of this safe
