#!/usr/bin/env bash
set -euo pipefail

NOTEBOOK_PATH="${NOTEBOOK_PATH:-/app/notebooks/job.ipynb}"
OUTPUT_DIR="${OUTPUT_DIR:-/output}"
EXECUTED_NOTEBOOK="${EXECUTED_NOTEBOOK:-${OUTPUT_DIR}/executed-notebook.ipynb}"
HTML_EXPORT="${HTML_EXPORT:-${OUTPUT_DIR}/report.html}"

mkdir -p "${OUTPUT_DIR}"

if [[ ! -f "${NOTEBOOK_PATH}" ]]; then
  echo "Notebook not found: ${NOTEBOOK_PATH}" >&2
  exit 1
fi

echo "Executing notebook: ${NOTEBOOK_PATH}"
echo "Writing artefacts to: ${OUTPUT_DIR}"

papermill \
  "${NOTEBOOK_PATH}" \
  "${EXECUTED_NOTEBOOK}" \
  -k python3 \
  -p output_dir "${OUTPUT_DIR}"

jupyter nbconvert \
  --to html \
  --output "$(basename "${HTML_EXPORT}")" \
  --output-dir "$(dirname "${HTML_EXPORT}")" \
  "${EXECUTED_NOTEBOOK}"

echo "Done. Created:"
ls -lah "${OUTPUT_DIR}"
