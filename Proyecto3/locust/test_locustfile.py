import unittest
from datetime import datetime, timezone
from types import SimpleNamespace
from unittest.mock import Mock

from locustfile import MilitaryReportUser


class MilitaryReportUserTest(unittest.TestCase):
    def test_sends_reports_with_the_required_contract(self):
        client = Mock()
        user = SimpleNamespace(client=client)
        started = datetime.now(timezone.utc).replace(microsecond=0)

        for _ in range(100):
            MilitaryReportUser.send_report(user)
            args, kwargs = client.post.call_args
            self.assertEqual(args, ("/grpc-202201117",))
            self.assertEqual(kwargs["timeout"], 10)
            report = kwargs["json"]
            self.assertEqual(set(report), {
                "country", "warplanes_in_air", "warships_in_water", "timestamp",
            })
            self.assertIn(report["country"], ("USA", "RUS", "CHN", "ESP", "GTM"))
            for field, maximum in (("warplanes_in_air", 50), ("warships_in_water", 30)):
                self.assertIs(type(report[field]), int)
                self.assertGreaterEqual(report[field], 0)
                self.assertLessEqual(report[field], maximum)
            self.assertTrue(report["timestamp"].endswith("Z"))
            timestamp = datetime.fromisoformat(report["timestamp"])
            self.assertEqual(timestamp.utcoffset().total_seconds(), 0)
            self.assertGreaterEqual(timestamp, started)
            self.assertLessEqual(timestamp, datetime.now(timezone.utc))


if __name__ == "__main__":
    unittest.main()
