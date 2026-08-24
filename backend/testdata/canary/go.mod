// A distribution the size of a test, kept in this repository.
//
// Its own module on purpose: a nested go.mod makes the parent's `go build ./...`
// skip it, and it is under testdata/ so the toolchain would skip it anyway. The
// replace points at the core it is checked out beside, so it always compiles
// against the working tree rather than against a tag.
module github.com/gerege-systems/open-gerege-nexus/testdata/canary

go 1.26

require (
	github.com/gerege-systems/open-gerege-nexus/backend v0.0.0
	github.com/go-chi/chi/v5 v5.3.2
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	golang.org/x/text v0.41.0 // indirect
)

replace github.com/gerege-systems/open-gerege-nexus/backend => ../../
