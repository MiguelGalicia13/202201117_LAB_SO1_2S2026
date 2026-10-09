import random
from datetime import datetime, timezone

from locust import HttpUser, between, task


class MilitaryReportUser(HttpUser):
    wait_time = between(1, 2)

    @task
    def send_report(self):
        self.client.post(
            "/grpc-202201117",
            json={
                "country": random.choice(("USA", "RUS", "CHN", "ESP", "GTM")),
                "warplanes_in_air": random.randint(0, 50),
                "warships_in_water": random.randint(0, 30),
                "timestamp": datetime.now(timezone.utc).isoformat(timespec="seconds").replace("+00:00", "Z"),
            },
            timeout=10,
        )
