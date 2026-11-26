package main

import (
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// oldTables are the six tables that stay untouched (renamed, altered or dropped later, by hand).
const oldTables = `ARRAY['public.channel', 'public.stream', 'public.break', 'public.event', 'public.whatnot_show', 'public.stream_whatnot_show']::regclass[]`

// rewriteTables hold a stream_id that points at a holder stream. Their rows of streams that were not copied are moved
// to <table>_archive; the rest get the new show id.
var rewriteTables = []string{"show_reconciliation", "stream_shipment"}

func (a *app) phase3(htx *sqlx.Tx, tr *Translation) error {
	done, err := stepDone(htx, stepRewrite)
	if err != nil {
		return err
	}
	if done {
		fmt.Println("already done")
		return nil
	}

	// 1. Foreign keys from tables that stay into the six old tables.
	var fks []struct {
		Tbl  string `db:"tbl"`
		Name string `db:"name"`
		Def  string `db:"def"`
	}
	if err := htx.Select(&fks, `SELECT c.conrelid::regclass::text AS tbl, c.conname AS name, pg_get_constraintdef(c.oid) AS def
		FROM pg_constraint c
		WHERE c.contype = 'f' AND c.confrelid = ANY (`+oldTables+`) AND NOT (c.conrelid = ANY (`+oldTables+`))
		ORDER BY 1, 2`); err != nil {
		return err
	}
	var dropped []string
	for _, f := range fks {
		if _, err := htx.Exec(`ALTER TABLE ` + f.Tbl + ` DROP CONSTRAINT ` + pq.QuoteIdentifier(f.Name)); err != nil {
			return fmt.Errorf("drop %s.%s: %w", f.Tbl, f.Name, err)
		}
		dropped = append(dropped, f.Tbl+"."+f.Name+" ("+f.Def+")")
		fmt.Printf("dropped foreign key %s.%s: %s\n", f.Tbl, f.Name, f.Def)
	}

	// 2 + 3. Move aside, then rewrite.
	for _, t := range rewriteTables {
		var neg int
		if err := htx.Get(&neg, `SELECT count(*) FROM `+t+` WHERE stream_id < 0`); err != nil {
			return err
		}
		if neg > 0 {
			return fmt.Errorf("%s has negative stream ids; the two-step rewrite needs them free", t)
		}
		if _, err := htx.Exec(`CREATE TABLE IF NOT EXISTS ` + t + `_archive (LIKE ` + t + `)`); err != nil {
			return err
		}
		notCopied := `stream_id NOT IN (SELECT old_id FROM copy_translation WHERE entity = 'stream')`
		var total, aside int64
		if err := htx.Get(&total, `SELECT count(*) FROM `+t); err != nil {
			return err
		}
		res, err := htx.Exec(`INSERT INTO ` + t + `_archive SELECT * FROM ` + t + ` WHERE ` + notCopied)
		if err != nil {
			return err
		}
		aside, _ = res.RowsAffected()
		if _, err := htx.Exec(`DELETE FROM ` + t + ` WHERE ` + notCopied); err != nil {
			return err
		}
		// Two steps (to negative, then back) so no row ever collides with another old or new id on a unique index.
		res, err = htx.Exec(`UPDATE ` + t + ` x SET stream_id = -tr.new_id FROM copy_translation tr WHERE tr.entity = 'stream' AND tr.old_id = x.stream_id`)
		if err != nil {
			return err
		}
		rewritten, _ := res.RowsAffected()
		if rewritten != total-aside {
			return fmt.Errorf("%s: %d rows kept but %d rewritten", t, total-aside, rewritten)
		}
		if _, err := htx.Exec(`UPDATE ` + t + ` SET stream_id = -stream_id WHERE stream_id < 0`); err != nil {
			return err
		}
		fmt.Printf("%-22s %d rows: %d moved to %s_archive, %d stream_id rewritten\n", t, total, aside, t, rewritten)
	}

	_, err = htx.Exec(`INSERT INTO copy_step_log (step, info) VALUES ($1, $2)`, stepRewrite,
		fmt.Sprintf("dropped %d foreign keys: %s", len(dropped), strings.Join(dropped, "; ")))
	return err
}

const idLike = `(?:stream|break|day|event|show)[_ -]?id`

// jsonScan looks through the JSON and text columns of holder tables for anything that looks like a stream, break,
// show or event id. Event logs (webhook payloads, reconciliation reports, Discord messages) are skipped on purpose.
func (a *app) jsonScan(hq q) error {
	var cols []struct {
		Table string `db:"table_name"`
		Col   string `db:"column_name"`
		Type  string `db:"data_type"`
	}
	if err := hq.Select(&cols, `SELECT table_name, column_name, data_type FROM information_schema.columns
		WHERE table_schema = 'public' AND data_type IN ('json', 'jsonb', 'text', 'character varying')
		AND table_name NOT IN ('channel', 'stream', 'break', 'event', 'whatnot_show', 'stream_whatnot_show', 'copy_translation',
			'copy_step_log', 'schema_migrations', 'whatnot_webhook_event', 'show_reconciliation', 'show_reconciliation_archive',
			'discord_show_message', 'stream_shipment', 'stream_shipment_archive')
		AND table_name IN (SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE')
		ORDER BY 1, 2`); err != nil {
		return err
	}
	fmt.Printf("\n== JSON/text scan for stream/break/show/event id references (%d columns, regex %s) ==\n", len(cols), idLike)
	hits := 0
	for _, c := range cols {
		var rows []struct {
			N      int    `db:"n"`
			Sample string `db:"sample"`
		}
		if err := hq.Select(&rows, `SELECT count(*) AS n, COALESCE(max(substring(left(`+pq.QuoteIdentifier(c.Col)+`::text, 4000) from '.{0,40}'||$1||'.{0,40}')), '') AS sample
			FROM `+pq.QuoteIdentifier(c.Table)+` WHERE `+pq.QuoteIdentifier(c.Col)+`::text ~* $1`, idLike); err != nil {
			return fmt.Errorf("scan %s.%s: %w", c.Table, c.Col, err)
		}
		if rows[0].N > 0 {
			hits++
			fmt.Printf("  HIT %s.%s (%s): %d rows, e.g. %q\n", c.Table, c.Col, c.Type, rows[0].N, rows[0].Sample)
		}
	}
	if hits == 0 {
		fmt.Println("  no hits")
	}
	return nil
}
