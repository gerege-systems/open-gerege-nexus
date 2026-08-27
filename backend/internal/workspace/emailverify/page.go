/*
 * Gerege Nexus
 * Copyright (c) 2026 Gerege Systems Development Team, Gerege Nomadica Foundation
 * Distributed under the Apache 2.0 License.
 *
 * What the recipient sees when nobody named a page to send them to.
 */

package emailverify

import "strings"

// A caller may pass no redirect_url, and somebody still clicked a link. What
// answers that click is a person in a browser, not the integration — so it is a
// page rather than a JSON body, in the language their browser asked for.
type resultWording struct {
	verifiedTitle, verifiedBody string
	spentTitle, spentBody       string
}

var resultWordings = map[string]resultWording{
	"mn": {
		verifiedTitle: "И-мэйл хаяг баталгаажлаа",
		verifiedBody:  "Танд баярлалаа. Энэ цонхыг хааж болно.",
		spentTitle:    "Холбоос хүчингүй болсон",
		spentBody:     "Энэ холбоосыг аль хэдийн ашигласан эсвэл хугацаа нь дууссан байна. Шинэ холбоос хүсэлт гаргана уу.",
	},
	"en": {
		verifiedTitle: "Email address confirmed",
		verifiedBody:  "Thank you. You can close this window.",
		spentTitle:    "This link no longer works",
		spentBody:     "It has already been used or it has expired. Ask for a new one.",
	},
	"ar": {
		verifiedTitle: "تم تأكيد عنوان البريد الإلكتروني",
		verifiedBody:  "شكرًا لك. يمكنك إغلاق هذه النافذة.",
		spentTitle:    "لم يعد هذا الرابط صالحًا",
		spentBody:     "لقد تم استخدامه بالفعل أو انتهت صلاحيته. اطلب رابطًا جديدًا.",
	},
	"zh": {
		verifiedTitle: "电子邮件地址已确认",
		verifiedBody:  "谢谢，您可以关闭此窗口。",
		spentTitle:    "此链接已失效",
		spentBody:     "该链接已被使用或已过期，请重新申请。",
	},
	"fr": {
		verifiedTitle: "Adresse e-mail confirmée",
		verifiedBody:  "Merci. Vous pouvez fermer cette fenêtre.",
		spentTitle:    "Ce lien ne fonctionne plus",
		spentBody:     "Il a déjà été utilisé ou il a expiré. Demandez-en un nouveau.",
	},
	"ru": {
		verifiedTitle: "Адрес электронной почты подтверждён",
		verifiedBody:  "Спасибо. Это окно можно закрыть.",
		spentTitle:    "Ссылка больше не действует",
		spentBody:     "Она уже использована или срок её действия истёк. Запросите новую.",
	},
	"es": {
		verifiedTitle: "Dirección de correo confirmada",
		verifiedBody:  "Gracias. Puede cerrar esta ventana.",
		spentTitle:    "Este enlace ya no funciona",
		spentBody:     "Ya se ha usado o ha caducado. Solicite uno nuevo.",
	},
}

// ResultPage returns the title and body for the page shown after a click.
func ResultPage(locale string, confirmed bool) (title, body string) {
	text, ok := resultWordings[strings.ToLower(strings.TrimSpace(locale))]
	if !ok {
		text = resultWordings["mn"]
	}
	if confirmed {
		return text.verifiedTitle, text.verifiedBody
	}
	return text.spentTitle, text.spentBody
}
