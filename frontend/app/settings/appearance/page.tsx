"use client";

import { Moon, Monitor, Palette, RotateCcw, Sun } from "lucide-react";
import {
  Button,
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
  RadioGroup,
  RadioItem,
  Switch,
  Tabs,
  TabsList,
  TabsTrigger,
} from "@gerege-systems/ui";
import { Accent, ColorMode, Density, useTheme } from "@/lib/theme";
import { DEFAULT_LOCALES, LOCALES, TranslationKey, useI18n } from "@/lib/i18n";

// Every label resolves through the dictionary, so this screen switches with the
// rest of the app instead of carrying its own inline translations.
const modes: { value: ColorMode; label: TranslationKey; icon: typeof Sun }[] = [
  { value: "light", label: "appearance.mode.light", icon: Sun },
  { value: "dark", label: "appearance.mode.dark", icon: Moon },
  { value: "system", label: "appearance.mode.system", icon: Monitor },
];

/**
 * The swatch beside each accent is a picture of the preset, not the preset: the
 * tokens themselves live in CSS (cobalt, teal and neutral in app/globals.css,
 * violet and emerald in the library's theme.css) and apply to <html> only. A
 * swatch cannot borrow them by setting `data-accent` on itself, so its light
 * value is written once, here.
 */
const accents: { value: Accent; label: TranslationKey; color: string }[] = [
  // The deployment's own theme (BRAND_THEME_*): no override in this browser.
  { value: "", label: "appearance.accent.default", color: "var(--accent)" },
  { value: "neutral", label: "appearance.accent.neutral", color: "#64748b" },
  { value: "cobalt", label: "appearance.accent.cobalt", color: "#0064e1" },
  { value: "teal", label: "appearance.accent.teal", color: "#008b99" },
  { value: "violet", label: "appearance.accent.violet", color: "#7656d6" },
  { value: "emerald", label: "appearance.accent.emerald", color: "#16845b" },
];

const densities: { value: Density; label: TranslationKey }[] = [
  { value: "comfortable", label: "appearance.density.comfortable" },
  { value: "compact", label: "appearance.density.compact" },
];

/** The bordered option a radio sits in; the chosen one is lifted onto the accent. */
const option = (chosen: boolean) =>
  `rounded-md border p-3 transition ${chosen ? "border-accent bg-accent-soft" : "border-line hover:border-input"}`;

export default function AppearanceSettingsPage() {
  const { t, availableLocales, setLocaleEnabled } = useI18n();
  const theme = useTheme();

  return (
    <div className="w-full space-y-6">
      <div className="border-b border-line pb-4 flex items-start justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold flex items-center gap-2">
            <Palette className="w-6 h-6 text-accent" />
            {t("appearance.view.title")}
          </h1>
          <p className="text-sm text-muted mt-1">{t("appearance.view.subtitle")}</p>
        </div>
        <Button variant="outline" leadingIcon={<RotateCcw />} onClick={theme.resetTheme}>
          {t("appearance.action.reset")}
        </Button>
      </div>

      <Card asChild padding="lg">
        <section>
          <CardHeader>
            <CardTitle>{t("appearance.field.languages")}</CardTitle>
            <CardDescription>{t("appearance.view.languages_hint")}</CardDescription>
            <p className="text-xs text-muted">{t("appearance.view.languages_partial")}</p>
          </CardHeader>
          <CardContent>
            <ul className="divide-y divide-line border border-line rounded-lg overflow-hidden">
              {LOCALES.map((option) => {
                const isDefault = DEFAULT_LOCALES.includes(option.code);
                const isOn = availableLocales.includes(option.code);
                return (
                  <li key={option.code} className="flex items-center gap-3 px-4 py-3">
                    <span className="text-sm font-medium text-foreground min-w-0 truncate">{option.label}</span>
                    <span className="text-xs uppercase tracking-wider text-muted">{option.code}</span>
                    <span className="ms-auto">
                      {isDefault ? (
                        // Not a disabled control: there is nothing to press, so the
                        // state is stated rather than shown as a dead switch.
                        <span className="text-xs text-muted">{t("appearance.state.language_always")}</span>
                      ) : (
                        <Switch
                          label={option.label}
                          hideLabel
                          checked={isOn}
                          onCheckedChange={(next) => setLocaleEnabled(option.code, next)}
                        />
                      )}
                    </span>
                  </li>
                );
              })}
            </ul>
          </CardContent>
        </section>
      </Card>

      <Card asChild padding="lg">
        <section>
          <CardHeader>
            <CardTitle>{t("appearance.field.color_mode")}</CardTitle>
            <CardDescription>{t("appearance.view.color_mode_hint")}</CardDescription>
          </CardHeader>
          <CardContent>
            <RadioGroup
              aria-label={t("appearance.field.color_mode")}
              value={theme.mode}
              onValueChange={(value) => theme.updateTheme({ mode: value as ColorMode })}
              className="grid sm:grid-cols-3 gap-3"
            >
              {modes.map(({ value, label, icon: Icon }) => (
                <RadioItem
                  key={value}
                  value={value}
                  className={option(theme.mode === value)}
                  label={
                    <span className="flex items-center gap-2 font-medium">
                      <Icon className="w-4 h-4 text-accent" aria-hidden="true" />
                      {t(label)}
                    </span>
                  }
                />
              ))}
            </RadioGroup>
          </CardContent>
        </section>
      </Card>

      <Card asChild padding="lg">
        <section>
          <CardHeader>
            <CardTitle>{t("appearance.field.accent")}</CardTitle>
            <CardDescription>{t("appearance.view.accent_hint")}</CardDescription>
          </CardHeader>
          <CardContent>
            <RadioGroup
              aria-label={t("appearance.field.accent")}
              value={theme.accent}
              onValueChange={(value) => theme.updateTheme({ accent: value as Accent })}
              className="grid sm:grid-cols-2 gap-3"
            >
              {accents.map((accent) => (
                <RadioItem
                  key={accent.value}
                  value={accent.value}
                  className={option(theme.accent === accent.value)}
                  label={
                    <span className="flex items-center gap-3 font-medium">
                      <span className="w-6 h-6 rounded-md" style={{ background: accent.color }} aria-hidden="true" />
                      {t(accent.label)}
                    </span>
                  }
                />
              ))}
            </RadioGroup>
          </CardContent>
        </section>
      </Card>

      <Card asChild padding="lg">
        <section>
          <CardHeader>
            <CardTitle>{t("appearance.field.density")}</CardTitle>
          </CardHeader>
          <CardContent>
            <Tabs value={theme.density} onValueChange={(value) => theme.updateTheme({ density: value as Density })}>
              <TabsList variant="pills" aria-label={t("appearance.field.density")}>
                {densities.map(({ value, label }) => (
                  <TabsTrigger key={value} value={value}>
                    {t(label)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
          </CardContent>
        </section>
      </Card>
    </div>
  );
}
