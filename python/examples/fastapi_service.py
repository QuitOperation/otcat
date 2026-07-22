"""A minimal FastAPI service exposing a Modbus device over HTTP/WebSocket.

    pip install otcat[fastapi]
    otcat-mockplc --addr 127.0.0.1:15020 &
    OTCAT_ENDPOINT=127.0.0.1:15020 uvicorn fastapi_service:app --reload

Then:
    curl localhost:8000/read/holding:100?raw_address=true
    curl -X POST localhost:8000/write -d '{"spec":"holding:100","value":42,"raw_address":true}' -H 'content-type: application/json'
    websocat ws://localhost:8000/watch/holding:0?raw_address=true&interval=500ms

This intentionally does not add its own write-confirmation UI/flow on
top of otcat's -- see otcat.Client's docstring on why the library
defaults to confirm=True. If you're building a service that lets a
remote caller trigger writes, put your own authorization and
confirmation step in *this* layer (the FastAPI app), since the network
boundary that actually needs protecting is "who can call this HTTP
endpoint at all," which is entirely this app's responsibility, not
otcat's.
"""
import os

from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from pydantic import BaseModel

from otcat import ProtocolError
from otcat import ConnectionError as OtcatConnectionError
from otcat.aio import AsyncClient

ENDPOINT = os.environ.get("OTCAT_ENDPOINT", "127.0.0.1:15020")

app = FastAPI(title="otcat FastAPI example")


class WriteRequest(BaseModel):
    spec: str
    value: str
    raw_address: bool = False
    confirm: bool = True


@app.get("/read/{spec:path}")
async def read(spec: str, raw_address: bool = False):
    client = AsyncClient(ENDPOINT, raw_address=raw_address)
    try:
        v = await client.read(spec)
    except ProtocolError as e:
        return {"error": str(e), "exception_code": e.exception_code}
    except OtcatConnectionError as e:
        return {"error": str(e)}
    return {"address": v.address, "value": v.value, "quality": v.quality, "ts": v.ts.isoformat()}


@app.post("/write")
async def write(req: WriteRequest):
    client = AsyncClient(ENDPOINT, raw_address=req.raw_address)
    try:
        await client.write(req.spec, req.value, confirm=req.confirm)
    except Exception as e:
        return {"ok": False, "error": str(e)}
    return {"ok": True}


@app.websocket("/watch/{spec:path}")
async def watch(websocket: WebSocket, spec: str, raw_address: bool = False, interval: str = "1s"):
    await websocket.accept()
    client = AsyncClient(ENDPOINT, raw_address=raw_address)
    try:
        async for v in client.watch(spec, interval=interval):
            await websocket.send_json(
                {"address": v.address, "value": v.value, "quality": v.quality, "ts": v.ts.isoformat()}
            )
    except WebSocketDisconnect:
        pass
