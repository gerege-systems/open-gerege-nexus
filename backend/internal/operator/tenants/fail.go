/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package tenants

import (
	"errors"
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/kernel/httpx"
	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// fail answers a refusal: this screen's own in words the operator can act on,
// and everything else through the console's shared answer.
func fail(w http.ResponseWriter, err error, doing string) {
	switch {
	case errors.Is(err, ErrInvalidSlug), errors.Is(err, ErrSlugTaken), errors.Is(err, ErrNotSuspended), errors.Is(err, ErrNotScheduled), errors.Is(err, ErrUnknownEnforcement):
		httpx.Error(w, http.StatusBadRequest, err.Error())
	default:
		operator.Fail(w, err, doing)
	}
}
