/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Gives a fresh deployment its first organisation and the administrator who
 * runs it. There is no sign-up screen, on purpose: see
 * internal/operator/tenants/bootstrap.go.
 */

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/tenants"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	org := flag.String("org", "", "the organisation's name, as people will read it")
	slug := flag.String("slug", "", "the organisation's slug: lowercase letters, digits and hyphens")
	email := flag.String("email", "", "the first administrator's e-mail address")
	name := flag.String("name", "", "the first administrator's name")
	flag.Usage = usage
	flag.Parse()

	if *org == "" || *slug == "" || *email == "" {
		flag.Usage()
		os.Exit(2)
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		fail("DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	db, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		fail("could not connect to the database: %v", err)
	}
	defer db.Close()
	if err := db.Ping(ctx); err != nil {
		fail("the database is not reachable: %v", err)
	}

	// Before the password is asked for, not after: an operator who has already
	// typed one twice should not then be told the deployment was never the
	// empty one they thought it was.
	empty, err := tenants.Unprovisioned(ctx, db)
	if err != nil {
		fail("could not read the organisations: %v (have the migrations run?)", err)
	}
	if !empty {
		fail("%v — create further organisations from the console", tenants.ErrAlreadyProvisioned)
	}

	password, err := security.ReadNewPassword(tenants.MinAdminPasswordLength)
	if err != nil {
		fail("%v", err)
	}

	tenantID, _, err := tenants.Bootstrap(ctx, db, tenants.FirstTenant{
		Slug:       *slug,
		Name:       *org,
		AdminEmail: *email,
		AdminName:  *name,
		Password:   password,
	})
	if errors.Is(err, tenants.ErrAlreadyProvisioned) {
		fail("another bootstrap got there first; nothing was created")
	}
	if err != nil {
		fail("%v", err)
	}

	fmt.Printf("\n%s (%s) is open, id %s.\n", *org, *slug, tenantID)
	fmt.Printf("Sign in at PUBLIC_ORIGIN as %s with the password you just chose.\n", *email)
	fmt.Println("Apps are installed from the store once you are in.")
}

func usage() {
	fmt.Fprintf(os.Stderr, `tenant-bootstrap — open a deployment's first organisation.

A fresh deployment has no sign-up screen and no organisation, so nobody can
sign in to it. This makes the first one and its administrator, run by somebody
who already holds the database credentials. It refuses once an organisation
exists: every one after the first is made from the control plane console.

Usage:
  DATABASE_URL=... tenant-bootstrap -org "Your Organisation" -slug your-org \
      -email you@example.mn -name "Your Name"

In production, with the compose stack running:
  docker exec -it gerege_nexus_backend /app/tenant-bootstrap \
      -org "Your Organisation" -slug your-org -email you@example.mn -name "Your Name"

The password is typed at the terminal — never a flag or an environment
variable, both of which outlive the command in a shell history or a container's
inspect output. `+"`docker exec -it`"+`, not `+"`docker exec`"+`.

`)
	flag.PrintDefaults()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "tenant-bootstrap: "+format+"\n", args...)
	os.Exit(1)
}
