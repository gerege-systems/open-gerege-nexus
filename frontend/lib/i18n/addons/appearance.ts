/**
 * appearance — Per-device presentation: theme, colour mode, accent and density.
 */
export const appearance = {
  "appearance.view.title": { mn: "Харагдац", en: "Appearance" },
  "appearance.view.subtitle": { mn: "Энэ төхөөрөмж дээр {brand} хэрхэн харагдахыг тохируулна.", en: "Choose how {brand} looks on this device." },
  "appearance.view.color_mode_hint": { mn: "Гэгээн, харанхуй эсвэл төхөөрөмжийн тохиргоог дагана.", en: "Light, dark, or follow the device setting." },
  "appearance.view.accent_hint": { mn: "Cobalt нь Gerege-ийн үндсэн брэнд өнгө.", en: "Cobalt is the primary Gerege brand colour." },

  // Language availability. Mongolian and English always ship; the rest are
  // offered per device, so the copy has to say that they are additions rather
  // than a full switch of the interface.
  "appearance.field.languages": { mn: "Хэлүүд", en: "Languages" },
  "appearance.view.languages_hint": {
    mn: "Монгол, англи хоёр үргэлж нээлттэй. НҮБ-ын үлдсэн хэлүүдийг энэ төхөөрөмж дээр нэмж, хасаж болно.",
    en: "Mongolian and English are always available. The remaining UN languages can be added or removed on this device.",
  },
  "appearance.view.languages_partial": {
    mn: "Орчуулагдаагүй үг англи хэлээр харагдана.",
    en: "Terms that are not translated yet appear in English.",
  },
  "appearance.state.language_always": { mn: "Үндсэн", en: "Default" },

  "appearance.field.color_mode": { mn: "Өнгөний горим", en: "Colour mode" },
  "appearance.field.accent": { mn: "Онцлох өнгө", en: "Accent colour" },
  "appearance.field.density": { mn: "Дэлгэцийн нягтрал", en: "Display density" },


  "appearance.mode.light": { mn: "Гэгээн", en: "Light" },
  "appearance.mode.dark": { mn: "Харанхуй", en: "Dark" },
  "appearance.mode.system": { mn: "Системийн", en: "System" },

  "appearance.accent.default": { mn: "Суулгацын өнгө", en: "This deployment's" },
  "appearance.accent.neutral": { mn: "Цагаан саарал", en: "White and grey" },
  "appearance.accent.cobalt": { mn: "Gerege cobalt", en: "Gerege cobalt" },
  "appearance.accent.teal": { mn: "Хөх ногоон", en: "Teal" },
  "appearance.accent.violet": { mn: "Нил ягаан", en: "Violet" },
  "appearance.accent.emerald": { mn: "Маргад", en: "Emerald" },

  "appearance.density.comfortable": { mn: "Тав тухтай", en: "Comfortable" },
  "appearance.density.compact": { mn: "Нягт", en: "Compact" },

  "appearance.action.reset": { mn: "Анхны төлөв", en: "Reset to defaults" },
} as const;
