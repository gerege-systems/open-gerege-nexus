// Package catalog is the app-store contract: what a manifest is, what a
// catalogue entry is, and what makes either one valid.
//
// It is separate from `pkg/nexus` because it has a different audience. The SDK
// is compiled against by whoever writes a Go module; this is compiled against
// by whoever runs a registry, publishes to one, or reads a catalogue — and by
// the third-party publisher who never writes Go at all and only needs the JSON
// shape these types define.
//
// `docs/ECOSYSTEM_GIT_STRATEGY.md` §2.4 names this one of three contracts that
// outlive the core's release cycle. It is versioned with this module today and
// may become a repository of its own once more than one team publishes against
// it; keeping it in `pkg/` rather than `internal/` is what makes that a move
// rather than a rewrite.
//
// What is here is the schema and its validation. Anything that knows where a
// catalogue lives on a particular deployment — the bundled file, the disk
// cache, the signed fetch from a registry — is that deployment's business and
// stays in internal/operator/appcatalog.
package catalog

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// App types. An app is a Go module compiled into this binary unless it says
// otherwise, which is why the empty string means TypeModule: every manifest
// written before external apps existed is a module manifest.
const (
	TypeModule   = "module"
	TypeExternal = "external"
)

type Manifest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`
	// Type is "module" (default) or "external". An external app is a service
	// that runs somewhere else entirely: the platform holds its registration,
	// its permissions and its menu entry, and hands its users over by OIDC.
	// Nothing of it is compiled in.
	Type         string                       `json:"type,omitempty"`
	External     *ExternalSpec                `json:"external,omitempty"`
	Platform     string                       `json:"platform"`
	Dependencies []nexus.Dependency           `json:"dependencies"`
	Permissions  []nexus.PermissionDefinition `json:"permissions"`
	Menus        []nexus.MenuDefinition       `json:"menus"`

	// Everything below is manifest v2.1: who stands behind this app and what
	// changed in this version of it. Every field is optional and every one is
	// omitempty, so a manifest written before any of this existed marshals to
	// exactly the bytes it did before — which is what keeps the signed
	// catalogue byte-reproducible across the two repositories that build it.
	Publisher   string   `json:"publisher,omitempty"`
	Authors     []Person `json:"authors,omitempty"`
	Maintainers []Person `json:"maintainers,omitempty"`
	Repository  string   `json:"repository,omitempty"`
	Homepage    string   `json:"homepage,omitempty"`
	// License is an SPDX identifier. It is shape-checked rather than looked up:
	// a closed list would have to be maintained here and would reject a licence
	// that is perfectly real and merely newer than this build.
	License string `json:"license,omitempty"`
	// ReleaseNotes is this version's chronicle entry, copied in as the
	// catalogue is assembled. It is not authored here — see chronicle.go.
	ReleaseNotes *ReleaseNote `json:"release_notes,omitempty"`

	// Order is where this app sits in the shell's app rail. Lower comes first;
	// an app that does not say goes after the ones that do, in id order.
	//
	// Optional, and optional on purpose: most apps have no opinion, and a
	// deployment with four apps does not need four numbers. What it replaces is
	// a list of app ids written into frontend/components/Layout.tsx, which no
	// app outside this repository could add itself to.
	Order int `json:"order,omitempty"`
	// Chrome marks an app the shell presents as part of itself rather than as a
	// tile in the app rail: its screens appear in the platform's own menu group.
	//
	// One app uses it — the organisation — and the reason is that it is not one
	// of the things installed into an organisation, it *is* the organisation you
	// are signed in to. Drawn as a tile, clicking one of its screens selected it
	// as the current app and replaced the whole sidebar, so the menu you clicked
	// in disappeared under you.
	//
	// Only a module may claim it. An external app runs somewhere else and is
	// reached by handing the user over; presenting it as part of this shell
	// would be a claim the platform cannot keep.
	Chrome bool `json:"chrome,omitempty"`

	// Visibility is who may be offered this app: every platform, or only the
	// ones the registry has been told to offer it to.
	//
	// It is on the manifest rather than only on the catalogue entry because the
	// manifest is the document that travels. A catalogue entry is assembled
	// where the catalogue is assembled; the manifest is what a publisher
	// submits (cmd/publish-catalog sends exactly this struct), what the
	// registry stores, and what the signature covers. A visibility that lived
	// only in the entry would be a declaration the publisher made and nobody
	// downstream ever received.
	//
	// Empty means public, which is why this is omitempty like everything else
	// in v2.1: a manifest written before private apps existed marshals to the
	// bytes it always did, and the signed catalogue stays byte-reproducible
	// across the two repositories that build it.
	//
	// **Where this is enforced.** Not here. A private app is kept from a
	// platform by not being in the catalogue that platform is served — the
	// registry decides, per deployment, using the credential the deployment
	// sends (APP_CATALOG_TOKEN; see appcatalog.Config). This field is the
	// publisher's declaration and the reason the registry has something to act
	// on; a platform that has received a private app has already been
	// authorised to have it, and what it does with this field is label it.
	//
	// Anything else would be self-enforcement: shipping every platform the same
	// document and asking each to hide what it should not see. That leaks the
	// names of private apps to everyone holding the catalogue, and asks the
	// party with the motive to look to be the party that decides.
	Visibility string `json:"visibility,omitempty"`
}

// The two visibilities an app can be published with.
//
// An unknown third value is refused rather than treated as either. Reading it
// as public would turn a typo — "Private", "internal", "restricted" — into a
// silent publication, and reading it as private would hide an app for a reason
// nobody could see. Both are the kind of failure that is only noticed by the
// person it should not have reached.
const (
	VisibilityPublic  = "public"
	VisibilityPrivate = "private"
)

// IsPrivate reports whether this app is offered only to the platforms the
// registry names.
func (m Manifest) IsPrivate() bool { return m.Visibility == VisibilityPrivate }

// IsPrivate reports whether this catalogue entry is a private app.
//
// A private declaration in either half wins. The manifest is the document that
// travels and the entry is assembled locally, so they should agree — and when
// they do not, the one that hides the app is the safe reading: an app kept
// private by mistake is a support question, and an app published by mistake is
// not recallable.
func (a CatalogApp) IsPrivate() bool {
	return a.Visibility == VisibilityPrivate || a.Manifest.IsPrivate()
}

// ExternalSpec describes how to reach a third-party platform and how it signs
// its users in.
type ExternalSpec struct {
	// LaunchURL is where a user is sent. It is the third party's own entry
	// point, typically the one that starts the OIDC dance back at this issuer.
	LaunchURL string `json:"launch_url"`
	// SSOClientID ties the app to an OAuth2 client registered here. It is what
	// makes "has this tenant installed the app" answerable at the authorization
	// endpoint — see the install gate in ssoprovider.
	SSOClientID string   `json:"sso_client_id,omitempty"`
	Scopes      []string `json:"scopes,omitempty"`
	// Embed is "new_tab" (default) or "iframe". new_tab is the default because
	// this platform sends X-Frame-Options: DENY and a Content-Security-Policy
	// to match: framing somebody else's product inside this one is a decision
	// both sides have to make, not a default.
	Embed string `json:"embed,omitempty"`
	// HealthURL is an address an operator can check. Nothing polls it yet.
	HealthURL string `json:"health_url,omitempty"`
}

// IsExternal reports whether this app runs outside the platform binary.
func (m Manifest) IsExternal() bool { return m.Type == TypeExternal }

type CatalogApp struct {
	ID          string   `json:"id"`
	Slug        string   `json:"slug"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	IconURL     string   `json:"icon_url"`
	Category    string   `json:"category"`
	Visibility  string   `json:"visibility"`
	Version     string   `json:"version"`
	Manifest    Manifest `json:"manifest"`

	// Translations holds per-locale overrides keyed by ISO 639-1 code. The
	// store API resolves them before responding, so clients never have to
	// translate catalog content themselves.
	Translations map[string]CatalogAppText `json:"translations,omitempty"`
}

// CatalogAppText is the translatable part of a catalog entry.
type CatalogAppText struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

// Localized returns a copy with any translation for the locale applied. Fields
// missing from the translation keep their default value.
func (a CatalogApp) Localized(locale string) CatalogApp {
	text, ok := a.Translations[locale]
	if !ok {
		return a
	}
	if text.Name != "" {
		a.Name = text.Name
	}
	if text.Description != "" {
		a.Description = text.Description
	}
	if text.Category != "" {
		a.Category = text.Category
	}
	return a
}

// ValidateManifest validates semver and manifest rules.
func ValidateManifest(m Manifest, platformVersion string) error {
	if m.ID == "" || m.Name == "" || m.Version == "" {
		return fmt.Errorf("invalid manifest: id, name, and version are required")
	}

	_, err := semver.NewVersion(m.Version)
	if err != nil {
		return fmt.Errorf("invalid app version semver %q: %w", m.Version, err)
	}

	if m.Platform != "" && platformVersion != "" {
		constraint, err := semver.NewConstraint(m.Platform)
		if err != nil {
			return fmt.Errorf("invalid platform constraint %q: %w", m.Platform, err)
		}
		platVer, err := semver.NewVersion(platformVersion)
		if err != nil {
			return fmt.Errorf("invalid platform version %q: %w", platformVersion, err)
		}
		if !constraint.Check(platVer) {
			return fmt.Errorf("app %s version %s requires platform %s, current is %s", m.ID, m.Version, m.Platform, platformVersion)
		}
	}

	switch m.Visibility {
	case "", VisibilityPublic, VisibilityPrivate:
	default:
		return fmt.Errorf("app %s has an unknown visibility %q; expected %q or %q",
			m.ID, m.Visibility, VisibilityPublic, VisibilityPrivate)
	}

	if m.Order < 0 {
		return fmt.Errorf("app %s declares a negative rail order %d", m.ID, m.Order)
	}

	// Held to the same rule as a compiled module's. A manifest arrives from a
	// registry and a module is compiled here; neither is a reason to check one
	// and not the other, and this is a statement about who may do what.
	for _, perm := range m.Permissions {
		if err := perm.Validate(); err != nil {
			return fmt.Errorf("app %s: %w", m.ID, err)
		}
	}

	switch m.Type {
	case "", TypeModule:
		if m.External != nil {
			return fmt.Errorf("app %s is a module but carries an external section", m.ID)
		}
	case TypeExternal:
		if m.Chrome {
			return fmt.Errorf("external app %s cannot be shell chrome; it runs somewhere else", m.ID)
		}
		if m.External == nil || m.External.LaunchURL == "" {
			return fmt.Errorf("external app %s must declare external.launch_url", m.ID)
		}
		// Absolute and HTTPS, checked here rather than at the point of use.
		// This URL is put in front of a user as a link this platform vouches
		// for: a relative one would resolve against the platform's own origin
		// and a plain-HTTP one would carry a signed-in person out of TLS on the
		// way to a service that is about to receive their identity.
		launch, err := url.Parse(m.External.LaunchURL)
		if err != nil {
			return fmt.Errorf("external app %s has an unparseable launch_url: %w", m.ID, err)
		}
		if launch.Scheme != "https" || launch.Host == "" {
			return fmt.Errorf("external app %s must have an absolute https launch_url, got %q",
				m.ID, m.External.LaunchURL)
		}
		if embed := m.External.Embed; embed != "" && embed != "new_tab" && embed != "iframe" {
			return fmt.Errorf("external app %s has an unknown embed mode %q", m.ID, embed)
		}
	default:
		return fmt.Errorf("app %s has an unknown type %q", m.ID, m.Type)
	}

	if err := validateProvenance(m); err != nil {
		return err
	}

	for _, dep := range m.Dependencies {
		if dep.ID == "" {
			return fmt.Errorf("dependency ID cannot be empty in app %s", m.ID)
		}
		if dep.VersionConstraint != "" {
			if _, err := semver.NewConstraint(dep.VersionConstraint); err != nil {
				return fmt.Errorf("invalid dependency constraint %q for dep %s in app %s: %w", dep.VersionConstraint, dep.ID, m.ID, err)
			}
		}
	}
	return nil
}

// spdxLicense is the shape of an SPDX identifier, not a list of them: letters,
// digits, dots and dashes, optionally joined by WITH/OR/AND. It catches a
// sentence typed into the field and lets "Apache-2.0", "MIT" and
// "GPL-3.0-or-later WITH Classpath-exception-2.0" through alike.
var spdxLicense = regexp.MustCompile(`^[A-Za-z0-9.+-]+( (WITH|OR|AND) [A-Za-z0-9.+-]+)*$`)

// validateProvenance checks the manifest v2.1 fields. Each is optional; each
// that is present has to mean something.
//
// The rule worth stating is the one about people: an entry with no name is not
// a weaker claim about who wrote the app, it is an empty line rendered in a
// storefront credit list. An app may name nobody. It may not name a blank.
func validateProvenance(m Manifest) error {
	for _, group := range [...]struct {
		field  string
		people []Person
	}{{"authors", m.Authors}, {"maintainers", m.Maintainers}} {
		for i, person := range group.people {
			if strings.TrimSpace(person.Name) == "" {
				return fmt.Errorf("app %s: %s[%d] has no name", m.ID, group.field, i)
			}
		}
	}

	for _, link := range [...]struct {
		field string
		raw   string
	}{{"repository", m.Repository}, {"homepage", m.Homepage}} {
		if link.raw == "" {
			continue
		}
		// Same reasoning as external.launch_url: these are put in front of a
		// user as links this platform vouches for.
		parsed, err := url.Parse(link.raw)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
			return fmt.Errorf("app %s: %s must be an absolute https URL, got %q",
				m.ID, link.field, link.raw)
		}
	}

	if m.License != "" && !spdxLicense.MatchString(m.License) {
		return fmt.Errorf("app %s: license %q is not an SPDX identifier", m.ID, m.License)
	}

	if m.ReleaseNotes != nil {
		if err := validateNote("app "+m.ID+" release_notes", *m.ReleaseNotes); err != nil {
			return err
		}
		// The note travels inside a manifest that already carries the version,
		// so a second copy is a second thing that can disagree with it.
		if v := m.ReleaseNotes.Version; v != "" && v != m.Version {
			return fmt.Errorf("app %s: release_notes are for version %q but the manifest declares %q",
				m.ID, v, m.Version)
		}
	}
	return nil
}

// IsNewerVersion reports whether candidate is a later release than installed.
//
// It is the one place the store decides that an update exists, so both the API
// that offers the button and the installer that refuses a pointless upgrade
// answer the same question the same way. Semver, not string comparison: "1.10.0"
// sorts before "1.9.0" as text and after it as a version.
//
// A version either side cannot parse falls back to "different means newer".
// Manifest versions are validated as semver on the way in, so this only covers
// a catalogue that reached this instance by some other route.
func IsNewerVersion(candidate, installed string) bool {
	if candidate == "" {
		return false
	}
	newer, candidateErr := semver.NewVersion(candidate)
	held, installedErr := semver.NewVersion(installed)
	if candidateErr != nil || installedErr != nil {
		return candidate != installed
	}
	return newer.GreaterThan(held)
}
