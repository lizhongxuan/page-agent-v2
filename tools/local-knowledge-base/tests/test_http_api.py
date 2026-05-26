import http.client
import json
import tempfile
import threading
import unittest

from kb_service.server import create_server


class KnowledgeHttpApiTest(unittest.TestCase):
    def test_documents_can_be_added_and_searched_over_http(self):
        with tempfile.NamedTemporaryFile() as file:
            server = create_server(("127.0.0.1", 0), db_path=file.name, api_key="secret")
            thread = threading.Thread(
                target=lambda: server.serve_forever(poll_interval=0.05),
                daemon=True,
            )
            thread.start()

            try:
                port = server.server_address[1]
                create_response = self._request(
                    port,
                    "POST",
                    "/documents",
                    {
                        "id": "runtime-log",
                        "title": "Runtime log convention",
                        "source": "manual",
                        "content": "Ascending timestamps mean the newest log entries are at the bottom.",
                        "projectKey": "aiops",
                        "tags": ["logs"],
                    },
                    token="secret",
                )
                self.assertEqual(create_response["id"], "runtime-log")

                search_response = self._request(
                    port,
                    "POST",
                    "/search",
                    {
                        "task": "where are the newest runtime log entries",
                        "url": "https://platform.local",
                        "title": "Runtime log",
                        "projectKey": "aiops",
                        "limit": 2,
                    },
                    token="secret",
                )
            finally:
                server.shutdown()
                server.server_close()

        self.assertEqual(search_response["hits"][0]["id"], "runtime-log")
        self.assertEqual(search_response["hits"][0]["source"], "manual")
        self.assertIn("newest log entries", search_response["hits"][0]["snippet"])

    def test_rejects_requests_with_wrong_api_key_when_configured(self):
        with tempfile.NamedTemporaryFile() as file:
            server = create_server(("127.0.0.1", 0), db_path=file.name, api_key="secret")
            thread = threading.Thread(
                target=lambda: server.serve_forever(poll_interval=0.05),
                daemon=True,
            )
            thread.start()

            try:
                port = server.server_address[1]
                connection = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
                connection.request(
                    "POST",
                    "/search",
                    body=json.dumps({"task": "logs", "url": "", "title": "", "limit": 1}),
                    headers={"content-type": "application/json", "authorization": "Bearer wrong"},
                )
                response = connection.getresponse()
                response.read()
                connection.close()
            finally:
                server.shutdown()
                server.server_close()

        self.assertEqual(response.status, 401)

    def _request(self, port, method, path, payload, token=None):
        headers = {"content-type": "application/json"}
        if token:
            headers["authorization"] = f"Bearer {token}"

        connection = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
        connection.request(method, path, body=json.dumps(payload), headers=headers)
        response = connection.getresponse()
        data = response.read().decode("utf-8")
        connection.close()

        self.assertGreaterEqual(response.status, 200)
        self.assertLess(response.status, 300)
        return json.loads(data)


if __name__ == "__main__":
    unittest.main()
