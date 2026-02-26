#!/bin/sh
set -e

MIGRATIONS_DIR=${MIGRATIONS_DIR:-/app/migrations}
MAX_ATTEMPTS=${MIGRATE_MAX_ATTEMPTS:-30}
SLEEP_SECONDS=1

wait_for_rabbitmq() {
  queue_url="${QUEUE_URL:-}"
  queue_type="${QUEUE_TYPE:-}"

  if [ "${queue_type}" != "rabbitmq" ] && [ "${queue_url#amqp}" = "${queue_url}" ]; then
    return 0
  fi

  if [ -z "${queue_url}" ]; then
    echo "QUEUE_URL not set; skipping RabbitMQ wait."
    return 0
  fi

  hostport="$(echo "${queue_url}" | sed -E 's,^[a-zA-Z]+://,,; s,/.*$,,'; s,^[^@]*@,,')"
  host="$(echo "${hostport}" | cut -d: -f1)"
  port="$(echo "${hostport}" | cut -s -d: -f2)"
  if [ -z "${port}" ]; then
    port="5672"
  fi

  attempt=1
  sleep_seconds=1
  while [ "${attempt}" -le "${MAX_ATTEMPTS}" ]; do
    echo "Waiting for RabbitMQ at ${host}:${port} (attempt ${attempt}/${MAX_ATTEMPTS})..."
    if nc -z "${host}" "${port}" >/dev/null 2>&1; then
      echo "RabbitMQ is reachable."
      return 0
    fi

    if [ "${attempt}" -ge "${MAX_ATTEMPTS}" ]; then
      echo "RabbitMQ not reachable after ${MAX_ATTEMPTS} attempts."
      return 1
    fi

    echo "RabbitMQ not ready; retrying in ${sleep_seconds}s..."
    sleep "${sleep_seconds}"
    attempt=$((attempt + 1))
    if [ "${sleep_seconds}" -lt 10 ]; then
      sleep_seconds=$((sleep_seconds + 1))
    fi
  done
}

wait_for_rabbitmq

attempt=1
while [ "$attempt" -le "$MAX_ATTEMPTS" ]; do
  echo "Running migrations (attempt ${attempt}/${MAX_ATTEMPTS})..."
  if /app/migrate -action up -dir "${MIGRATIONS_DIR}"; then
    echo "Migrations completed successfully."
    break
  fi

  if [ "$attempt" -ge "$MAX_ATTEMPTS" ]; then
    echo "Migrations failed after ${MAX_ATTEMPTS} attempts."
    exit 1
  fi

  echo "Migration failed; retrying in ${SLEEP_SECONDS}s..."
  sleep "$SLEEP_SECONDS"
  attempt=$((attempt + 1))
  if [ "$SLEEP_SECONDS" -lt 10 ]; then
    SLEEP_SECONDS=$((SLEEP_SECONDS + 1))
  fi
done

exec /app/api "$@"
