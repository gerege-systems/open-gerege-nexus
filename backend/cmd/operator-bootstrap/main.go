/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * Creates the first operator account for the control plane. There is no web
 * registration for the console, on purpose: see internal/operator/operator.
 */

package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/security"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	email := flag.String("email", "", "the operator's e-mail address")
	name := flag.String("name", "", "the operator's name")
	role := flag.String("role", string(operator.RoleSuperadmin),
		"superadmin, operator, support or auditor")
	breakGlass := flag.Bool("break-glass", false,
		"the emergency account: its use is logged at ERROR and pages the team")
	confirm := flag.Bool("confirm", false,
		"only confirm the authenticator of an account whose enrolment was interrupted")
	flag.Usage = usage
	flag.Parse()

	if *email == "" || (*name == "" && !*confirm) {
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

	// Confirming an interrupted enrolment: the account and its secret already
	// exist, so there is no password to ask for and nothing new to print. The
	// person still holds the authenticator they enrolled — or they do not, in
	// which case the row has to be removed and the command run again.
	if *confirm {
		id, err := operator.PendingEnrolment(ctx, db, *email)
		if err != nil {
			fail("%v", err)
		}
		if !confirmLoop(ctx, db, id, *email) {
			fail("the authenticator was not confirmed")
		}
		return
	}

	password, err := security.ReadNewPassword(operator.MinPasswordLength)
	if err != nil {
		fail("%v", err)
	}

	account, enrolment, err := operator.CreateOperator(ctx, db, operator.NewOperator{
		Email:      *email,
		Name:       *name,
		Role:       operator.Role(*role),
		Password:   password,
		BreakGlass: *breakGlass,
	})
	if errors.Is(err, operator.ErrOperatorExists) {
		fail("an operator with that address already exists")
	}
	if err != nil {
		fail("%v", err)
	}

	fmt.Printf("\nOperator created: %s (%s)\n\n", account.Email, account.Role)
	if *breakGlass {
		fmt.Println("This is the BREAK-GLASS account. Put its password in the safe, not in a")
		fmt.Println("password manager anybody uses daily: signing in with it pages the team.")
		fmt.Println()
	}
	fmt.Println("Add this to an authenticator application — 1Password, Aegis, Google Authenticator:")
	fmt.Printf("\n  secret: %s\n  uri:    %s\n\n", enrolment.Secret, enrolment.URI)
	fmt.Println("The account cannot sign in until a code from it is confirmed below.")

	if confirmLoop(ctx, db, account.ID, account.Email) {
		return
	}

	fail("the authenticator was not confirmed; the account exists but cannot sign in.\n"+
		"Confirm it later without creating anything:\n"+
		"  operator-bootstrap -confirm -email %s\n"+
		"or remove the row and start over:\n"+
		"  DELETE FROM platform.operator_accounts WHERE lower(email) = lower('%s');",
		account.Email, account.Email)
}

// confirmLoop asks for a code until one is right or three are not.
//
// Looping rather than exiting on a wrong code: the alternative is an account
// that exists, cannot sign in, and cannot be created again because the address
// is taken — a support call on the day the console is first set up.
func confirmLoop(ctx context.Context, db *pgxpool.Pool, operatorID, email string) bool {
	reader := bufio.NewReader(os.Stdin)
	for attempt := 0; attempt < 3; attempt++ {
		fmt.Print("Code from the authenticator: ")
		code, err := reader.ReadString('\n')
		if err != nil {
			fail("could not read the code: %v", err)
		}
		if err := operator.ConfirmSecondFactor(ctx, db, operatorID, strings.TrimSpace(code)); err == nil {
			fmt.Printf("\nDone. %s can sign in at the control plane.\n", email)
			return true
		} else {
			fmt.Fprintf(os.Stderr, "  %v\n", err)
		}
	}
	return false
}

func usage() {
	fmt.Fprintf(os.Stderr, `operator-bootstrap — create a control plane operator.

The console has no sign-up screen. The first account is made here, by somebody
who already holds the database credentials, and every account after it is made
from the console by a superadmin.

Usage:
  DATABASE_URL=... operator-bootstrap -email you@example.mn -name "Your Name" [-role superadmin]

In production, with the compose stack running:
  docker exec -it gerege_nexus_backend /app/operator-bootstrap -email ... -name "..."

If the authenticator step was interrupted, finish it without creating anything:
  docker exec -it gerege_nexus_backend /app/operator-bootstrap -confirm -email ...

Roles: superadmin (everything), operator (daily work), support (people),
auditor (read-only). See docs/CONTROL_PLANE.md.

`)
	flag.PrintDefaults()
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "operator-bootstrap: "+format+"\n", args...)
	os.Exit(1)
}
