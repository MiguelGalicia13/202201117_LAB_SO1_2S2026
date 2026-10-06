use actix_web::{App, HttpResponse, HttpServer, web};
use serde::{Deserialize, Serialize};
use time::{OffsetDateTime, format_description::well_known::Rfc3339};
use reqwest::Client;
use std::{env, time::Duration};

#[derive(Debug, Deserialize, Serialize)]
struct WarReport {
    country: String,
    warplanes_in_air: i32,
    warships_in_water: i32,
    timestamp: String,
}

impl WarReport {
    //Valida que el reporte recibido es válido
    fn is_valid(&self) -> bool {
        matches!(self.country.as_str(), "USA" | "RUS" | "CHN" | "ESP" | "GTM")
            && self.warplanes_in_air >= 0
            && self.warships_in_water >= 0
            && (self.timestamp.ends_with('Z') || self.timestamp.ends_with("+00:00"))
            && OffsetDateTime::parse(&self.timestamp, &Rfc3339).is_ok()
    }
}

#[derive(Serialize)]
struct ApiResponse {
    status: String,
    message: String,
}


#[actix_web::main]
async fn main() -> std::io::Result<()> {
    HttpServer::new(|| App::new().route("/health", web::get().to(health_check)))
        .bind(("0.0.0.0", 8080))?
        .run()
        .await
}
async fn health_check() -> HttpResponse {
    HttpResponse::Ok().json(
        ApiResponse {
            status: "ok".into(),
            message: "API Rust operational".into(),
        })
}

#[cfg(test)]
mod tests {
    use super::WarReport;
    fn sample() -> WarReport {
        WarReport {
            country: "ESP".into(),
            warplanes_in_air: 0,
            warships_in_water: 0,
            timestamp: "2026-10-06T20:15:30Z".into(),
        }
    }
    #[test]
    fn validate_report() {
        assert!(sample().is_valid());
        let mut report = sample();
        report.country = "INVALID".into();
        assert!(!report.is_valid());

        let mut report = sample();
        report.warplanes_in_air = -1;
        assert!(!report.is_valid());

        let mut report = sample();
        report.timestamp = "ayer".into();
        assert!(!report.is_valid());
    }
}
