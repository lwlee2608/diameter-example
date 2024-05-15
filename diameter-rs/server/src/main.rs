use chrono::Local;
use diameter::avp;
use diameter::avp::flags::M;
use diameter::avp::Avp;
use diameter::avp::Enumerated;
use diameter::avp::Identity;
use diameter::avp::UTF8String;
use diameter::avp::Unsigned32;
use diameter::flags;
use diameter::transport::DiameterServer;
use diameter::transport::DiameterServerConfig;
use diameter::CommandCode;
use diameter::DiameterMessage;
use std::io::Write;
use std::thread;

#[tokio::main]
async fn main() {
    env_logger::Builder::new()
        .format(|buf, record| {
            let now = Local::now();
            let thread = thread::current();
            let thread_name = thread.name().unwrap_or("unnamed");
            let thread_id = thread.id();

            writeln!(
                buf,
                "{} [{}] {:?} - ({}): {}",
                now.format("%Y-%m-%d %H:%M:%S%.3f"),
                record.level(),
                thread_id,
                thread_name,
                record.args()
            )
        })
        .filter(None, log::LevelFilter::Info)
        .init();

    let addr = "0.0.0.0:3868";
    let config = DiameterServerConfig { native_tls: None };
    let mut server = DiameterServer::new(addr, config).await.unwrap();
    log::info!("Diameter server started on {}", addr);

    // Asynchronously handle incoming requests to the server
    server
        .listen(|req| async move {
            log::info!("Received request: {}", req);

            // Create a response message based on the received request
            let mut res = DiameterMessage::new(
                req.get_command_code(),
                req.get_application_id(),
                req.get_flags() ^ flags::REQUEST,
                req.get_hop_by_hop_id(),
                req.get_end_to_end_id(),
            );

            if req.get_command_code() == CommandCode::CapabilitiesExchange {
                let auth_app_id = req.get_avp(258).unwrap().get_unsigned32().unwrap();
                res.add_avp(avp!(264, None, M, Identity::new("host.example.com")));
                res.add_avp(avp!(296, None, M, Identity::new("realm.example.com")));
                res.add_avp(avp!(258, None, M, Unsigned32::new(auth_app_id)));
                res.add_avp(avp!(268, None, M, Unsigned32::new(2001)));
            } else {
                // Add various Attribute-Value Pairs (AVPs) to the response
                res.add_avp(avp!(264, None, M, Identity::new("host.example.com")));
                res.add_avp(avp!(296, None, M, Identity::new("realm.example.com")));
                res.add_avp(avp!(263, None, M, UTF8String::new("ses;123458890")));
                res.add_avp(avp!(416, None, M, Enumerated::new(1)));
                res.add_avp(avp!(415, None, M, Unsigned32::new(1000)));
                res.add_avp(avp!(268, None, M, Unsigned32::new(2001)));
            }

            // Return the response
            log::info!("Replying response: {}", res);
            Ok(res)
        })
        .await
        .unwrap();
}
