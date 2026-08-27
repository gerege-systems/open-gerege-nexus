package operator

// Role is what an operator is allowed to be. The four come from §2.2 of the
// plan and are stored as text, checked by the database as well (migration
// 00049), because an unrecognised role must not be a role that quietly means
// "superadmin" to a `switch` with no default.
type Role string

const (
	// RoleSuperadmin may do everything, including the two-person actions —
	// it is also the only role that can approve another superadmin's.
	RoleSuperadmin Role = "superadmin"
	// RoleOperator does the daily work: tenants, settings, deployments. Not
	// deletion, and nothing that reveals a person's data.
	RoleOperator Role = "operator"
	// RoleSupport answers for people: find an account, reset it, look inside
	// an organisation with consent and a reason.
	RoleSupport Role = "support"
	// RoleAuditor reads. Every screen, no button.
	RoleAuditor Role = "auditor"
)

// Capability is one thing a role may do. Roles are compared against these
// rather than against each other, so that adding a role later is a row in one
// table instead of an inequality every handler got slightly differently.
type Capability string

const (
	// CapTenantRead is the organisation list and its detail pages.
	CapTenantRead Capability = "tenant.read"
	// CapAuditRead is the operator audit trail. Every role has it: a console
	// where the people using it cannot see what they did to each other is not
	// one anybody should trust.
	CapAuditRead Capability = "audit.read"
	// CapOperatorRead is the roster of operators. Who can reach this platform
	// is itself a thing to be able to check.
	CapOperatorRead Capability = "operator.read"

	// CapTenantCreate opens an organisation on this deployment.
	CapTenantCreate Capability = "tenant.create"
	// CapTenantSuspend closes one, and opens it again. Reversible, so it is
	// day-to-day work rather than a two-person action.
	CapTenantSuspend Capability = "tenant.suspend"
	// CapTenantDelete asks for an organisation to be deleted. Asking is all it
	// does: the deletion needs a second superadmin's approval, then thirty
	// days, and either can stop it.
	CapTenantDelete Capability = "tenant.delete"
	// CapApprove answers somebody else's request. Held only by superadmin, and
	// never by the operator who made the request — the database enforces that
	// half (migration 00050).
	CapApprove Capability = "approval.decide"
	// CapQuotaWrite sets an organisation's limits.
	CapQuotaWrite Capability = "quota.write"
	// CapSupport is the help-desk work: find a person, unlock them, end their
	// sessions, send them a way back in. None of it reveals anything they
	// keep on the platform.
	CapSupport Capability = "support.act"
	// CapImpersonate is looking at the platform as somebody else.
	//
	// Deliberately not held by `operator`. That role's remit is the platform —
	// tenants, settings, deployments — and §2.2 draws the line at seeing what
	// an organisation keeps. Support has to cross it to do their job, with a
	// reason, a time limit and a banner; operators do not.
	CapImpersonate Capability = "user.impersonate"

	// CapSettingsWrite changes how the platform behaves — the access mode, the
	// timeouts, the maintenance switch.
	CapSettingsWrite Capability = "settings.write"
	// CapFlagsWrite turns features on and off, including the kill switches.
	CapFlagsWrite Capability = "flags.write"
	// CapAnnounce tells everybody something.
	CapAnnounce Capability = "announce"

	// CapDeploy asks GitHub to run the deployment workflow.
	//
	// Superadmin only, and with a second factor. Not because the action is
	// dangerous in itself — the workflow is the same one a merge runs — but
	// because a console that can deploy is a console that can put any tag into
	// production, and that is the whole supply chain in one button.
	CapDeploy Capability = "deploy.trigger"
)

// capabilities is the whole authorization model, in one readable place.
//
// Every question of the form "may this role do that" is answered by reading
// this table, and adding a capability is editing it — never an `if role ==
// "superadmin"` inside a handler, which is where privilege bugs live and where
// nobody can see the whole picture at once.
//
// The shape follows §2.2 of the plan. Three things are worth saying out loud,
// because they are the ones a reader would otherwise assume wrong:
//
//   - The four roles are not a ladder. `operator` can create an organisation
//     and `support` cannot; `support` can look inside one and `operator`
//     cannot. Neither is "more" than the other.
//   - `auditor` reads everything and can do nothing. That is the point of it:
//     somebody who checks the platform without being able to change it.
//   - Deletion is a superadmin's request and a *different* superadmin's
//     approval. One person holding both roles is not two people, so the check
//     is on the identity rather than on the capability (see approvals.go).
var capabilities = map[Role]map[Capability]bool{
	RoleSuperadmin: {
		CapTenantRead: true, CapAuditRead: true, CapOperatorRead: true,
		CapTenantCreate: true, CapTenantSuspend: true, CapTenantDelete: true,
		CapApprove: true, CapQuotaWrite: true, CapSupport: true, CapImpersonate: true,
		CapSettingsWrite: true, CapFlagsWrite: true, CapAnnounce: true,
		CapDeploy: true,
	},
	RoleOperator: {
		CapTenantRead: true, CapAuditRead: true, CapOperatorRead: true,
		CapTenantCreate: true, CapTenantSuspend: true, CapQuotaWrite: true,
		CapSupport: true,
		// The day-to-day configuration work is this role's remit — including
		// the kill switches, because the person on call at two in the morning
		// is an operator and not a superadmin.
		CapSettingsWrite: true, CapFlagsWrite: true, CapAnnounce: true,
	},
	RoleSupport: {
		CapTenantRead: true, CapAuditRead: true,
		CapSupport: true, CapImpersonate: true,
	},
	RoleAuditor: {
		CapTenantRead: true, CapAuditRead: true, CapOperatorRead: true,
	},
}

// Can reports whether this role holds a capability. An unknown role holds
// none — a row whose `role` column was written by something that did not know
// the list ends up able to sign in and do nothing, rather than able to do
// everything.
func (r Role) Can(c Capability) bool { return capabilities[r][c] }

// Valid reports whether r is one of the four.
func (r Role) Valid() bool { _, known := capabilities[r]; return known }

// Operator is an account, as the console needs to know it.
type Operator struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  Role   `json:"role"`
}
