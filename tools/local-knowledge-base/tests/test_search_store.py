import tempfile
import unittest

from kb_service.store import KnowledgeStore


class KnowledgeStoreTest(unittest.TestCase):
    def test_search_prioritizes_matching_business_article(self):
        store = KnowledgeStore(":memory:")
        store.upsert_document(
            {
                "id": "pg-log-runbook",
                "title": "PG instance runtime log runbook",
                "source": "runbook",
                "content": (
                    "When checking runtime logs, compare visible timestamps first. "
                    "If log timestamps are ascending, the newest entries are at the bottom. "
                    "Use bounded scrolling to avoid following live output forever."
                ),
                "projectKey": "aiops",
                "tags": ["postgresql", "runtime-log"],
            }
        )
        store.upsert_document(
            {
                "id": "unrelated",
                "title": "User management",
                "source": "manual",
                "content": "Create users from the account page.",
                "projectKey": "aiops",
            }
        )

        hits = store.search(
            {
                "task": "排查 PG 实例运行日志最新错误",
                "title": "172.25.1.74 运行日志",
                "url": "https://platform.local/console/#/service/kme",
                "projectKey": "aiops",
                "limit": 3,
            }
        )

        self.assertGreater(len(hits), 0)
        self.assertEqual(hits[0]["id"], "pg-log-runbook")
        self.assertIn("newest entries are at the bottom", hits[0]["snippet"])
        self.assertGreater(hits[0]["score"], 0)

    def test_search_filters_by_project_key_but_keeps_global_documents(self):
        store = KnowledgeStore(":memory:")
        store.upsert_document(
            {
                "id": "aiops-doc",
                "title": "KME troubleshooting",
                "source": "manual",
                "content": "KME service details and PG runtime log checks.",
                "projectKey": "aiops",
            }
        )
        store.upsert_document(
            {
                "id": "global-doc",
                "title": "Shared log convention",
                "source": "manual",
                "content": "Runtime logs may need bounded scrolling.",
            }
        )
        store.upsert_document(
            {
                "id": "other-project-doc",
                "title": "KME troubleshooting",
                "source": "manual",
                "content": "This belongs to another project.",
                "projectKey": "other",
            }
        )

        hits = store.search(
            {
                "task": "KME runtime log",
                "url": "https://platform.local",
                "title": "KME",
                "projectKey": "aiops",
                "limit": 10,
            }
        )

        ids = {hit["id"] for hit in hits}
        self.assertIn("aiops-doc", ids)
        self.assertIn("global-doc", ids)
        self.assertNotIn("other-project-doc", ids)

    def test_persists_documents_on_disk(self):
        with tempfile.NamedTemporaryFile() as file:
            first = KnowledgeStore(file.name)
            first.upsert_document(
                {
                    "id": "persisted",
                    "title": "Persistent document",
                    "source": "manual",
                    "content": "The database survives service restarts.",
                }
            )
            first.close()

            second = KnowledgeStore(file.name)
            hits = second.search(
                {
                    "task": "database service restarts",
                    "url": "https://platform.local",
                    "title": "Restart",
                    "limit": 5,
                }
            )
            second.close()

        self.assertEqual([hit["id"] for hit in hits], ["persisted"])


if __name__ == "__main__":
    unittest.main()
