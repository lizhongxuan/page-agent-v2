FROM python:3.12-slim

WORKDIR /app

COPY kb_service /app/kb_service

ENV KB_HOST=0.0.0.0
ENV KB_PORT=8787
ENV KB_DB_PATH=/data/knowledge.sqlite3

EXPOSE 8787
VOLUME ["/data"]

CMD ["python", "-m", "kb_service"]
