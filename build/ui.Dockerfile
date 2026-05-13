# Build UI
FROM node:22-alpine AS build
WORKDIR /app
ARG VERSION=
ENV VITE_VERSION=${VERSION}

COPY ui/package.json ui/package-lock.json* ./
RUN npm install

COPY ui/ ./

RUN npm run build

# Serve UI
FROM nginx:alpine
ARG VERSION=
ENV UI_BUILD_VERSION=${VERSION}
RUN apk add --no-cache gettext
WORKDIR /usr/share/nginx/html
COPY build/nginx.conf /etc/nginx/conf.d/default.conf
COPY build/ui-entrypoint.sh /usr/local/bin/ui-entrypoint.sh
COPY --from=build /app/dist ./
RUN chmod +x /usr/local/bin/ui-entrypoint.sh
ENTRYPOINT ["/usr/local/bin/ui-entrypoint.sh"]
