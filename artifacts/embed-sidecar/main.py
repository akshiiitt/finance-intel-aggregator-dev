"""FinanceIntel embedding sidecar.

A small FastAPI service wrapping a local sentence-embedding model via
fastembed (ONNX runtime under the hood, CPU-only, no GPU required, no
external API calls). Runs on the same Oracle VM as the Go backend.

It has exactly one job: turn text into a vector. Everything downstream
(dedup, semantic search, niche tagging) is plain math in the Go backend and
Postgres/pgvector — this service never sees or makes dedup/scoring decisions.

Not exposed on any public port — see docker-compose.yml, which puts this on
the internal network only. The Go backend reaches it at EMBED_SIDECAR_URL.
"""

import os
from typing import List

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field
from fastembed import TextEmbedding

MODEL_NAME = os.environ.get("EMBED_MODEL", "BAAI/bge-small-en-v1.5")
MAX_BATCH = 64

app = FastAPI(title="financeintel-embed-sidecar")
model = TextEmbedding(model_name=MODEL_NAME)


class EmbedRequest(BaseModel):
    texts: List[str] = Field(..., min_length=1, max_length=MAX_BATCH)


class EmbedResponse(BaseModel):
    vectors: List[List[float]]
    model: str
    dim: int


@app.get("/health")
def health():
    return {"status": "ok", "model": MODEL_NAME}


@app.post("/embed", response_model=EmbedResponse)
def embed(req: EmbedRequest):
    if len(req.texts) > MAX_BATCH:
        raise HTTPException(400, f"batch too large, max {MAX_BATCH}")

    for t in req.texts:
        if len(t) > 15000:
            raise HTTPException(400, "text too long, max 15000 chars")

    # fastembed.embed() returns a generator of numpy float32 arrays, one per
    # input text, in the same order they were given.
    vectors = [v.tolist() for v in model.embed(req.texts)]
    dim = len(vectors[0]) if vectors else 0
    return EmbedResponse(vectors=vectors, model=MODEL_NAME, dim=dim)
