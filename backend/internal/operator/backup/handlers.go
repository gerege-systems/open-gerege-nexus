/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

// Package backup is what has been kept and what has been shipped:
// platform_backups, the restore test, and the release workflow.
//
// It stands on internal/operator/operator, which decides who is asking and
// records what they did; nothing in this plane stands on this package.
package backup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

// Status is what the console shows about the thing nobody thinks about
// until the morning they need it.
type Status struct {
	// Configured is false when nothing has ever reported a backup, which is
	// the state a deployment that never installed the cron job is in — and it
	// is shown as a warning rather than as an empty panel.
	Configured   bool       `json:"configured"`
	LastBackupAt *time.Time `json:"last_backup_at"`
	LastSizeMB   float64    `json:"last_size_mb"`
	LastOK       bool       `json:"last_ok"`
	LastDetail   string     `json:"last_detail"`
	// LastRestoreTestAt is recorded by hand. An untested backup is not a
	// backup, and the only way to know it has been tested is that somebody
	// says so.
	LastRestoreTestAt *time.Time `json:"last_restore_test_at"`
}

// StatusOf is what the health screen shows about what has been kept.
func (s *Service) StatusOf(ctx context.Context) Status {
	status := Status{}
	ctx = operator.Scoped(ctx)

	var size *int64
	err := s.db.QueryRow(ctx,
		`SELECT started_at, size_bytes, ok, detail FROM operator.platform_backups
		  WHERE kind = 'backup' ORDER BY started_at DESC LIMIT 1`).
		Scan(&status.LastBackupAt, &size, &status.LastOK, &status.LastDetail)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return status
	case err != nil:
		slog.Warn("control plane: could not read the backup status", "error", err)
		return status
	}
	status.Configured = true
	if size != nil {
		status.LastSizeMB = float64(*size) / (1024 * 1024)
	}

	if err := s.db.QueryRow(ctx,
		`SELECT started_at FROM operator.platform_backups
		  WHERE kind = 'restore_test' ORDER BY started_at DESC LIMIT 1`).
		Scan(&status.LastRestoreTestAt); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		slog.Warn("control plane: could not read the restore tests", "error", err)
	}
	return status
}

// RecordRestoreTest writes down that somebody restored a backup and it worked.
func (s *Service) RecordRestoreTest(ctx context.Context, sess operator.Session, detail, reason string) error {
	return s.op.Do(ctx, sess, operator.Change{
		Action:     "backup.restore_test",
		TargetType: "platform",
		TargetID:   "backups",
		Reason:     reason,
		After:      map[string]any{"detail": detail},
	}, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO operator.platform_backups (kind, finished_at, ok, detail, recorded_by)
			 VALUES ('restore_test', NOW(), TRUE, $1, $2::uuid)`, detail, sess.ID)
		return err
	})
}

// deployWorkflow is the file name in .github/workflows. Configurable, because a
// fork may have renamed it, and defaulted because most have not.
func deployWorkflow() string {
	return firstNonEmpty(os.Getenv("GITHUB_DEPLOY_WORKFLOW"), "deploy.yml")
}

// TriggerDeploy asks GitHub to run the deployment workflow at a ref.
//
// Returns the address of the workflow's own page, because the console
// deliberately does not follow the run: watching it would mean polling
// somebody else's API for minutes, and GitHub already has a screen for it that
// shows more than this console ever should.
func (s *Service) TriggerDeploy(ctx context.Context, sess operator.Session, ref, reason string) (string, error) {
	token := strings.TrimSpace(os.Getenv("GITHUB_DEPLOY_TOKEN"))
	repository := strings.TrimSpace(os.Getenv("GITHUB_REPOSITORY"))
	if token == "" || repository == "" {
		return "", ErrDeployNotConfigured
	}
	if ref = strings.TrimSpace(ref); ref == "" {
		ref = "main"
	}

	runsURL := fmt.Sprintf("https://github.com/%s/actions/workflows/%s", repository, deployWorkflow())
	err := s.op.Do(ctx, sess, operator.Change{
		Action:     "deploy.trigger",
		TargetType: "platform",
		TargetID:   repository,
		Reason:     reason,
		After:      map[string]any{"ref": ref, "workflow": deployWorkflow()},
	}, func(ctx context.Context, tx pgx.Tx) error {
		// Inside the transaction: a deployment that GitHub refused should not
		// leave an audit row saying it was triggered, and one that GitHub
		// accepted must not be missing from the trail because a commit failed
		// afterwards.
		return dispatchWorkflow(ctx, token, repository, deployWorkflow(), ref)
	})
	if err != nil {
		return "", err
	}
	slog.Warn("control plane: a deployment was triggered",
		"operator_email", sess.Email, "ref", ref, "repository", repository)
	return runsURL, nil
}

func dispatchWorkflow(ctx context.Context, token, repository, workflow, ref string) error {
	body, err := json.Marshal(map[string]string{"ref": ref})
	if err != nil {
		return err
	}
	address := fmt.Sprintf("https://api.github.com/repos/%s/actions/workflows/%s/dispatches",
		repository, workflow)

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("Content-Type", "application/json")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrDeployRefused, err)
	}
	defer func() { _ = response.Body.Close() }()

	// 204 is what a dispatch answers with. Anything else is reported without
	// the response body, which on this endpoint can name the token.
	if response.StatusCode != http.StatusNoContent {
		return fmt.Errorf("%w: %d", ErrDeployRefused, response.StatusCode)
	}
	return nil
}

var (
	// ErrDeployNotConfigured is a deployment with no token.
	ErrDeployNotConfigured = errors.New("this deployment has no GitHub token for the deploy workflow")
	// ErrDeployRefused is GitHub saying no.
	ErrDeployRefused = errors.New("GitHub refused to start the workflow")
)

func (s *Service) handleDeploy(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		Ref string `json:"ref"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	link, err := s.TriggerDeploy(r.Context(), sess, body.Ref, body.Reason)
	if err != nil {
		fail(w, err, "could not trigger the deployment")
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]string{"status": "started", "url": link})
}

func (s *Service) handleRestoreTest(w http.ResponseWriter, r *http.Request) {
	sess, _ := operator.SessionFrom(r.Context())
	var body struct {
		operator.Reasoned
		Detail string `json:"detail"`
	}
	if !operator.Decode(w, r, &body) {
		return
	}
	if err := s.RecordRestoreTest(r.Context(), sess, body.Detail, body.Reason); err != nil {
		fail(w, err, "could not record the restore test")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "recorded"})
}

// Routes are this screen's, mounted on the console's signed-in group. The
// capability each one asks for is the decision about who may see or do it.
func (s *Service) Routes(r chi.Router) {
	r.With(s.op.RequireCapability(operator.CapDeploy), s.op.RequireStepUp).
		Post("/deploy", s.handleDeploy)
	r.With(s.op.RequireCapability(operator.CapSettingsWrite)).
		Post("/backups/restore-test", s.handleRestoreTest)
}

// firstNonEmpty is the first of its arguments that says anything.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
