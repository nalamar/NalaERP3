# Flutter Web Entwicklungsserver auf Port 3000
FROM ghcr.io/cirruslabs/flutter:stable

ENV LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    TZ=Europe/Berlin

WORKDIR /app

# Nur pubspec zuerst kopieren für bessere Caching-Effizienz
COPY pubspec.yaml ./
RUN flutter pub get || true

# Quellcode
COPY lib ./lib
COPY web ./web

EXPOSE 3000

CMD ["bash", "-lc", "flutter config --no-analytics && flutter pub get && flutter run -d web-server --web-port 3000 --web-hostname 0.0.0.0"]

