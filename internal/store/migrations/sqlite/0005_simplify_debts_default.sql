-- simplify_debts now actually gates the group page: on = minimal transfer set,
-- off = raw pairwise balances. Until this release the page simplified regardless
-- of the flag, while the column defaulted to 0 and no creation path set it — so
-- every existing group would silently flip to the raw view. Backfill to 1 to
-- preserve what those groups have always shown; new groups set it explicitly.

UPDATE groups SET simplify_debts = 1;
