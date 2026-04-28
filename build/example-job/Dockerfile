FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1 \
    NOTEBOOK_PATH=/app/notebooks/job.ipynb \
    OUTPUT_DIR=/output

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    tini \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt ./requirements.txt
RUN pip install -r requirements.txt

COPY entrypoint.sh /entrypoint.sh
COPY notebooks /app/notebooks

RUN chmod +x /entrypoint.sh && mkdir -p /output

ENTRYPOINT ["/usr/bin/tini", "--", "/entrypoint.sh"]
