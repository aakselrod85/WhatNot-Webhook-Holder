// Command copy_to_mobwebapp is the one-time copy of the holder's channels, streams, breaks and events into
// mobwebapp's channels, shows, show_breaks and break_spots (step 6 of mobwebapp/docs/holder-data-move-plan.md).
//
// It reads two DSNs from the environment only (HOLDER_DATABASE_URL, MOBWEBAPP_DATABASE_URL), never from .env files.
// Default is a dry run: every step runs for real inside transactions that are rolled back at the end, so it
// validates the real SQL and prints the real counts, and changes nothing. --execute writes.
package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

// stepRewrite is the step-log row written by the last holder transaction. Once it exists the script is done for
// good: a later run never touches mobwebapp again (rows people created there after the switch are safe).
const (
	stepTranslate = "translate"
	stepRewrite   = "holder_rewrite"
)

// lockTimeout keeps the holder transactions from queueing behind (or in front of) a running server.
const lockTimeout = "10s"

type app struct {
	holder    *sqlx.DB
	mob       *sqlx.DB
	execute   bool
	since     time.Time
	chanMap   []chanMapping // mobwebapp channel key -> holder id it gets
	stopAfter int           // testing aid: exit right after this phase has committed (0 = run all)
}

type chanMapping struct {
	Key string
	ID  int64
}

func main() {
	execute := flag.Bool("execute", false, "write to both databases (default: dry run, nothing changes)")
	allowRemote := flag.Bool("allow-remote", false, "allow a DSN whose host is not localhost/127.0.0.1")
	sinceFlag := flag.String("since", "", "copy streams dated on or after this day (YYYY-MM-DD or RFC3339); default 31 days before now")
	mapFlag := flag.String("channel-map", "rips=2,breaks=1,sports=6", "mobwebapp channel key=new id, the three channels that are renumbered")
	stopAfter := flag.Int("stop-after-phase", 0, "testing aid: with --execute, exit after phase 1 or 2 has committed, to rehearse a run killed between phases")
	flag.Parse()

	holderDSN, mobDSN := os.Getenv("HOLDER_DATABASE_URL"), os.Getenv("MOBWEBAPP_DATABASE_URL")
	if holderDSN == "" || mobDSN == "" {
		fatal(errors.New("set HOLDER_DATABASE_URL and MOBWEBAPP_DATABASE_URL (the script reads no .env file)"))
	}
	for name, dsn := range map[string]string{"HOLDER_DATABASE_URL": holderDSN, "MOBWEBAPP_DATABASE_URL": mobDSN} {
		hosts := dsnHosts(dsn)
		if !*allowRemote && !allLocal(hosts) {
			fatal(fmt.Errorf("%s points at host %q, not localhost/127.0.0.1; refusing (pass --allow-remote to override)", name, strings.Join(hosts, ",")))
		}
	}

	a := &app{execute: *execute, stopAfter: *stopAfter, since: time.Now().UTC().AddDate(0, 0, -31)}
	if *sinceFlag != "" {
		t, err := time.Parse("2006-01-02", *sinceFlag)
		if err != nil {
			t, err = time.Parse(time.RFC3339, *sinceFlag)
		}
		if err != nil {
			fatal(fmt.Errorf("--since: %w", err))
		}
		a.since = t.UTC()
	}
	for _, part := range strings.Split(*mapFlag, ",") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) != 2 {
			fatal(fmt.Errorf("--channel-map: bad entry %q", part))
		}
		id, err := strconv.ParseInt(kv[1], 10, 64)
		if err != nil {
			fatal(fmt.Errorf("--channel-map: %w", err))
		}
		a.chanMap = append(a.chanMap, chanMapping{kv[0], id})
	}

	var err error
	if a.holder, err = sqlx.Connect("postgres", holderDSN); err != nil {
		fatal(fmt.Errorf("connect holder: %w", errScrub(err)))
	}
	defer a.holder.Close()
	if a.mob, err = sqlx.Connect("postgres", mobDSN); err != nil {
		fatal(fmt.Errorf("connect mobwebapp: %w", errScrub(err)))
	}
	defer a.mob.Close()

	var hasTitle int
	if err := a.mob.Get(&hasTitle, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = 'shows' AND column_name = 'whatnot_show_title'`); err != nil {
		fatal(fmt.Errorf("check shows.whatnot_show_title: %w", errScrub(err)))
	}
	if hasTitle == 0 {
		fatal(errors.New("mobwebapp's shows table has no whatnot_show_title column: apply mobwebapp migration 20261126000020_show_whatnot_title first"))
	}

	mode := "DRY RUN (nothing is changed; every step runs in a transaction that is rolled back)"
	if a.execute {
		mode = "EXECUTE"
	}
	fmt.Printf("mode: %s\ncutoff (--since): %s\n", mode, a.since.Format(time.RFC3339))

	ok, err := a.run(context.Background())
	if err != nil {
		fatal(err)
	}
	if !ok {
		fmt.Println("RESULT: MISMATCH")
		os.Exit(2)
	}
	fmt.Println("RESULT: OK")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

// errScrub keeps a connection error from echoing the DSN (it can hold a password).
func errScrub(err error) error {
	s := err.Error()
	if i := strings.Index(s, "://"); i >= 0 {
		return errors.New("connection failed (details hidden)")
	}
	return err
}

func dsnHosts(dsn string) []string {
	var hosts []string
	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return []string{"<unparseable>"}
		}
		add := func(h string) {
			for _, one := range strings.Split(h, ",") {
				if hh, _, err := net.SplitHostPort(one); err == nil {
					one = hh
				}
				hosts = append(hosts, strings.Trim(one, "[]"))
			}
		}
		add(u.Host)
		for _, k := range []string{"host", "hostaddr"} {
			if v := u.Query().Get(k); v != "" {
				add(v)
			}
		}
	} else {
		for _, f := range strings.Fields(dsn) {
			if kv := strings.SplitN(f, "=", 2); len(kv) == 2 && (kv[0] == "host" || kv[0] == "hostaddr") {
				hosts = append(hosts, strings.Trim(kv[1], "'"))
			}
		}
	}
	return hosts
}

func allLocal(hosts []string) bool {
	if len(hosts) == 0 {
		return false // no host means the unix socket or PGHOST: not provably local
	}
	for _, h := range hosts {
		// u.Host of "user:pass@localhost:5432" was already stripped by url.Parse; compare the bare host.
		if h != "localhost" && h != "127.0.0.1" && h != "::1" {
			return false
		}
	}
	return true
}

// q is what both *sqlx.DB and *sqlx.Tx offer, so reads run the same against a transaction or a pool.
type q interface {
	Select(dest interface{}, query string, args ...interface{}) error
	Get(dest interface{}, query string, args ...interface{}) error
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func sortedKeys(m map[int64]int64) []int64 {
	keys := make([]int64, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}
