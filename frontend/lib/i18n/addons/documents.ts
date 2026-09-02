/**
 * documents — Document routing, categories and e-signatures.
 */
export const documents = {
  "documents.view.title": { mn: "Цахим баримт ба тоон гарын үсэг", en: "Digital Documents & E-Signatures" },
  "documents.view.create_title": { mn: "Цахим баримт үүсгэх", en: "Create Digital Document" },
  "documents.view.subtitle": {
    mn: "Байгууллагын баримтын урсгал, тоон гарын үсэг ба батламжийн процесс.",
    en: "Enterprise document routing, digital signatures, and approval workflows.",
  },
  "documents.field.title_pattern_placeholder": {
    mn: "Хамтран ажиллах гэрээ {year}",
    en: "Partnership agreement {year}",
  },
  "documents.view.sign_title": { mn: "Баримтад гарын үсэг зурах", en: "Sign Document" },
  "documents.view.history_title": { mn: "Гарын үсгийн түүх", en: "Signature history" },
  "documents.view.approvals_hint": {
    mn: "Гарын үсэг хүлээж байгаа баримтууд — E-ID / ДАН-аар батлах эсвэл татгалзах.",
    en: "Documents awaiting a decision — approve with an E-ID / DAN signature, or reject.",
  },

  // Screen title for the queue. The sidebar entry itself is labelled by the
  // menu blueprint on the server; this is the heading the page draws.
  "documents.menu.approvals": { mn: "Батлах дараалал", en: "Approval queue" },

  "documents.field.title": { mn: "Баримтын гарчиг", en: "Document Title" },
  "documents.field.title_placeholder": { mn: "e.g. Хамтран ажиллах гэрээ 2026", en: "e.g. Partnership agreement 2026" },
  "documents.field.category": { mn: "Баримтын ангилал", en: "Document Category" },
  "documents.field.signature": { mn: "Тоон гарын үсэг (E-ID / ДАН)", en: "Digital Signature (E-ID / DAN)" },
  "documents.field.created": { mn: "Үүсгэсэн огноо", en: "Created Date" },
  "documents.field.signature_method": { mn: "Гарын үсгийн арга", en: "Signature Method" },
  "documents.field.reg_number": { mn: "Регистрийн дугаар", en: "Registration Number" },
  "documents.field.otp_code": { mn: "Нэг удаагийн нууц код (OTP)", en: "One-Time Code (OTP)" },
  "documents.field.verification_code": { mn: "Баталгаажуулах код", en: "Verification code" },
  "documents.field.signed_at": { mn: "Зурсан цаг", en: "Signed at" },
  "documents.field.certificate_serial": { mn: "Гэрчилгээний дугаар", en: "Certificate serial" },
  "documents.field.certificate_issuer": { mn: "Гэрчилгээ олгогч", en: "Certificate issuer" },
  "documents.field.approval_reference": { mn: "Батламжийн сурвалж", en: "Approval reference" },
  "documents.field.waiting_days": { mn: "Хүлээсэн хоног", en: "Days waiting" },

  "documents.stat.awaiting": { mn: "Гарын үсэг хүлээж буй", en: "Awaiting signature" },
  "documents.message.showing_some": {
    mn: "{total} баримтаас хамгийн шинэ {shown}-г харуулж байна.",
    en: "Showing the {shown} most recent of {total} documents.",
  },
  "documents.message.showing_some_oldest": {
    mn: "Хүлээж байгаа {total}-аас хамгийн урт хүлээсэн {shown}-г харуулж байна.",
    en: "Showing the {shown} longest-waiting of {total}.",
  },
  "documents.action.load_more": { mn: "Дараагийнхыг ачаалах", en: "Load more" },
  "documents.action.rename": { mn: "Гарчгийг засах", en: "Correct the title" },
  "documents.message.renamed": {
    mn: "Гарчгийг «{title}» болголоо.",
    en: "The title is now \"{title}\".",
  },
  "documents.message.rename_failed": { mn: "Гарчгийг засаж чадсангүй", en: "The title could not be corrected" },
  "documents.message.stale_rows": {
    mn: "Эдгээр мөр хоцрогдсон байж болно — жагсаалтыг шинэчилж чадсангүй.",
    en: "These rows may be out of date — the list could not be refreshed.",
  },
  "documents.action.retry": { mn: "Дахин оролдох", en: "Try again" },
  "documents.field.search_placeholder": { mn: "Гарчгаар хайх", en: "Search by title" },
  "documents.field.any_type": { mn: "Бүх төрөл", en: "Any type" },
  "documents.field.any_status": { mn: "Бүх төлөв", en: "Any status" },
  "documents.message.no_matches": {
    mn: "Хайлтад тохирох баримт олдсонгүй. Хайлт эсвэл шүүлтийг сольж үзээрэй.",
    en: "No documents match. Try a different search or filter.",
  },
  "documents.stat.oldest_days": { mn: "Хамгийн урт хүлээлт (хоног)", en: "Longest wait (days)" },

  "documents.category.legal_contract": { mn: "Гэрээ", en: "Legal Contract" },
  "documents.category.official_request": { mn: "Албан хүсэлт", en: "Official Request" },
  "documents.category.internal_approval": { mn: "Дотоод батламж", en: "Internal Approval" },

  "documents.state.pending_signature": { mn: "Гарын үсэг хүлээж буй", en: "Pending Signature" },
  "documents.state.draft": { mn: "Ноорог", en: "Draft" },
  "documents.state.pending": { mn: "Хүлээгдэж буй", en: "Pending" },
  "documents.state.approved": { mn: "Баталсан", en: "Approved" },
  "documents.state.rejected": { mn: "Татгалзсан", en: "Rejected" },
  "documents.state.awaiting_now": { mn: "Одоо хүлээж байна", en: "Awaiting now" },
  "documents.state.awaiting_later": { mn: "Дараа", en: "Later" },
  "documents.state.never_given": { mn: "Аваагүй", en: "Never given" },

  "documents.action.create": { mn: "Баримт үүсгэх", en: "Create Document" },
  "documents.action.sign": { mn: "Гарын үсэг зурах", en: "Sign" },
  "documents.action.reject": { mn: "Татгалзах", en: "Reject" },
  "documents.action.request_approval": { mn: "Батлах хүсэлт илгээх", en: "Send approval request" },
  "documents.action.view_history": { mn: "Гарын үсгийн түүх", en: "Signature history" },
  "documents.action.route": { mn: "Батлахад илгээх", en: "Send for approval" },

  "documents.message.loading": { mn: "Баримтуудыг ачаалж байна...", en: "Loading documents..." },
  "documents.message.empty": {
    mn: "Одоогоор баримт байхгүй. Баримт үүсгээд E-ID / ДАН гарын үсэгт илгээнэ үү.",
    en: "No documents yet. Create one and route it for an E-ID / DAN signature.",
  },
  "documents.message.signing": { mn: "Гарын үсэг зурж байна...", en: "Signing..." },
  "documents.message.no_pending": {
    mn: "Батлах хүлээж байгаа баримт байхгүй.",
    en: "Nothing is waiting for approval.",
  },
  "documents.message.sign_not_granted": {
    mn: "Танд гарын үсэг зурах эрх (documents.sign) байхгүй тул үйлдлүүд харагдахгүй.",
    en: "The actions are hidden because you do not hold the documents.sign permission.",
  },
  "documents.message.rejected_not_signed": {
    mn: "Татгалзсан — гарын үсэг зураагүй",
    en: "Rejected — not signed",
  },
  "documents.message.sign_success": {
    mn: "\"{title}\"-д {method}-ээр гарын үсэг зурлаа.",
    en: "\"{title}\" was successfully signed via {method}.",
  },
  "documents.message.sign_failed": { mn: "Гарын үсэг зурж чадсангүй", en: "Signature failed" },
  "documents.message.history_failed": {
    mn: "Гарын үсгийн түүхийг ачаалж чадсангүй",
    en: "Could not load the signature history",
  },
  "documents.message.signature_outside_chain": {
    mn: "хэлхээнээс гадуур",
    en: "outside the chain",
  },
  "documents.message.step_open_to_anyone": {
    mn: "хэн ч зурж болно",
    en: "open to any signer",
  },
  "documents.message.no_signatures": {
    mn: "Энэ баримтад гарын үсэг зураагүй байна.",
    en: "Nothing has been signed on this document yet.",
  },

  // The E-ID ceremony: the citizen approves on their own device, so the screen
  // explains what is happening on the other end of it.
  "documents.message.eid_method_hint": {
    mn: "Иргэний eID апп руу батлах хүсэлт илгээгдэнэ. Тэр өөрийн төхөөрөмж дээрээ баримтын нэрийг харж, өөрийн гэрчилгээгээр батална.",
    en: "An approval request is pushed to the citizen's eID app. They see the document's name on their own device and approve it with their own certificate.",
  },
  "documents.message.dan_method_hint": {
    mn: "ДАН нь батлах хүсэлт түлхдэггүй тул регистр ба нэг удаагийн код хэрэгтэй.",
    en: "DAN pushes no approval request, so it needs a registration number and a one-time code.",
  },
  "documents.message.verification_code_hint": {
    mn: "Иргэний төхөөрөмж дээр яг ижил код харагдах ёстой. Зөрвөл батлахгүй.",
    en: "The citizen's device must show exactly this code. If it differs, they should not approve.",
  },
  "documents.message.awaiting_approval": {
    mn: "{reg}-ийн батламжийг хүлээж байна...",
    en: "Waiting for {reg} to approve...",
  },
  "documents.message.approval_display_text": {
    mn: "Иргэнд харагдах текст",
    en: "What the citizen sees",
  },
  "documents.message.approval_refused": {
    mn: "Иргэн батлахаас татгалзсан — гарын үсэг зурагдаагүй.",
    en: "The citizen declined the request — nothing was signed.",
  },
  "documents.message.approval_expired": {
    mn: "Батлах хүсэлтийн хугацаа дууссан — гарын үсэг зурагдаагүй. Дахин илгээнэ үү.",
    en: "The approval request expired — nothing was signed. Send it again.",
  },
  "documents.message.reject_confirm": {
    mn: "\"{title}\"-г татгалзах уу? Үүнийг буцаах боломжгүй.",
    en: "Reject \"{title}\"? This cannot be undone.",
  },
  "documents.message.reject_success": { mn: "\"{title}\"-г татгалзлаа.", en: "\"{title}\" was rejected." },
  "documents.message.reject_failed": { mn: "Татгалзаж чадсангүй", en: "Reject failed" },
  "documents.message.create_failed": { mn: "Баримт үүсгэж чадсангүй", en: "Failed to create document" },
  "documents.message.create_success": {
    mn: "\"{title}\" үүсгэж, батлах дараалалд оруулав.",
    en: "\"{title}\" was created and sent for approval.",
  },
  "documents.message.route_success": {
    mn: "\"{title}\" батлах дараалалд орлоо.",
    en: "\"{title}\" was sent for approval.",
  },
  "documents.message.route_failed": { mn: "Батлахад илгээж чадсангүй", en: "Could not send it for approval" },
  "documents.message.load_failed": {
    mn: "Баримтуудыг ачаалж чадсангүй. Дахин оролдоно уу.",
    en: "The documents could not be loaded. Try again.",
  },
  "documents.message.step_needs_name": {
    mn: "Шат бүрд нэр шаардлагатай.",
    en: "Every step needs a name.",
  },
  "documents.message.signature_progress": {
    mn: "{required} гарын үсгээс {applied} зурагдсан",
    en: "{applied} of {required} signatures applied",
  },

  // Templates
  "documents.menu.templates": { mn: "Баримтын загвар", en: "Document templates" },
  "documents.view.templates_hint": {
    mn: "Баримт үүсгэхэд хэрэглэх бэлдэц: гарчгийн загвар ба ангилал.",
    en: "Presets a document is started from: a title pattern and a category.",
  },
  "documents.field.template_name": { mn: "Загварын нэр", en: "Template name" },
  "documents.field.title_pattern": { mn: "Гарчгийн загвар", en: "Title pattern" },
  "documents.action.add_template": { mn: "Загвар нэмэх", en: "Add template" },
  "documents.action.use_template": { mn: "Баримт үүсгэх", en: "Create document" },
  // The braces here are literal: this text teaches the syntax, so it is called
  // without vars and t() leaves them alone. Do not "fix" them into placeholders.
  "documents.message.title_pattern_hint": {
    mn: "Гарчигт {year}, {month}, {date} гэж бичвэл загварыг хэрэглэх үед орлуулагдана. Танихгүй хэсэг хөндөгдөхгүй.",
    en: "A title may hold {year}, {month} or {date}; they are filled in when the template is used. Anything else is left as written.",
  },
  "documents.message.no_templates": {
    mn: "Загвар байхгүй. Дээрээс нэгийг нэмнэ үү.",
    en: "No templates yet. Add one above.",
  },
  "documents.message.templates_failed": { mn: "Загваруудыг ачаалж чадсангүй", en: "Could not load templates" },
  "documents.message.template_saved": { mn: "Загвар хадгалагдлаа.", en: "Template saved." },
  "documents.message.template_active_hint": {
    mn: "Идэвхгүй загвараас баримт үүсгэхгүй, гэхдээ бүртгэл хадгалагдана.",
    en: "An inactive template produces no documents, but its record is kept.",
  },
  "documents.message.template_unsaved": {
    mn: "Хадгалаагүй засвар байна — эхлээд хадгална уу.",
    en: "There are unsaved changes — save them first.",
  },
  "documents.message.template_inactive": {
    mn: "Идэвхгүй загвар — эхлээд идэвхжүүлнэ үү.",
    en: "This template is inactive — activate it first.",
  },
  "documents.message.template_used": {
    mn: "\"{title}\" баримт үүсч, батлах дараалалд орлоо.",
    en: "\"{title}\" was created and is waiting for approval.",
  },
  "documents.message.template_delete_confirm": {
    mn: "\"{name}\" загварыг устгах уу?",
    en: "Delete the template \"{name}\"?",
  },
  "documents.message.template_deleted": { mn: "\"{name}\" загварыг устгалаа.", en: "Template \"{name}\" was deleted." },

  // Signature policies
  "documents.menu.signature_policies": { mn: "Гарын үсгийн бодлого", en: "Signature policies" },
  "documents.view.signature_policies_hint": {
    mn: "Баримтын төрөл тус бүрийг ямар сувгаар гарын үсэг зурж болохыг тогтооно.",
    en: "Which national channel may sign each document type, and who is allowed to.",
  },
  "documents.field.require_named_signer": { mn: "Зөвхөн нэрлэсэн хүн", en: "Named signer only" },
  "documents.state.policy_configured": { mn: "Тохируулсан", en: "Configured" },
  "documents.state.policy_default": { mn: "Үндсэн", en: "Default" },
  "documents.message.policies_failed": { mn: "Бодлогуудыг ачаалж чадсангүй", en: "Could not load policies" },
  "documents.message.policy_saved": { mn: "{type}-ийн бодлого хадгалагдлаа.", en: "The {type} policy was saved." },
  "documents.message.policy_named_signer_hint": {
    mn: "\"Зөвхөн нэрлэсэн хүн\" гэдэг нь баримтын урсгалд регистрийн дугаараар нэрлэгдсэн хүн л гарын үсэг зурна гэсэн үг. Урсгалд хэн ч нэрлэгдээгүй бол энэ тохиргоо хадгалагдахгүй — тэр төрөл гарын үсэг зурах боломжгүй болох тул. Мөн баримт бүр өөрийн эхэлж авсан хэлхээгээрээ явдаг тул, аль хэдийн хүлээгдэж байгаа баримтуудын дунд хэн ч нэрлэгдээгүй нь байвал энэ тохиргоог асаахгүй — эхлээд тэднийг шийднэ.",
    en: "\"Named signer only\" means the signature must come from a registration number the type's approval chain names. It cannot be saved while the chain names nobody, because that would leave the type unsignable. Documents already waiting are held to the chain they started under, so it also cannot be turned on while any of them names nobody for an approval still to come — decide those first.",
  },

  // Approval chains
  "documents.menu.workflows": { mn: "Баримтын урсгал", en: "Document workflows" },
  "documents.view.workflows_hint": {
    mn: "Баримтын төрөл тус бүр хэдэн хүний гарын үсгийг ямар дараалалтай шаардахыг тогтооно.",
    en: "How many signatures each document type needs, and in what order.",
  },
  "documents.action.add_step": { mn: "Шат нэмэх", en: "Add step" },
  "documents.field.step_name": { mn: "Шатны нэр", en: "Step name" },
  "documents.field.step_signer": { mn: "Гарын үсэг зурагчийн регистр", en: "Signer's registration number" },
  "documents.message.workflows_failed": { mn: "Урсгалуудыг ачаалж чадсангүй", en: "Could not load approval chains" },
  "documents.message.workflow_saved": { mn: "{type}-ийн урсгал хадгалагдлаа.", en: "The {type} approval chain was saved." },
  "documents.message.chain_single_signature": {
    mn: "Шат тохируулаагүй — нэг гарын үсэг батална",
    en: "No steps — a single signature approves",
  },
  "documents.message.chain_signature_count": {
    mn: "{count} гарын үсэг шаардана",
    en: "{count} signatures required",
  },
  "documents.message.no_steps": {
    mn: "Шат байхгүй. Ингэснээр нэг гарын үсэг баримтыг батална.",
    en: "No steps. One signature approves the document.",
  },
  "documents.message.step_signer_hint": {
    mn: "Регистрийн дугаарыг хоосон орхивол шат тоологдох ч тодорхой хүн нэрлэгдэхгүй — гарын үсэг зурах эрхтэй хэн ч зурж болно.",
    en: "Leave a registration number empty and the step still counts, but names nobody: anyone who may sign can take it.",
  },

  // Retention
  "documents.menu.retention": { mn: "Хадгалалтын дүрэм", en: "Retention rules" },
  "documents.view.retention_hint": {
    mn: "Баримтын төрөл тус бүрийг хэдэн жил хадгалахыг тогтооно.",
    en: "How many years a document of each type is kept.",
  },
  "documents.field.retain_years": { mn: "Хадгалах жил", en: "Retain (years)" },
  "documents.field.retention_note": { mn: "Тайлбар", en: "Note" },
  "documents.stat.filed": { mn: "Бүртгэсэн баримт", en: "Documents filed" },
  "documents.stat.past_term": { mn: "Хугацаа дууссан", en: "Past their term" },
  "documents.stat.rules_set": { mn: "Тохируулсан дүрэм", en: "Rules set" },
  "documents.message.retention_failed": { mn: "Дүрмүүдийг ачаалж чадсангүй", en: "Could not load retention rules" },
  "documents.message.retention_saved": { mn: "{type}-ийн дүрэм хадгалагдлаа.", en: "The {type} retention rule was saved." },
  "documents.message.retention_no_deletion": {
    mn: "Энэ хуваарь ямар ч баримтыг устгахгүй. Хугацаа дууссаныг л харуулна — шийдвэрийг хүн гаргана.",
    en: "Nothing is deleted on this schedule. The screen only reports what is past its term; a person decides.",
  },

  // ── Гэрээ: талуудтай баримт. Дэлгэц нь app/module/documents/contracts ба
  // app/module/documents/inbox — хоёул client-gerege-nexus-ийн documents
  // модулийн API дээр сууна.
  "contracts.view.title": { mn: "Гэрээ", en: "Contracts" },
  "contracts.view.subtitle": {
    mn: "Олон талт гэрээ: бичвэрээ бичиж, талуудаа нэрлэж, илгээгээд тал бүр eID PIN2-оор гарын үсэг зурна.",
    en: "Multi-party contracts: write the text, name the parties, send, and each party signs with a qualified eID signature.",
  },
  "contracts.view.inbox_title": { mn: "Ирсэн гэрээ", en: "Incoming contracts" },
  "contracts.view.inbox_subtitle": {
    mn: "Танай байгууллагад ирсэн гэрээнүүд: уншиж, гарын үсэг зурагчаа нэрлэж, зурах эсвэл татгалзана.",
    en: "Contracts sent to your organisation: read, nominate your signatory, sign or decline.",
  },
  "contracts.view.new": { mn: "Шинэ гэрээ", en: "New contract" },
  "contracts.view.empty": { mn: "Хараахан гэрээ байхгүй байна.", en: "No contracts yet." },
  "contracts.view.inbox_empty": { mn: "Танд ирсэн гэрээ алга.", en: "Nothing has been sent to you." },
  "contracts.view.back": { mn: "Гэрээнүүд", en: "Contracts" },
  "contracts.view.inbox_back": { mn: "Ирсэн гэрээ", en: "Incoming" },

  "contracts.col.contract": { mn: "Гэрээ", en: "Contract" },
  "contracts.col.parties": { mn: "Талууд", en: "Parties" },
  "contracts.col.state": { mn: "Төлөв", en: "State" },
  "contracts.col.signatures": { mn: "Гарын үсэг", en: "Signatures" },
  "contracts.col.amount": { mn: "Дүн", en: "Amount" },
  "contracts.col.date": { mn: "Огноо", en: "Date" },
  "contracts.col.issuer": { mn: "Илгээгч", en: "From" },
  "contracts.col.received": { mn: "Ирсэн", en: "Received" },
  "contracts.col.due": { mn: "Эцсийн хугацаа", en: "Due" },

  "contracts.state.draft": { mn: "Ноорог", en: "Draft" },
  "contracts.state.sent": { mn: "Илгээгдсэн", en: "Sent" },
  "contracts.state.partial": { mn: "Хэсэгчлэн зурагдсан", en: "Partially signed" },
  "contracts.state.executed": { mn: "Хүчин төгөлдөр", en: "Executed" },
  "contracts.state.declined": { mn: "Татгалзсан", en: "Declined" },
  "contracts.state.withdrawn": { mn: "Эргүүлж татсан", en: "Withdrawn" },
  "contracts.state.expired": { mn: "Хугацаа дууссан", en: "Expired" },
  "contracts.state.terminated": { mn: "Цуцлагдсан", en: "Terminated" },
  "contracts.party_state.draft": { mn: "Ноорог", en: "Draft" },
  "contracts.party_state.invited": { mn: "Илгээгдсэн", en: "Sent" },
  "contracts.party_state.viewed": { mn: "Уншсан", en: "Opened" },
  "contracts.party_state.signed": { mn: "Гарын үсэг зурсан", en: "Signed" },
  "contracts.party_state.declined": { mn: "Татгалзсан", en: "Declined" },

  "contracts.role.issuer": { mn: "Гаргагч", en: "Issuer" },
  "contracts.role.counterparty": { mn: "Тал", en: "Counterparty" },
  "contracts.role.witness": { mn: "Гэрч", en: "Witness" },
  "contracts.role.guarantor": { mn: "Батлан даагч", en: "Guarantor" },
  "contracts.kind.member": { mn: "Дотоод хэрэглэгч", en: "Member of this organisation" },
  "contracts.kind.tenant": { mn: "Энэ платформ дээрх байгууллага", en: "Organisation on this platform" },
  "contracts.kind.person": { mn: "Иргэн (дансгүй)", en: "Person (no account)" },
  "contracts.kind.organisation": { mn: "Байгууллага (дансгүй)", en: "Organisation (no account)" },

  "contracts.field.title": { mn: "Гэрээний гарчиг", en: "Contract title" },
  "contracts.field.title_hint": {
    mn: "Гарчгийг дараа ч засаж болно. Талууд нэмэгдэх агшинд энэ баримт гэрээ болно.",
    en: "The title can still be edited later. The document becomes a contract when its first party is named.",
  },
  "contracts.field.number": { mn: "Гэрээний дугаар", en: "Contract number" },
  "contracts.field.amount": { mn: "Дүн", en: "Amount" },
  "contracts.field.currency": { mn: "Валют", en: "Currency" },
  "contracts.field.effective_from": { mn: "Хүчинтэй эхлэх", en: "Effective from" },
  "contracts.field.effective_to": { mn: "Хүчинтэй дуусах", en: "Effective to" },
  "contracts.field.due": { mn: "Хариу өгөх эцсийн хугацаа", en: "Answer due" },
  "contracts.field.name": { mn: "Нэр", en: "Name" },
  "contracts.field.reg": { mn: "Регистр", en: "Registration number" },
  "contracts.field.role": { mn: "Үүрэг", en: "Role" },
  "contracts.field.kind": { mn: "Төрөл", en: "Kind" },
  "contracts.field.email": { mn: "И-мэйл", en: "E-mail" },
  "contracts.field.phone": { mn: "Утас", en: "Phone" },
  "contracts.field.address": { mn: "Хаяг", en: "Address" },
  "contracts.field.sign_order": { mn: "Зурах дараалал", en: "Signing order" },
  "contracts.field.sign_order_hint": {
    mn: "Дараалал бичвэл өмнөх тал зурах хүртэл дараагийнх нь зурж чадахгүй.",
    en: "With an order set, a party cannot sign until the one before it has.",
  },
  "contracts.field.home_user": { mn: "Хэрэглэгчийн ID", en: "User ID" },
  "contracts.field.home_tenant": { mn: "Байгууллагын ID", en: "Organisation ID" },
  "contracts.field.home_tenant_hint": {
    mn: "Энэ суулгац дээрх байгууллагын UUID. Тэд өөрсдийн «Ирсэн гэрээ» дотроос харна.",
    en: "The UUID of an organisation on this platform. The contract appears in their own incoming list.",
  },
  "contracts.field.full_name": { mn: "Овог нэр", en: "Full name" },
  "contracts.field.position": { mn: "Албан тушаал", en: "Position" },
  "contracts.field.reason": { mn: "Шалтгаан", en: "Reason" },
  "contracts.field.mode": { mn: "Гарын үсгийн горим", en: "Signing mode" },
  "contracts.mode.counterpart": { mn: "Зэрэг (хэн ч эхэлж болно)", en: "Concurrent (any order)" },
  "contracts.mode.joint": { mn: "Дараалалтай", en: "In order" },

  "contracts.section.facts": { mn: "Гэрээний мэдээлэл", en: "Contract details" },
  "contracts.section.body": { mn: "Гэрээний текст (PDF байхгүй үед)", en: "Contract text (when there is no PDF)" },
  "contracts.section.parties": { mn: "Талууд", en: "Parties" },
  "contracts.section.add_party": { mn: "Тал нэмэх", en: "Add a party" },
  "contracts.section.send": { mn: "Илгээх", en: "Send" },
  "contracts.section.signatory": { mn: "Гарын үсэг зурах хүн", en: "Signatory" },
  "contracts.section.decision": { mn: "Шийдвэр", en: "Decision" },

  "contracts.body.placeholder": { mn: "Гэрээний бичвэрээ энд бичнэ.", en: "Write the contract text here." },
  "contracts.body.hint": {
    mn: "Хүлээн авагч бүрийн хувь дээр нэр, регистр нь автоматаар орно.",
    en: "Each recipient's copy carries their own name and registration automatically.",
  },
  "contracts.body.advanced": {
    mn: "Нарийн тохиргоо: бичвэрт {tokens} гэж бичсэн газарт тухайн талын мэдээлэл орлуулагдана.",
    en: "Advanced: where the text says {tokens}, that party's own details are substituted.",
  },
  "contracts.body.frozen_note": {
    mn: "Гэрээ илгээгдсэн. Бичвэрийг засах нь дахин илгээсэн хувьд л нөлөөлнө — аль хэдийн хүргэгдсэн хувь ХӨЛДӨӨТЭЙ.",
    en: "The contract has been sent. Edits only reach copies you re-send — a delivered copy is frozen.",
  },
  "contracts.send.note": {
    mn: "Илгээх агшинд тал бүрийн PDF зурагдаж ХӨЛДӨНӨ. Тэд яг тэр байтад гарын үсэг зурна.",
    en: "At send time each party's PDF is rendered and FROZEN. They sign exactly those bytes.",
  },
  "contracts.send.withdrawn_note": {
    mn: "Энэ гэрээ эргүүлж татагдсан: холбоосууд нь унтарсан, хариу өгөөгүй талууд хаагдсан.",
    en: "This contract was withdrawn: its links are dead and unanswered parties are closed.",
  },

  "contracts.action.create": { mn: "Үүсгэх", en: "Create" },
  "contracts.action.save": { mn: "Хадгалах", en: "Save" },
  "contracts.action.add": { mn: "Нэмэх", en: "Add" },
  "contracts.action.remove": { mn: "Хасах", en: "Remove" },
  "contracts.action.cancel": { mn: "Болих", en: "Cancel" },
  "contracts.action.send": { mn: "Илгээх", en: "Send" },
  "contracts.action.resend": { mn: "Дахин илгээх", en: "Send again" },
  "contracts.action.withdraw": { mn: "Эргүүлж татах", en: "Withdraw" },
  "contracts.action.reopen": { mn: "Дахин нээх", en: "Reopen" },
  "contracts.action.sign": { mn: "Гарын үсэг зурах (PIN2)", en: "Sign (PIN2)" },
  "contracts.action.sign_for_party": { mn: "Энэ талын өмнөөс зурах", en: "Sign for this party" },
  "contracts.action.decline": { mn: "Татгалзах", en: "Decline" },
  "contracts.action.invite": { mn: "Холбоос үүсгэх", en: "Create a link" },
  "contracts.action.add_signatory": { mn: "Гарын үсэг зурагч нэмэх", en: "Add a signatory" },
  "contracts.action.nominate": { mn: "Нэрлэх", en: "Nominate" },
  "contracts.action.copy_link": { mn: "Хуулах", en: "Copy" },
  "contracts.action.frozen_pdf": { mn: "Илгээсэн хувь", en: "Sent copy" },
  "contracts.action.signed_pdf": { mn: "Гарын үсэгтэй хувь", en: "Signed copy" },
  "contracts.action.view_pdf": { mn: "PDF харах", en: "View PDF" },

  "contracts.msg.check_phone": { mn: "Утсаа шалгана уу", en: "Check your phone" },
  "contracts.msg.poll_lost": {
    mn: "Сервертэй холбоо тасарлаа — утсан дээрх ёслол нээлттэй хэвээр. Дахин оролдоно уу.",
    en: "Lost contact with the server — the ceremony on your phone is still open. Try again.",
  },
  "contracts.issue.progress": { mn: "Тараалт: {done} / {total}", en: "Issuing: {done} / {total}" },
  "contracts.msg.refused": { mn: "Иргэн татгалзлаа.", en: "The citizen refused." },
  "contracts.msg.ceremony_ended": { mn: "Ёслол дуусав: {state}", en: "The ceremony ended: {state}" },
  "contracts.msg.saved": { mn: "Хадгаллаа.", en: "Saved." },
  "contracts.msg.sent": { mn: "{count} талд хүргэгдлээ.", en: "Delivered to {count} parties." },
  "contracts.msg.signed": { mn: "Гарын үсэг бүртгэгдлээ.", en: "The signature has been recorded." },
  "contracts.msg.declined_done": { mn: "Та энэ гэрээнээс татгалзсан.", en: "You declined this contract." },
  "contracts.msg.signed_done": { mn: "Та энэ гэрээнд гарын үсэг зурсан.", en: "You have signed this contract." },
  "contracts.msg.not_delivered": { mn: "Гэрээ хараахан хүргэгдээгүй байна.", en: "The contract has not been delivered yet." },
  "contracts.msg.no_signatory": {
    mn: "Танай байгууллагаас хэн гарын үсэг зурахыг нэрлээгүй байна.",
    en: "Your organisation has not yet named who signs.",
  },
  "contracts.msg.nominate_hint": {
    mn: "PIN2 хүсэлт энэ регистрийн дугаараар очно. Дугааргүй бол ёслол эхлэхгүй.",
    en: "The PIN2 request is addressed by this registration number; without one the ceremony cannot start.",
  },
  "contracts.msg.decline_hint": {
    mn: "Шалтгаангүй татгалзал нь илгээгчид юу засахыг нь хэлэхгүй.",
    en: "A refusal without a reason tells the issuer nothing about what to fix.",
  },
  "contracts.msg.invite_once": {
    mn: "Энэ холбоос ЗӨВХӨН ОДОО харагдана. Хуудсыг хаасны дараа дахин харах зам байхгүй — санд зөвхөн түүний хэш хадгалагдана.",
    en: "This link is shown ONLY NOW. There is no way to see it again — the database keeps only its hash.",
  },
  "contracts.msg.invite_expires": { mn: "Хүчинтэй хугацаа: {when}", en: "Valid until: {when}" },
  "contracts.msg.invite_for": { mn: "{name} — энэ холбоосыг илгээнэ үү.", en: "Send this link to {name}." },
  "contracts.msg.no_parties": {
    mn: "Хараахан тал нэрлээгүй. Гэрээ гэдэг нь хамгийн багадаа хоёр тал.",
    en: "No parties named yet. A contract is at least two parties.",
  },
  "contracts.msg.decline_reason_of": { mn: "Татгалзсан шалтгаан: {reason}", en: "Reason for declining: {reason}" },
  "contracts.msg.sha": { mn: "Баримтын SHA-256: {sha}", en: "Document SHA-256: {sha}" },
  "contracts.msg.load_failed": { mn: "Ачаалж чадсангүй.", en: "Could not load." },
  "contracts.msg.you": { mn: "(та)", en: "(you)" },
  "contracts.msg.signs_at": { mn: "{n}-рт зурна", en: "signs {n}." },

  // Мастер PDF ба Excel импорт.
  "contracts.section.pdf": { mn: "Гэрээний файл", en: "Contract file" },
  "contracts.pdf.note": {
    mn: "Word (.docx) хавсаргавал тараах үед хүн бүрийн хувь дээр ӨӨРИЙНХ нь нэр, регистр орж, PDF болж очно — гарын үсэг тэр PDF дээр зурагдана. PDF хавсаргавал бүгд яг ижил файл авна (доор өөрөө PIN2 зурж болно). Аль нь ч байхгүй бол доорх текстээс PDF үүсгэнэ.",
    en: "Attach a Word file (.docx) and at issue time each recipient's copy carries their own name and registration, converted to PDF — signatures land on that PDF. Attach a PDF and everyone receives the identical file (you can PIN2-sign it below). With neither, the PDF is rendered from the text below.",
  },
  "contracts.pdf.none": { mn: "Файл хавсаргаагүй — гэрээ доорх текстээс үүснэ.", en: "No file attached — the contract is rendered from the text below." },
  "contracts.pdf.word_badge": { mn: "Word загвар — тараахад хүн бүрд PDF болно", en: "Word template — becomes a per-recipient PDF at issue" },
  "contracts.action.word_template": { mn: "Word загвар татах (бөглөөд буцааж хавсаргана)", en: "Download the Word template (fill it in and attach it back)" },
  "contracts.pdf.master_signed": { mn: "Гаргагч зурсан ✓", en: "Signed by issuer ✓" },
  "contracts.action.attach_pdf": { mn: "Word / PDF хавсаргах", en: "Attach Word / PDF" },
  "contracts.action.replace_pdf": { mn: "Файл солих", en: "Replace the file" },
  "contracts.action.master_sign": { mn: "Өөрөө зурах (PIN2)", en: "Sign it yourself (PIN2)" },
  "contracts.msg.pdf_attached": { mn: "PDF хавсаргагдлаа.", en: "The PDF is attached." },
  "contracts.msg.master_signed": { mn: "Таны гарын үсэг PDF дээр суулаа.", en: "Your signature is on the PDF." },

  "contracts.section.import": { mn: "Excel-ээс талууд оруулах", en: "Import parties from a file" },
  "contracts.action.import_excel": { mn: "Файл сонгох (.xlsx / .csv)", en: "Choose a file (.xlsx / .csv)" },
  "contracts.action.import_template": { mn: "Загвар татах", en: "Download the template" },
  "contracts.import.hint": {
    mn: "Мөр бүр нэг хүлээн авагч: нэр | байгууллагын регистр | зурагчийн нэр | зурагчийн регистр | албан тушаал. Хоёрхон баганатай нь ч болно: нэр + регистр. Нэг мөрийн алдаа бусдыг унагахгүй.",
    en: "One recipient per row: name | org registration | signatory name | signatory registration | position. Two columns also work: name + registration. One bad row does not stop the rest.",
  },
  "contracts.import.result": { mn: "{added} тал нэмэгдэв, {skipped} мөр алгасав.", en: "{added} parties added, {skipped} rows skipped." },
  "contracts.import.row": { mn: "{row}-р мөр", en: "Row {row}" },

  // Тараалт: нэг загвар, хүн бүрд тусдаа гэрээ.
  "contracts.section.issue": { mn: "Олон хүнтэй байгуулах (тараалт)", en: "Conclude with many people (issue)" },
  "contracts.issue.note": {
    mn: "Ижил гэрээг олон хүнтэй байгуулах бол энд. Хүн бүртэй ТУС ТУСДАА гэрээ үүснэ: хэн ч бусдыгаа харахгүй, хүн бүрийн гэрээ өөрийнхөө гарын үсгээр хүчинтэй болно. Дээрх «Талууд» хэсэг бол өөр зүйл — НЭГ гэрээнд хамт зурах хүмүүс (худалдагч, худалдан авагч, гэрч).",
    en: "To conclude the same contract with many people, use this. Each person gets their OWN contract: nobody sees the others, and each contract is executed by its own signature. The Parties section above is different — the people who co-sign ONE contract (buyer, seller, witness).",
  },
  "contracts.issue.done": { mn: "{count} хүнд тус тусын гэрээ илгээгдлээ.", en: "{count} people received their own contract." },
  "contracts.issue.children": { mn: "Тараагдсан гэрээ ({count})", en: "Issued contracts ({count})" },
  "contracts.action.issue": { mn: "{count} хүнд тараах", en: "Issue to {count} people" },
  "contracts.field.recipient_name": { mn: "Хүний / байгууллагын нэр", en: "Person or organisation" },
  "contracts.field.recipient_reg": { mn: "Регистрийн дугаар", en: "Registration number" },
  "contracts.parties.note": {
    mn: "НЭГ гэрээнд хамт гарын үсэг зурах талууд. Олон хүнтэй тус тусад нь байгуулах бол доорх «Тараалт»-ыг ашиглана.",
    en: "The parties who co-sign THIS one contract. To conclude it separately with many people, use Issue below.",
  },
  "contracts.list.issued": { mn: "Тараасан: {total} · хүчин төгөлдөр: {executed}", en: "Issued: {total} · executed: {executed}" },
} as const;
