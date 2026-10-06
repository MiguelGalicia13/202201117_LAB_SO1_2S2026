use actix_web::{App, HttpResponse, HttpServer, web};
use reqwest::Client;
use serde::{Deserialize, Serialize};
use std::{env, time::Duration};
use time::{OffsetDateTime, format_description::well_known::Rfc3339};

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

// Web server
#[actix_web::main]
async fn main() -> std::io::Result<()> {
    let go_service_url = env::var("GO_SERVICE_URL").expect("GO_SERVICE_URL not set");

    let client = web::Data::new(
        Client::builder()
            .timeout(Duration::from_secs(5))
            .build()
            .unwrap(),
    );
    let go_service_url = web::Data::new(go_service_url);

    HttpServer::new(move || {
        App::new()
            .app_data(client.clone())
            .app_data(go_service_url.clone())
            .route("/health", web::get().to(health_check))
            .route("/grpc-202201117", web::post().to(ingres_report))
    })
    .bind(("0.0.0.0", 8080))?
    .run()
    .await
}

// ================================================Endpoints==========================================
async fn health_check() -> HttpResponse {
    HttpResponse::Ok().json(ApiResponse {
        status: "ok".into(),
        message: "API Rust operational".into(),
    })
}

async fn ingres_report(
    client: web::Data<Client>,
    go_service_url: web::Data<String>,
    payload: web::Json<WarReport>,
) -> HttpResponse {
    let report = payload.into_inner();
    if !report.is_valid() {
        return HttpResponse::BadRequest().json(ApiResponse {
            status: "error".into(),
            message: "Invalid report".into(),
        });
    }
    match client
        .post(go_service_url.get_ref().as_str())
        .json(&report)
        .send()
        .await
    {
        Ok(response) if response.status().is_success() => HttpResponse::Ok().json(ApiResponse {
            status: "success".into(),
            message: "Report processed and forwarded to Go".into(),
        }),
        Ok(response) => {
            eprintln!("Go service returned status: {}", response.status());
            HttpResponse::BadGateway().json(ApiResponse {
                status: "error".into(),
                message: "Go service returned error".into(),
            })
        }
        Err(e) => HttpResponse::ServiceUnavailable().json(ApiResponse {
            status: "error".into(),
            message: format!("Failed to ingest report: {}",e.to_string()),
        }),
    }
}

// Tests
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
