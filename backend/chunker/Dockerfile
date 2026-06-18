FROM python:3.11-slim AS builder

WORKDIR /build

ENV PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PIP_NO_CACHE_DIR=1

COPY requirements.txt .

RUN apt-get update && apt-get install -y --no-install-recommends \
        build-essential \
        gcc \
    && pip install --upgrade pip wheel \
    && pip wheel --wheel-dir /wheels -r requirements.txt \
    && apt-get purge -y build-essential gcc \
    && rm -rf /var/lib/apt/lists/*


FROM python:3.11-slim

WORKDIR /app

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

# Runtime libs occasionally needed by document parsers
RUN apt-get update && apt-get install -y --no-install-recommends \
        libmagic1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /wheels /wheels

RUN pip install --no-cache-dir /wheels/* \
    && rm -rf /wheels

COPY . .

EXPOSE 50051

CMD ["python", "src/server.py"]