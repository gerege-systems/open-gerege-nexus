/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package catalog

import (
	"net/http"

	"github.com/gerege-systems/open-gerege-nexus/backend/internal/operator/operator"
)

// fail answers a refusal. This screen has no sentinels of its own, so every
// answer is the console's shared one.
func fail(w http.ResponseWriter, err error, doing string) { operator.Fail(w, err, doing) }
