use actix_web::{App, HttpResponse, HttpServer,web};
use serde_json;
use serde::{Deserialize, Serialize};
use time::{format_description::well_known::Rfc3339, OffsetDateTime};

#[derive(Debug, Deserialize, Serialize)]
struct WarReport{
    country: String,
    warplanes_in_air: i32,
    warships_in_water: i32,
    time_stamp: String,
}

impl WarReport {
    //Valida que el reporte recibido es válido
    fn is_valid(&self)->bool{
        matches!(self.country.as_str(), "USA"|"RUS"|"CHN"|"ESP"|"GTM") &&
        self.warplanes_in_air >= 0 &&
        self.warships_in_water >= 0 &&
        (self.time_stamp.ends_with('Z')||self.time_stamp.ends_with("+00:00")) &&
        OffsetDateTime::parse(&self.time_stamp, &Rfc3339).is_ok()
    }
}

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    HttpServer::new(|| App::new()
        .route(
            "/HEALTH", web::get().to(health_check)
    ))
        .bind(("127.0.0.1", 8080))?
        .run()
        .await
}
async fn health_check() -> HttpResponse {
    HttpResponse::Ok().json(serde_json::json!({
        "status": "ok",
        "message": "API Rust operational"
    }))
}