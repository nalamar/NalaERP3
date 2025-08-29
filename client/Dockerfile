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

RUN dart pub global activate dhttpd
ENV PATH="/root/.pub-cache/bin:${PATH}"
CMD ["bash", "-lc", "flutter config --no-analytics && flutter pub upgrade && flutter pub get && flutter build web --release --no-wasm-dry-run && dhttpd --path build/web --host 0.0.0.0 --port 3000"]
