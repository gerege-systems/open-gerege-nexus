/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 */

package credentials

// The credentials this platform holds.
//
// Three, and the list is short on purpose. A credential belongs here when the
// console can actually change what the platform does with it — which means
// something in this process reads it through Get on the path that uses it. A
// name declared here and read from the environment somewhere else would be a
// screen that lies: the operator sets a value, the badge says "database", and
// the integration goes on using the variable.
//
// So the rule for adding one is not "is it a secret" but "did the call site
// move". SSO_DEFAULT_CLIENT_SECRET and the eID relying-party secrets are
// deliberately absent: both are read while the process boots, and a value
// changed after that would not be honoured until a restart — which is the
// screen that lies, wearing a different hat.
const (
	// GeminiAPIKey is what the copilot, the voice features and the translation
	// helper authenticate with.
	// #nosec G101 -- the name of a credential, not one: the value is sealed in
	// operator.platform_credentials or read from GEMINI_API_KEY.
	GeminiAPIKey = "ai.gemini_api_key"
	// CoreAPIToken is what the first-run wizard and the organisation screen
	// search the Gerege Core register with.
	// #nosec G101 -- the name of a credential, not one: see GeminiAPIKey above.
	CoreAPIToken = "core.api_token"
	// ReportSMTPURL is where scheduled reports are posted. A URL rather than a
	// key, and here rather than in settings for the ordinary reason: it
	// carries the mailbox password in its userinfo.
	ReportSMTPURL = "reports.smtp_url"
)

func init() {
	Register(Spec{
		Name:        GeminiAPIKey,
		Env:         "GEMINI_API_KEY",
		Description: "Google AI Studio-гийн түлхүүр. Байхгүй бол Copilot, дуу, орчуулга унтарна.",
		Docs:        "https://aistudio.google.com/apikey",
	})
	Register(Spec{
		Name: CoreAPIToken,
		Env:  "GEREGE_CORE_TOKEN",
		Description: "Gerege Core бүртгэлийн токен. Байхгүй бол байгууллага, админыг " +
			"регистрээр хайхын оронд гараар бөглөнө.",
	})
	Register(Spec{
		Name: ReportSMTPURL,
		Env:  "REPORT_SMTP_URL",
		Description: "Товлосон тайлан илгээх smtps://хэрэглэгч:нууц@хост:порт. Байхгүй бол " +
			"тайлан бэлтгэгдэнэ, илгээгдэхгүй.",
	})
}
