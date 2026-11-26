-- Stand-in, written 2026-11-26. Production was already at this version and had this table, but the migration file
-- that created it was never pushed. The definition is copied from production so the versions line up; nothing in
-- this repository reads the table. IF NOT EXISTS keeps it harmless on a database that has it.
CREATE TABLE IF NOT EXISTS discord_role_message (
    guild_id           VARCHAR     NOT NULL,
    kind               VARCHAR     NOT NULL,
    discord_channel_id VARCHAR     NOT NULL,
    discord_message_id VARCHAR     NOT NULL,
    text               TEXT        NOT NULL DEFAULT '',
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT discord_role_message_pkey PRIMARY KEY (guild_id, kind),
    CONSTRAINT discord_role_message_discord_message_id_key UNIQUE (discord_message_id)
);
