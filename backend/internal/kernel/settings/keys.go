package settings

// Every setting this platform has, in one file.
//
// New keys go here rather than beside the code that reads them, because the
// question an operator asks — "what can I change?" — should have one answer,
// and because the review that matters for a setting is "should this be
// editable at all", which is easier to hold when the whole list is on one
// screen.
//
// The first four are the migration the plan asks for: values that were env-only
// and are safely dynamic. The env variable stays the fallback in every case, so
// a deployment that has never opened the console is unaffected.

const (
	// AccessMode decides whether strangers may become users of this platform.
	//
	// The one that changes the most about how the platform behaves, and the
	// reason it is the first setting rather than an afterthought: a platform
	// that provisions an account for anybody who can authenticate somewhere
	// else is a very different thing from one where somebody has to be
	// invited, and until now that difference was decided by which environment
	// variables happened to be set.
	AccessMode = "platform.access_mode"

	// SessionIdleTimeout is how long a session may go unused.
	SessionIdleTimeout = "session.idle_timeout"

	// CatalogSyncInterval is how often the app catalogue is refetched.
	CatalogSyncInterval = "catalog.sync_interval"

	// AIModel is the Gemini model the copilot asks.
	AIModel = "ai.model"

	// Maintenance closes the platform to writing.
	Maintenance = "platform.maintenance"

	// MaintenanceMessage is what people are told while it is closed.
	MaintenanceMessage = "platform.maintenance_message"

	// AITTSModel is the Gemini model the voice features ask. Beside AIModel,
	// and separate from it, because the two move for different reasons: the
	// chat model changes when a better one ships, the voice model when the
	// preview one it names is retired.
	AITTSModel = "ai.tts_model"

	// BrandName is what this deployment calls itself.
	//
	// A name, not an address: what it changes is the word a citizen reads on
	// the eID approval prompt, which is the one place this API puts a product
	// name in front of a human. The browser app reads its own copy
	// (frontend/lib/brand.ts) — the two are halves of one setting, and a
	// deployment that renames only one has a sign-in screen and an eID prompt
	// that disagree about which product somebody is standing in front of.
	BrandName = "brand.name"

	// ConsoleTOTPIssuer is the name an authenticator app shows beside the
	// console's code. Empty means the brand followed by "Control Plane".
	//
	// Separate from BrandName because the two answer different people: the
	// brand is what a citizen reads, while this is what an operator scans
	// a phone for — and an operator with several consoles tells them apart by
	// address rather than by product name.
	ConsoleTOTPIssuer = "console.totp_issuer"

	// The addresses of this deployment's own monitoring stack, which the
	// console's front page links to and reads its numbers from.
	//
	// Addresses, which the note at the top of registry.go says do not belong
	// here. That line is about credentials, and these three carry none: they
	// are where an operator's own Prometheus answers, and getting one wrong
	// makes a panel on one console screen say "not configured". Kept out of
	// the credentials store for the same reason — nothing here is secret, and
	// a value nobody may read back is a value nobody can check.
	PrometheusURL   = "observability.prometheus_url"
	AlertmanagerURL = "observability.alertmanager_url"
	GrafanaURL      = "observability.grafana_url"
)

// The access modes.
const (
	// AccessPublic: anybody who can prove an identity gets an account.
	AccessPublic = "public"
	// AccessPrivate: only people somebody has already registered.
	AccessPrivate = "private"
)

func init() {
	Register(Spec{
		Key:     AccessMode,
		Kind:    KindEnum,
		Options: []string{AccessPrivate, AccessPublic},
		// Private is the safe default and therefore the default.
		//
		// It is also a behaviour change for a deployment that relied on
		// just-in-time provisioning through eID or a federated provider, and
		// that is the right way round: a platform that quietly starts creating
		// accounts for strangers because nobody set a variable is the failure
		// this setting exists to prevent. The console says which mode it is
		// in, and switching to public is one field and a reason.
		Default: AccessPrivate,
		// The env fallback every other setting here has, and the one this
		// setting needed most: a distribution that is public by design —
		// sso.gerege.mn — otherwise has to be switched by hand in the console
		// after every fresh deployment, and until somebody does, eID sign-in
		// answers 403 to everybody. The console still wins over it.
		Env: "PLATFORM_ACCESS_MODE",
		Description: "Хаалттай (private) горимд зөвхөн урьдчилан бүртгэгдсэн хүн нэвтэрнэ: " +
			"eID, ДАН, SSO-гоор баталгаажсан ч бүртгэлгүй хүнд шинэ данс үүсэхгүй. " +
			"Нээлттэй (public) горимд анх удаа нэвтэрсэн хүнд данс автоматаар үүснэ.",
	})

	Register(Spec{
		Key:         SessionIdleTimeout,
		Kind:        KindDuration,
		Default:     "90m",
		Env:         "SESSION_IDLE_TIMEOUT",
		Description: "Session хэдэн хугацаанд ашиглагдаагүй бол дуусах вэ. 0 нь хугацаагүй.",
	})

	Register(Spec{
		Key:         CatalogSyncInterval,
		Kind:        KindDuration,
		Default:     "1h",
		Env:         "CATALOG_SYNC_INTERVAL",
		Description: "Апп сторын каталогийг хэдэн хугацаа тутам registry-ээс шинэчлэх вэ.",
	})

	Register(Spec{
		Key:     AIModel,
		Kind:    KindString,
		Default: "",
		Env:     "GEMINI_MODEL",
		Description: "Copilot ямар Gemini загвар асуух вэ. Хоосон бол клиентийн " +
			"өөрийн анхдагч (тухайн хувилбарын flash загвар).",
	})

	Register(Spec{
		Key:     Maintenance,
		Kind:    KindBool,
		Default: "false",
		Description: "Платформ даяар зөвхөн унших горим. Хэрэглэгчид нэвтэрч, харж чадах " +
			"ч юу ч өөрчилж чадахгүй. Операторын консол үүнд хамаарахгүй.",
	})

	Register(Spec{
		Key:         MaintenanceMessage,
		Kind:        KindString,
		Default:     "",
		Description: "Засварын горимд хэрэглэгчдэд харагдах мессеж. Хоосон бол ерөнхий текст.",
	})
	Register(Spec{
		Key:     AITTSModel,
		Kind:    KindString,
		Default: "gemini-2.5-flash-preview-tts",
		Env:     "GEMINI_TTS_MODEL",
		Description: "Дуут боломжуудын асуудаг Gemini загвар. Preview загвар хаагдвал 404 " +
			"буцаадаг бөгөөд түүнийг deploy хийхгүйгээр энд солино.",
	})
	Register(Spec{
		Key:     BrandName,
		Kind:    KindString,
		Default: "Gerege Nexus",
		Env:     "BRAND_NAME",
		Description: "Энэ суулгац өөрийгөө юу гэж нэрлэх вэ. API нь хүний өмнө нэр тавьдаг " +
			"хоёр газарт үйлчилнэ: eID-гийн зөвшөөрлийн цонх, мөн иргэний бүртгэл холбох амжилтгүй " +
			"болсон үеийн мессеж. Хөтчийн апп өөрийн хуулбарыг уншина (BRAND_NAME).",
	})
	Register(Spec{
		Key:  ConsoleTOTPIssuer,
		Kind: KindString,
		Env:  "CONTROL_PLANE_TOTP_ISSUER",
		Description: "Консолын хоёр дахь хүчин зүйлийг authenticator апп дээр ямар нэрээр харуулах вэ " +
			"(жишээ нь admin.example.mn). Хоосон бол «<брэнд> Control Plane». Зөвхөн ШИНЭЭР " +
			"бүртгүүлэх операторт хамаарна — аль хэдийн бүртгэгдсэн утасны нэрийг апп дотор засна.",
	})
	Register(Spec{
		Key:  PrometheusURL,
		Kind: KindString,
		Env:  "PROMETHEUS_URL",
		Description: "Консол энэ суулгацын хэмжүүрийг хаанаас уншихыг заана. Хоосон бол " +
			"хэмжүүрийн хэсэг «тохируулаагүй» гэж харагдана.",
	})
	Register(Spec{
		Key:         AlertmanagerURL,
		Kind:        KindString,
		Env:         "ALERTMANAGER_URL",
		Description: "Консол идэвхтэй дохиог хаанаас уншихыг заана. Хоосон бол тэр хэсэг унтарна.",
	})
	Register(Spec{
		Key:         GrafanaURL,
		Kind:        KindString,
		Env:         "GRAFANA_URL",
		Description: "Консолын хяналтын самбар руу чиглэсэн шууд холбоос хаашаа заахыг тодорхойлно. Хоосон бол холбоос гарахгүй.",
	})
}
