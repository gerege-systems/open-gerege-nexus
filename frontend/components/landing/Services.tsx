"use client";

import {ArrowUpRight} from "lucide-react";

import ServiceArt from "@/components/landing/ServiceArt";
import type {Service} from "@/lib/services";
import {useI18n} from "@/lib/i18n";

/**
 * The rest of the deployment, as cards.
 *
 * Everything else on this page argues about the platform. This section says
 * what is actually running beside it and hands over the address: the console,
 * the warehouse, the backup store, the monitoring stack, the manual. A
 * visitor who reads the whole landing page and still does not know the
 * documentation has its own hostname has been told the wrong things.
 *
 * The whole card is the link rather than a "read more" at the bottom of it.
 * A card that looks like a button in five places but only responds in one is
 * a small, repeated disappointment; the arrow marks where the eye expects the
 * control, and the surface it sits on is what receives the press.
 *
 * They open in a new tab. These are separate applications with their own
 * sign-in, and replacing the page somebody is reading in order to show them a
 * login screen loses both.
 *
 * Nothing configured means nothing rendered: the section is not drawn at all
 * rather than drawn empty. See `lib/services.ts` for why the addresses live in
 * the deployment's environment.
 */
export default function Services({services}: {services: Service[]}) {
  const {t} = useI18n();
  if (services.length === 0) return null;

  return (
    <section className="gp-section" id="services">
      <div className="gp-heading">
        <span>{t("website.service.eyebrow")}</span>
        <h2>{t("website.service.title")}</h2>
        <p>{t("website.service.lede")}</p>
      </div>

      <div className="gp-services">
        {services.map((service) => (
          <a
            key={service.id}
            className="gp-service"
            href={service.href}
            target="_blank"
            rel="noopener"
          >
            <span className="gp-service__art">
              <ServiceArt id={service.id} />
            </span>
            <span className="gp-service__body">
              <h3>
                {t(service.title)}
                <ArrowUpRight />
              </h3>
              <p>{t(service.body)}</p>
              {/* The address itself, because it is the thing being handed
                  over. `new URL` rather than a regular expression: the value
                  was already validated as an absolute URL when it was read. */}
              <span className="gp-service__host">{new URL(service.href).host}</span>
            </span>
          </a>
        ))}
      </div>
    </section>
  );
}
