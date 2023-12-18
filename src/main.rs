use anyhow::Result;
use arti_client::{TorClient, TorClientConfig};
use arti_hyper::ArtiHttpConnector;
use clap::{Parser, Subcommand};
use hyper::{Body, Client, Request};
use serde::{Deserialize, Serialize};
use std::collections::{BTreeMap, BTreeSet};
use std::convert::TryInto;
use std::sync::Arc;
use tls_api::{TlsConnector as TlsConnectorTrait, TlsConnectorBuilder};
use tls_api_native_tls::TlsConnector;
use tokio::sync::Semaphore;
use tor_rtcompat::PreferredRuntime;
use tracing::info;
use tracing_subscriber::{prelude::*, EnvFilter};

type HttpClient = Client<ArtiHttpConnector<PreferredRuntime, TlsConnector>, Body>;

const CONCURRENCY: usize = 8;
// aka https://securedrop.org/api/v1/directory/
const DIRECTORY_URL: &str =
    "http://sdolvtfhatvsysc6l34d65ymdwxcujausv7k5jk4cy5ttzhjoi6fzvyd.onion/api/v1/directory/";

// SDMetadata stores the information obtained from a given SecureDrop
// instance's /metadata endpoint, a JSON API with platform info.
#[derive(Deserialize, Serialize, Debug)]
struct SDMetadata {
    sd_version: String,
    server_os: String,
    gpg_fpr: String,
    v2_source_url: Option<String>,
    v3_source_url: String,
    supported_languages: Vec<String>,
    directory: Option<SDDirectoryInstance>,
}

#[derive(Deserialize, Serialize, Debug)]
struct SDDirectoryInstance {
    onion_name: Option<String>,
    title: String,
    landing_page_url: String,
    onion_address: String,
}

async fn fetch_metadata(client: &HttpClient, onion_address: &str) -> Result<SDMetadata> {
    info!("Fetching metadata from {onion_address}...");
    let mut resp = client
        .get(format!("http://{onion_address}/metadata").try_into()?)
        .await?;
    println!("stat = {}", resp.status());
    let body = hyper::body::to_bytes(resp.body_mut()).await?;
    Ok(serde_json::from_slice(&body)?)
}

/// Scans each SecureDrop Directory instance in order to populate the metadata
/// field. If the instance is down, metadata is None.
async fn fetch_all_metadata(
    client: &HttpClient,
    instances: Vec<SDDirectoryInstance>,
) -> Result<Vec<SDMetadata>> {
    let mut threads = vec![];
    let semaphore = Arc::new(Semaphore::new(CONCURRENCY));
    for instance in instances {
        let http = client.clone();
        let lock = semaphore.clone();
        threads.push(tokio::spawn(async move {
            let _permit = lock.acquire().await.unwrap();
            fetch_metadata(&http, &instance.onion_address)
                .await
                .map(|mut val| {
                    // Store the directory entry in the metadata
                    val.directory = Some(instance);
                    val
                })
        }));
    }
    let mut results = vec![];
    for handle in threads {
        let metadata = handle.await??;
        results.push(metadata);
    }
    Ok(results)
}

fn build_l10n_report(metadata: &[SDMetadata]) -> Result<String> {
    let mut locales: BTreeMap<String, BTreeSet<String>> = BTreeMap::new();
    for server in metadata {
        for locale in &server.supported_languages {
            locales
                .entry(locale.to_string())
                .or_default()
                .insert(server.directory.as_ref().unwrap().title.to_string());
        }
    }
    let mut report = vec![];
    for (locale, sites) in locales {
        report.push(format!(
            "{} ({}):\n  {}\n\n",
            &locale,
            &sites.len(),
            sites.into_iter().collect::<Vec<_>>().join("\n  ")
        ));
    }
    Ok(report.join("\n"))
}

async fn fetch_directory(http: &HttpClient) -> Result<Vec<SDDirectoryInstance>> {
    info!("Fetching instances from directory...");
    let req = Request::get(DIRECTORY_URL)
        .header("content-type", "application/json")
        .body(Body::empty())?;
    let mut resp = http.request(req).await?;
    println!("stat = {}", resp.status());
    let body = hyper::body::to_bytes(resp.body_mut()).await?;
    Ok(serde_json::from_slice(&body)?)
}

#[derive(Parser)]
#[command(name = "sdstatus")]
#[command(about = "Reports metadata about SecureDrop sites")]
#[command(version, long_about=None)]
struct Args {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    L10n,
}

async fn build_client() -> Result<HttpClient> {
    let mut config = TorClientConfig::builder();
    config.address_filter().allow_onion_addrs(true);
    info!("Connecting to Tor...");
    let tor_client = TorClient::create_bootstrapped(config.build()?).await?;
    let tls_connector = TlsConnector::builder()?.build()?;
    let tor_connector = ArtiHttpConnector::new(tor_client, tls_connector);
    let http = Client::builder().build(tor_connector);
    Ok(http)
}

async fn main_l10n() -> Result<()> {
    let client = build_client().await?;
    let directory = fetch_directory(&client).await?;
    let metadata = fetch_all_metadata(&client, directory).await?;
    let report = build_l10n_report(&metadata)?;
    println!("{report}");
    Ok(())
}

#[tokio::main]
async fn main() -> Result<()> {
    tracing_subscriber::registry()
        .with(tracing_subscriber::fmt::layer())
        .with(EnvFilter::from_default_env())
        .init();
    let args = Args::parse();
    match args.command {
        Commands::L10n => {
            main_l10n().await?;
        }
    }

    Ok(())
}
