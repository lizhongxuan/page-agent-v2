import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from typing import Any
from urllib.parse import urlparse

from .store import KnowledgeStore


class KnowledgeHttpServer(ThreadingHTTPServer):
    daemon_threads = True

    def __init__(self, server_address, RequestHandlerClass, db_path: str, api_key: str = ""):
        super().__init__(server_address, RequestHandlerClass)
        self.store = KnowledgeStore(db_path)
        self.api_key = api_key

    def server_close(self) -> None:
        self.store.close()
        super().server_close()


class KnowledgeRequestHandler(BaseHTTPRequestHandler):
    server: KnowledgeHttpServer

    def do_OPTIONS(self) -> None:
        self._send_json(204, None)

    def do_GET(self) -> None:
        path = urlparse(self.path).path
        if path == "/health":
            self._send_json(
                200,
                {
                    "ok": True,
                    "documentCount": self.server.store.count_documents(),
                },
            )
            return

        self._send_json(404, {"error": "Not found"})

    def do_POST(self) -> None:
        if not self._is_authorized():
            self._send_json(401, {"error": "Unauthorized"})
            return

        path = urlparse(self.path).path
        try:
            payload = self._read_json()
            if path == "/documents":
                document_id = self.server.store.upsert_document(payload)
                self._send_json(200, {"id": document_id})
                return

            if path == "/ingest":
                documents = payload.get("documents")
                if not isinstance(documents, list):
                    raise ValueError("Expected documents to be a list")
                ids = [self.server.store.upsert_document(document) for document in documents]
                self._send_json(200, {"ids": ids, "count": len(ids)})
                return

            if path == "/search":
                hits = self.server.store.search(payload)
                self._send_json(200, {"hits": hits})
                return

            self._send_json(404, {"error": "Not found"})
        except ValueError as error:
            self._send_json(400, {"error": str(error)})
        except json.JSONDecodeError:
            self._send_json(400, {"error": "Invalid JSON"})

    def log_message(self, format: str, *args: Any) -> None:
        if os.getenv("KB_ACCESS_LOG", "").lower() in {"1", "true", "yes"}:
            super().log_message(format, *args)

    def _is_authorized(self) -> bool:
        expected = self.server.api_key
        if not expected:
            return True

        authorization = self.headers.get("authorization", "")
        api_key = self.headers.get("x-api-key", "")
        return authorization == f"Bearer {expected}" or api_key == expected

    def _read_json(self) -> dict[str, Any]:
        length = int(self.headers.get("content-length", "0"))
        raw = self.rfile.read(length).decode("utf-8") if length else "{}"
        payload = json.loads(raw)
        if not isinstance(payload, dict):
            raise ValueError("JSON body must be an object")
        return payload

    def _send_json(self, status: int, payload: dict[str, Any] | None) -> None:
        body = b"" if payload is None else json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("access-control-allow-origin", "*")
        self.send_header("access-control-allow-methods", "GET, POST, OPTIONS")
        self.send_header("access-control-allow-headers", "content-type, authorization, x-api-key")
        self.send_header("content-type", "application/json; charset=utf-8")
        self.send_header("content-length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)


def create_server(address, db_path: str, api_key: str = "") -> KnowledgeHttpServer:
    return KnowledgeHttpServer(address, KnowledgeRequestHandler, db_path=db_path, api_key=api_key)


def main() -> None:
    host = os.getenv("KB_HOST", "0.0.0.0")
    port = int(os.getenv("KB_PORT", "8787"))
    db_path = os.getenv("KB_DB_PATH", "/data/knowledge.sqlite3")
    api_key = os.getenv("KB_API_KEY", "")

    server = create_server((host, port), db_path=db_path, api_key=api_key)
    print(f"Knowledge base listening on http://{host}:{port}")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
