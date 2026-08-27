/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * E-signature reports: what was signed, on which rail, by whom.
 */

package signing

import (
	"context"
	"time"

	"github.com/gerege-systems/open-gerege-nexus/backend/pkg/nexus"
)

// AppID is the app these reports belong to.
//
// The documents app, since the merge — the reports list is filtered by which
// apps a tenant has installed, and a report claiming an app nobody can install
// is a report nobody can reach.
const AppID = "io.gerege.nexus.documents"

func registerReports() {
	nexus.RegisterReport(signaturesByRail{})
	nexus.RegisterReport(signersActivity{})
}

func periodParam(window time.Duration) nexus.ParamSpec {
	return nexus.ParamSpec{
		Key:  "period",
		Kind: nexus.ParamDateRange,
		Titles: map[string]string{
			"mn": "Хугацаа", "en": "Period", "ru": "Период",
			"zh": "期间", "fr": "Période", "es": "Periodo", "ar": "الفترة",
		},
		DefaultWindow: window,
	}
}

// ------------------------------------------------------------ by rail

type signaturesByRail struct{}

func (signaturesByRail) Key() string { return "documents.signatures_by_rail" }
func (signaturesByRail) App() string { return AppID }

func (signaturesByRail) Titles() map[string]string {
	return map[string]string{
		"mn": "Гарын үсгийн статистик (rail-аар)",
		"en": "Signatures by rail",
		"ru": "Подписи по каналам",
		"zh": "按签署方式统计",
		"fr": "Signatures par canal",
		"es": "Firmas por canal",
		"ar": "التوقيعات حسب القناة",
	}
}

func (signaturesByRail) Params() []nexus.ParamSpec {
	return []nexus.ParamSpec{periodParam(365 * 24 * time.Hour)}
}

func (signaturesByRail) Columns() []nexus.ColumnSpec {
	return []nexus.ColumnSpec{
		{Key: "month", Kind: nexus.ColumnMonth, Chart: nexus.ChartCategory,
			Titles: map[string]string{"mn": "Сар", "en": "Month", "ru": "Месяц", "zh": "月份", "fr": "Mois", "es": "Mes", "ar": "الشهر"}},
		{Key: "rail", Kind: nexus.ColumnText,
			Titles: map[string]string{"mn": "Гарын үсгийн зам", "en": "Rail", "ru": "Канал", "zh": "签署方式", "fr": "Canal", "es": "Canal", "ar": "القناة"}},
		{Key: "qualified", Kind: nexus.ColumnText,
			Titles: map[string]string{"mn": "Баталгаажсан эсэх", "en": "Qualified", "ru": "Квалифицированная", "zh": "合格签名", "fr": "Qualifiée", "es": "Cualificada", "ar": "مؤهلة"}},
		{Key: "signed", Kind: nexus.ColumnNumber, Chart: nexus.ChartValue, Total: true,
			Titles: map[string]string{"mn": "Гарын үсэг зурсан", "en": "Signed", "ru": "Подписано", "zh": "已签署", "fr": "Signés", "es": "Firmados", "ar": "موقعة"}},
	}
}

// Run separates the rails and says which of them is qualified.
//
// That column is not decoration. Only the eID rail produces a qualified
// electronic signature in Mongolian law; the HSM rail predates it and does not.
// A report that counted both together would answer "how many documents were
// signed" and quietly not answer "how many were signed in a way that stands up",
// which is the question a compliance reader is actually asking.
func (signaturesByRail) Run(ctx context.Context, q nexus.Querier, p nexus.Params) (nexus.Result, error) {
	const query = `
		SELECT date_trunc('month', signed_at)::date AS month,
		       coalesce(provider, 'UNKNOWN')        AS rail,
		       count(*)                             AS signed
		  FROM workspace.esign_documents
		 WHERE tenant_id = $1
		   AND status = 'SIGNED'
		   AND deleted_at IS NULL
		   AND signed_at >= $2 AND signed_at <= $3
		 GROUP BY 1, 2
		 ORDER BY 1, 2`

	rows, err := q.Query(ctx, query,
		nexus.WorkspaceOf(ctx), p.Time("period_from"), p.Time("period_to"))
	if err != nil {
		return nexus.Result{}, err
	}

	locale := p.Locale()
	collected, err := nexus.Collect(rows, func() (map[string]any, error) {
		var month time.Time
		var rail string
		var signed int64
		if err := rows.Scan(&month, &rail, &signed); err != nil {
			return nil, err
		}
		return map[string]any{
			"month": month, "rail": rail,
			"qualified": qualifiedLabel(rail, locale),
			"signed":    signed,
		}, nil
	})
	if err != nil {
		return nexus.Result{}, err
	}
	return nexus.Result{Rows: collected}, nil
}

func qualifiedLabel(rail, locale string) string {
	qualified := rail == ProviderEID
	if locale == "en" {
		if qualified {
			return "Yes"
		}
		return "No"
	}
	if qualified {
		return "Тийм"
	}
	return "Үгүй"
}

// ------------------------------------------------------------- signers

type signersActivity struct{}

func (signersActivity) Key() string { return "documents.signer_activity" }
func (signersActivity) App() string { return AppID }

func (signersActivity) Titles() map[string]string {
	return map[string]string{
		"mn": "Гарын үсэг зурагчдын идэвх",
		"en": "Signer activity",
		"ru": "Активность подписантов",
		"zh": "签署人活动",
		"fr": "Activité des signataires",
		"es": "Actividad de firmantes",
		"ar": "نشاط الموقعين",
	}
}

func (signersActivity) Params() []nexus.ParamSpec {
	return []nexus.ParamSpec{periodParam(90 * 24 * time.Hour)}
}

func (signersActivity) Columns() []nexus.ColumnSpec {
	return []nexus.ColumnSpec{
		{Key: "signer", Kind: nexus.ColumnText, Chart: nexus.ChartCategory,
			Titles: map[string]string{"mn": "Гарын үсэг зурагч", "en": "Signer", "ru": "Подписант", "zh": "签署人", "fr": "Signataire", "es": "Firmante", "ar": "الموقّع"}},
		{Key: "signed", Kind: nexus.ColumnNumber, Chart: nexus.ChartValue, Total: true,
			Titles: map[string]string{"mn": "Баримтын тоо", "en": "Documents", "ru": "Документов", "zh": "文件数", "fr": "Documents", "es": "Documentos", "ar": "المستندات"}},
		{Key: "last_signed", Kind: nexus.ColumnDate,
			Titles: map[string]string{"mn": "Сүүлд зурсан", "en": "Last signed", "ru": "Последняя подпись", "zh": "最后签署", "fr": "Dernière signature", "es": "Última firma", "ar": "آخر توقيع"}},
	}
}

// Run groups by the signer's name and not by their registration number.
//
// The registration number is a national identifier. It is on the row, it has to
// be — it is what makes the signature attributable — but a report is a thing
// that gets exported to a spreadsheet, mailed to a list of addresses on a
// schedule, and left in a downloads folder. The name answers "who has been
// signing" without putting an identifier in all three places.
func (signersActivity) Run(ctx context.Context, q nexus.Querier, p nexus.Params) (nexus.Result, error) {
	const query = `
		SELECT coalesce(nullif(signer_name, ''), '—') AS signer,
		       count(*)                               AS signed,
		       max(signed_at)::date                   AS last_signed
		  FROM workspace.esign_documents
		 WHERE tenant_id = $1
		   AND status = 'SIGNED'
		   AND deleted_at IS NULL
		   AND signed_at >= $2 AND signed_at <= $3
		 GROUP BY 1
		 ORDER BY 2 DESC, 1`

	rows, err := q.Query(ctx, query,
		nexus.WorkspaceOf(ctx), p.Time("period_from"), p.Time("period_to"))
	if err != nil {
		return nexus.Result{}, err
	}

	collected, err := nexus.Collect(rows, func() (map[string]any, error) {
		var signer string
		var signed int64
		var last time.Time
		if err := rows.Scan(&signer, &signed, &last); err != nil {
			return nil, err
		}
		return map[string]any{"signer": signer, "signed": signed, "last_signed": last}, nil
	})
	if err != nil {
		return nexus.Result{}, err
	}
	return nexus.Result{Rows: collected}, nil
}
