package config

import "testing"

func TestLooksLikePostgresKeywordDSN(t *testing.T) {
	yes := []string{
		"host=db port=5432 user=u password=p dbname=app",
		"host='db-postgre' port='5432' user='splitpro' password='xxfIh]UG^Gf3[.=^ALwn' dbname='splitpro'",
		"dbname=app",
		"user=u password=has=equals=inside dbname=app",
	}
	for _, s := range yes {
		if !looksLikePostgresKeywordDSN(s) {
			t.Errorf("expected keyword DSN: %q", s)
		}
	}
	no := []string{
		"file:./data/gosplit.db",
		"postgres://u:p@host:5432/db", // URL form is matched by the scheme check, not this
		"sqlite:///tmp/x.db",
		"./data/gosplit.db",
		"",
	}
	for _, s := range no {
		if looksLikePostgresKeywordDSN(s) {
			t.Errorf("did not expect keyword DSN: %q", s)
		}
	}
}
