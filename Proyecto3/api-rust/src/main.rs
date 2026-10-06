use actix_web::{App, HttpResponse, HttpServer,web};
use serde_json;

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