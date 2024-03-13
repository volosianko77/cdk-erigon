//go:build gorules

package gorules

import (
	"github.com/quasilyte/go-ruleguard/dsl"
)

func init() {
	// Uncomment to import additional rules
	// dsl.ImportRules("qrules", quasilyterules.Bundle)
}

func txDeferRollback(m dsl.Matcher) {
	m.Match(
		`$tx, $err := $db.BeginRw($ctx); $chk; $rollback`,
		`$tx, $err = $db.BeginRw($ctx); $chk; $rollback`,
		`$tx, $err := $db.Begin($ctx); $chk; $rollback`,
		`$tx, $err = $db.Begin($ctx); $chk; $rollback`,
	).
		Where(!m["rollback"].Text.Matches(`defer .*\.Rollback()`)).Report(`
		Add "defer $tx.Rollback()" right after transaction creation error check. 
		If you are in the loop - consider use "$db.View" or "$db.Update" or extract whole transaction to function.
		Without rollback in defer - app can deadlock on error or panic.
		Rules are in ./rules.go file.`)
}

func closeCollector(m dsl.Matcher) {
	m.Match(`$c := etl.NewCollector($*_); $close`).
		Where(!m["close"].Text.Matches(`defer .*\.Close()`)).Report(`
		Add "defer $c.Close()" right after collector creation.`)
}

func closeLockedDir(m dsl.Matcher) {
	m.Match(`$c := dir.OpenRw($*_); $close`).
		Where(!m["close"].Text.Matches(`defer .*\.Close()`)).Report(`
		Add "defer $c.Close()" after locked.OpenDir.`)
}

func passValuesByContext(m dsl.Matcher) {
	m.Match(`ctx.WithValue($*_)`).Report(`
		Don't pass app-level parameters by context, pass them as-is or as typed objects.`)
}

func mismatchingUnlock(m dsl.Matcher) {
	m.Match(`$mu.Lock(); defer $mu.$unlock()`).
		Where(m["unlock"].Text == "RUnlock").
		At(m["unlock"]).Report(`
		Maybe $2mu.Unlock() was intended?
		Rules are in ./rules.go file.`)

	m.Match(`$mu.RLock(); defer $mu.$unlock()`).
		Where(m["unlock"].Text == "Unlock").
		At(m["unlock"]).Report(`
		Maybe $mu.RUnlock() was intended?
		Rules are in ./rules.go file.`)
}
