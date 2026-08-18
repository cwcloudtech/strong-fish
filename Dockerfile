# syntax=docker/dockerfile:1

ARG NODE_IMAGE_TAG=20-alpine
ARG GOLANG_IMAGE_TAG=1.26-alpine
ARG ALPINE_IMAGE_TAG=3.20
ARG NGINX_IMAGE_TAG=1.27-alpine
ARG FLUTTER_IMAGE_TAG=3.44.0

# Stage ui build
FROM node:${NODE_IMAGE_TAG} AS ui-build
WORKDIR /app
COPY sf-ui/package.json sf-ui/package-lock.json ./
RUN npm ci
COPY sf-ui/ ./
COPY manifest.json ./manifest.json
RUN npm run build:docker

# Stage wiki build: the Docusaurus documentation site (sf-wiki), served on its
# own domain rather than from the app - it is public, static, and changes on a
# different rhythm than the product.
FROM node:${NODE_IMAGE_TAG} AS wiki-build
WORKDIR /app
COPY sf-wiki/package.json sf-wiki/package-lock.json ./
RUN npm ci
COPY sf-wiki/ ./
RUN npm run build

# Stage api build
FROM golang:${GOLANG_IMAGE_TAG} AS api-build
WORKDIR /app
COPY sf-api/go.mod sf-api/go.sum ./
RUN go mod download
COPY sf-api/ ./
COPY manifest.json ./manifest.json
RUN CGO_ENABLED=0 go build -o /out/sf-api .

# Stage mobile build (android only)
FROM ghcr.io/cirruslabs/flutter:${FLUTTER_IMAGE_TAG} AS mobile-build
WORKDIR /app
COPY sf-mobile/ ./
COPY VERSION ./VERSION

# The APK is signed with a persistent key supplied as a build secret, never
# baked into an image layer. Without it (local builds) the mounted file is empty
# and Gradle falls back to debug signing - fine for a test install, but a
# release must keep the same key or self-updates break with "App not installed".
RUN --mount=type=secret,id=mobile_keystore,target=/run/secrets/mobile_keystore.jks \
    --mount=type=secret,id=mobile_keystore_password,env=MOBILE_KEYSTORE_PASSWORD \
    --mount=type=secret,id=mobile_key_alias,env=MOBILE_KEY_ALIAS \
    --mount=type=secret,id=mobile_key_password,env=MOBILE_KEY_PASSWORD \
    VERSION="$(cat VERSION)" && \
    ANDROID_VERSION_CODE="$(echo "${VERSION}" | awk -F. '{printf "%d%02d%02d", $1, $2, $3}')" && \
    sed -i "s/^version: .*/version: ${VERSION}+${ANDROID_VERSION_CODE}/" pubspec.yaml && \
    flutter pub get && \
    if [ -s /run/secrets/mobile_keystore.jks ]; then export MOBILE_KEYSTORE_PATH=/run/secrets/mobile_keystore.jks; fi && \
    flutter build apk --release && \
    mv /app/build/app/outputs/flutter-apk/app-release.apk "/app/build/app/outputs/flutter-apk/strong-fish-v${VERSION}.apk"

# Stage api run
FROM alpine:${ALPINE_IMAGE_TAG} AS api
RUN apk add --no-cache ca-certificates
COPY --from=api-build /out/sf-api /usr/local/bin/sf-api
COPY --from=api-build /app/manifest.json /manifest.json
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/sf-api"]

# Stage ui run
FROM nginx:${NGINX_IMAGE_TAG} AS ui
COPY --from=ui-build /app/build /usr/share/nginx/html
COPY --from=ui-build /app/manifest.json /usr/share/nginx/html/manifest-version.json
COPY .docker/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY .docker/nginx/docker-entrypoint.sh /docker-entrypoint.sh
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]

# Stage wiki run.
# Stage wiki run. The same nginx config and entrypoint the app uses - the wiki
# is a static site behind the same reverse proxy, and the entrypoint's env
# substitution is what lets one image be pointed at any deployment.
FROM nginx:${NGINX_IMAGE_TAG} AS wiki
COPY .docker/nginx/default.conf /etc/nginx/conf.d/default.conf
COPY .docker/nginx/docker-entrypoint.sh /docker-entrypoint.sh
COPY --from=wiki-build /app/build /usr/share/nginx/html
COPY manifest.json /usr/share/nginx/html/manifest.json
RUN chmod +x /docker-entrypoint.sh && \
    chmod -R 755 /usr/share/nginx/html && \
    chown -R nginx:nginx /usr/share/nginx/html
ENTRYPOINT ["/docker-entrypoint.sh"]
CMD ["nginx", "-g", "daemon off;"]

# Stage ui-and-mobile run: serves the web app plus the downloadable APK.
FROM ui AS ui-and-mobile
COPY --from=mobile-build /app/build/app/outputs/flutter-apk/*.apk /usr/share/nginx/html/
