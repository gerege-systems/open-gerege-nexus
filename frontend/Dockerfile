FROM node:20-alpine AS builder
WORKDIR /app

# Playwright's postinstall would otherwise fetch three browsers into a layer
# that only ever runs `next build`. The end-to-end suite is CI's and the
# developer's; the image does not run it.
ENV PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1

# package-lock.json must be copied too, otherwise `npm ci` is impossible and
# `npm install` silently resolves different dependency versions per build.
COPY package.json package-lock.json ./
RUN npm ci

COPY . .

# NEXT_PUBLIC_* values are baked into the client bundle at build time.
# Empty by default, and production leaves it empty: the browser then asks the
# host that served the page, which is what keeps one image usable by every
# deployment. Set it only where the API is genuinely on another origin.
ARG NEXT_PUBLIC_API_URL=
ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL
ARG NEXT_PUBLIC_CONTROL_PLANE_API_URL=
ENV NEXT_PUBLIC_CONTROL_PLANE_API_URL=$NEXT_PUBLIC_CONTROL_PLANE_API_URL

# Browser error reporting. It has to be a build argument rather than a runtime
# variable because NEXT_PUBLIC_* is substituted into the bundle here — an image
# built without it cannot be given one later by editing compose. Empty is off,
# which is what every build that does not pass it gets.
#
# The DSN is not a secret: it names the project to write to and grants nothing
# else. The release is what ties an error to the build that caused it.
ARG NEXT_PUBLIC_SENTRY_DSN=
ENV NEXT_PUBLIC_SENTRY_DSN=$NEXT_PUBLIC_SENTRY_DSN
ARG NEXT_PUBLIC_RELEASE_VERSION=
ENV NEXT_PUBLIC_RELEASE_VERSION=$NEXT_PUBLIC_RELEASE_VERSION
ARG NEXT_PUBLIC_ENVIRONMENT=production
ENV NEXT_PUBLIC_ENVIRONMENT=$NEXT_PUBLIC_ENVIRONMENT

RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production

COPY --from=builder /app/package.json /app/package-lock.json ./
RUN npm ci --omit=dev
COPY --from=builder /app/.next ./.next
COPY --from=builder /app/public ./public

USER node
EXPOSE 3000
CMD ["npm", "start"]
