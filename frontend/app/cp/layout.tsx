import { headers } from "next/headers";
import { notFound } from "next/navigation";

/**
 * The operator console's routes exist only on the console's hostname.
 *
 * This is a server component so the decision is made before any of it is sent:
 * a request to nexus.gerege.mn/cp gets the 404 page, not the console's HTML
 * with a client-side redirect after it. The same rule is enforced again by the
 * API (controlplane.HostGate) and again by nginx's address allowlist — three
 * layers, none of which is asked to trust another.
 *
 * CONTROL_PLANE_HOST is read here rather than baked in as NEXT_PUBLIC_*,
 * because a NEXT_PUBLIC_ value is compiled into the bundle every visitor
 * downloads, and the console's hostname is not something the public build
 * should be carrying around.
 */
export default async function ControlPlaneLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  const configured = (process.env.CONTROL_PLANE_HOST || "").trim().toLowerCase();
  const host = (await headers()).get("host")?.toLowerCase() ?? "";

  if (configured) {
    // Compared without the port: a hostname written with one in the
    // environment file, or a development server on :3000, must still match.
    if (stripPort(host) !== stripPort(configured)) notFound();
  } else if (process.env.NODE_ENV === "production") {
    // A deployment that never set the variable has no console. That is the
    // safe reading of an unset value: the alternative is a console reachable
    // on the hostname every tenant already uses.
    notFound();
  }

  return <div className="cp-root min-h-screen">{children}</div>;
}

function stripPort(host: string): string {
  const colon = host.lastIndexOf(":");
  return colon > -1 && !host.slice(colon).includes("]") ? host.slice(0, colon) : host;
}
