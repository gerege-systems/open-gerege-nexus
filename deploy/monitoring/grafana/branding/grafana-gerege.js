/*
 * Grafana-ийн дээд баруун булан дахь gravatar зургийг хэрэглэгчийн нэрний
 * эхний үсгүүдээр солино.
 *
 * DOM-д хүрэхгүй: React-ийн модыг гараар өөрчлөх нь хожим "node not found"
 * гэж унадаг. Оронд нь :root дээр нэг CSS хувьсагч тавьж, солилтыг
 * grafana-gerege.css дэх `img[alt="User avatar"] { content: var(--gg-avatar) }`
 * дүрэм хийнэ. Хувьсагч тавигдаагүй бол зураг хэвийн харагдана.
 *
 * `alt="User avatar"` нь зөвхөн дээд талын профайл товчны зурагт байдаг —
 * админы хэрэглэгчдийн жагсаалт дахь бусад хүний зураг хөндөгдөхгүй.
 */
(() => {
  const user = window.grafanaBootData && window.grafanaBootData.user;
  if (!user) return;

  const name = String(user.name || user.login || user.email || '').trim();
  const initials =
    name.split(/\s+/).filter(Boolean).slice(0, 2).map((w) => w[0]).join('').toUpperCase() || '?';

  const svg =
    "<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 40 40'>" +
    "<circle cx='20' cy='20' r='20' fill='#e7f0fd'/>" +
    "<text x='20' y='20' text-anchor='middle' dominant-baseline='central'" +
    " font-family='Inter, -apple-system, system-ui, sans-serif' font-size='15'" +
    " font-weight='700' fill='#0050b0'>" + initials + "</text></svg>";

  document.documentElement.style.setProperty(
    '--gg-avatar',
    'url("data:image/svg+xml,' + encodeURIComponent(svg) + '")',
  );
})();
