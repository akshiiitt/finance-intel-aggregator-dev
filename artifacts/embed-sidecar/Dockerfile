FROM python:3.11-slim

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY main.py .

# Pre-download the model at build time so the first real request isn't slow.
RUN python -c "from fastembed import TextEmbedding; TextEmbedding(model_name='BAAI/bge-small-en-v1.5')"

EXPOSE 8900
# Bind 0.0.0.0 inside the container — this is fine because docker-compose.yml
# does NOT publish a host port for this service; it's only reachable from
# other containers on the internal compose network (e.g. the Go backend).
CMD ["uvicorn", "main:app", "--host", "0.0.0.0", "--port", "8900", "--workers", "1"]
