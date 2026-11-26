
## Tables shared with mobwebapp

`channels`, `shows`, `show_breaks` and `break_spots` live in **mobwebapp's database**, not in this service's own. They
are defined by mobwebapp's migrations (`mobwebapp/backend/db/migrations/`); this repo never creates or alters them.
The holder reaches them through a second connection pool (`MOBWEBAPP_DATABASE_URL`, sizes `MOBWEBAPP_DB_MAX_OPEN_CONNS`
default 5 and `DB_MAX_OPEN_CONNS` default 7 for the holder's own pool) and its channel, stream, break and event
repositories translate them into `entity.Channel/Stream/Break/Event` (stream = show, event = spot). The database user
has rights on these four tables and their sequences only, so no statement on that pool may touch any other table.
Shows and breaks are created only in mobwebapp. Any change to these four tables' columns or meaning is made in both
repos in the same piece of work. Plan and rules: `mobwebapp/docs/holder-data-move-plan.md`.
