import type { LineContent } from "@/app/line/[line]/lines";

/**
 * Шугамын хоёр дахь дэлгэц — дөрвөн шугамд нэг бие.
 *
 * Шугам бүр өөрийн дэлгэцүүдийг `app/line/<line>/…` дор хөгжүүлнэ. Тэдгээр
 * нь платформын дэлгэц (`/apps`, `/settings/…`) руу холбогддоггүй: тэр
 * хуудсууд өөрийн толгой хэсэг, хажуугийн цэс, хэрэглэгчийн цэстэйгээ
 * бүтнээрээ ирдэг бөгөөд native бүрхүүлийн хүрээн дотор ХОЁР ДАХЬ бүрхүүл
 * болж давхарладаг — `docs/SHELL_CONTRACT.md` §1 яг үүнээс сэргийлдэг.
 *
 * Дэлгэцийн дүрэм:
 *
 *   1. Өөрийн толгой хэсэг, навигаци зурахгүй. Буцах нь БҮРХҮҮЛИЙН товч:
 *      macOS-д толгой хэсэгт (`⌘[`), iOS/Android-д ажлын мужийн дээд мөрөнд.
 *      Хуудас өөрөө хоёр дахь буцахыг зурвал нэг үйлдэлд хоёр товч болно.
 *   2. Нэвтрэлт шаардахгүй.
 *   3. Платформын route руу хөтлөхгүй.
 */
export default function LineSecondScreen({ content }: { content: LineContent }) {
  return (
    <div
      className={`line-home line-home--${content.posture}`}
      style={{ ["--line-alloy" as string]: content.alloy, ["--line-alloy-rgb" as string]: content.alloyRGB }}
    >
      <header className="line-mast">
        <p className="line-eyebrow">{content.eyebrow}</p>
        <h1 className="line-title">2 дахь дэлгэц</h1>
      </header>
    </div>
  );
}
