package repository

import "github.com/lib/pq"

// slugArray wraps a []string for use as a Postgres text[] parameter. nil is
// normalized to an empty slice so NOT NULL array columns never receive SQL NULL.
func slugArray(s []string) any {
	if s == nil {
		s = []string{}
	}
	return pq.Array(s)
}

// uuidArray wraps a []string for use as a Postgres uuid[] parameter. nil is
// normalized to an empty slice — same reason as slugArray.
func uuidArray(s []string) any {
	if s == nil {
		s = []string{}
	}
	return pq.Array(s)
}
