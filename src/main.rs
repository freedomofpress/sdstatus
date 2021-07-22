use serde::{Deserialize, Serialize};
use serde_json::json;
// use reqwest::Error;
use clap::{crate_version, App, Arg};
use std::error::Error;
//use std::sync::mpsc::channel;
use tokio::sync::mpsc::channel;

use std::collections::BTreeMap;
use std::time::Duration;

use custom_error::custom_error;

#[macro_use]
extern crate log;
use env_logger::Env;

const DIRECTORY_URL: &str = "https://securedrop.org/api/v1/directory/";
const TOR_PROXY: &str = "socks5h://127.0.0.1:9050";
const TOR_TIMEOUT: u64 = 30;

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
}

#[derive(Deserialize, Serialize, Debug)]
struct SDDirectoryInstance {
    metadata: Option<SDMetadata>,
    onion_name: Option<String>,
    title: String,
    landing_page_url: String,
    onion_address: String,
}

// We must implement a custom error type, because `Box<dyn Error>`
// cannot be safely shared via channels.
custom_error! {pub SdStatusError
    NetworkError{source: reqwest::Error} = "Onion not available",
}

impl SDDirectoryInstance {
    pub async fn get_metadata(&mut self) -> Result<(), SdStatusError> {
        let client = reqwest::Client::builder()
            .proxy(reqwest::Proxy::http(TOR_PROXY)?)
            .proxy(reqwest::Proxy::https(TOR_PROXY)?)
            .timeout(Duration::from_secs(TOR_TIMEOUT))
            .build()?;
        debug!("Fetching metadata: {}", self.onion_address);
        let metadata_url = format!("http://{}/metadata", self.onion_address);
        match client.get(&metadata_url).send().await {
            Ok(r) => {
                let m: SDMetadata = r.json().await?;
                self.metadata = Some(m);
                Ok(())
            }
            Err(e) => {
                warn!(
                    "Failed to connect to {} ({})",
                    self.title, self.onion_address
                );
                self.metadata = None;
                Err(SdStatusError::NetworkError { source: e })
            }
        }
    }
}

/// Fetches the securedrop.org API route for info about all SecureDrops.
async fn get_securedrop_directory() -> Result<Vec<SDDirectoryInstance>, Box<dyn Error>> {
    info!("Fetching directory API at {}", DIRECTORY_URL);
    let response = reqwest::get(DIRECTORY_URL).await?;
    let instances: Vec<SDDirectoryInstance> = response.json().await?;
    Ok(instances)
}

/// Scans each SecureDrop Directory instance in order to populate the metadata
/// field. If the instance is down, metadata is None.
async fn populate_metadata(
    instances: Vec<SDDirectoryInstance>,
) -> Result<Vec<SDDirectoryInstance>, Box<dyn Error>> {
    let mut results = vec![];
    let (tx, mut rx) = channel(1024);
    let l = &instances.len();
    for mut i in instances {
        let mut tx = tx.clone();
        tokio::spawn(async move {
            // Errors will be logged, send results to channel regardless.
            match i.get_metadata().await {
                Ok(_) => tx.send(i).await,
                Err(_) => tx.send(i).await,
            }
        });
    }
    let mut counter: usize = 1;
    while let Some(i) = rx.recv().await {
        debug!("Scanned instance {}/{}: {}", counter, l, i.title);
        results.push(i);
        counter += 1;
        // TODO: The while loop on recv blocks forever, why?
        // We'll just break out manually if we've received all results, but
        // this is a hack.
        if &counter == l {
            debug!("Finished reading from channel, reporting results");
            break;
        }
    }
    Ok(results)
}

/// Reads in a file containing JSON results from a previous scan,
/// and inspects the metadata for languages to generate a report.
async fn generate_l10n_report(input_file: &str) -> Result<String, Box<dyn Error>> {
    let j = std::fs::read_to_string(input_file)?;
    let instances: Vec<SDDirectoryInstance> = serde_json::from_str(&j)?;
    let mut locales: BTreeMap<String, Vec<String>> = BTreeMap::new();
    for i in instances {
        // Metadata won't exist for down instances, so check first.
        if let Some(m) = i.metadata {
            for l in m.supported_languages {
                locales
                    .entry(l)
                    .or_insert_with(Vec::new)
                    .push(i.title.to_owned());
            }
        }
    }
    let mut report = String::from("");
    for (locale, sites) in locales {
        report += &format!(
            "{} ({}):\n  {}\n\n",
            &locale,
            &sites.len(),
            sites.join("\n  ")
        );
    }
    Ok(report)
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn Error>> {
    let env = Env::default().filter_or("RUST_LOG", "debug,reqwest=info,hyper=info");
    env_logger::init_from_env(env);

    let matches = App::new("sdstatus")
        .version(crate_version!())
        .about("Reports metadata about SecureDrop sites")
        .subcommand(
            App::new("scan")
                .about("Retrieve metadata from SecureDrop sites")
                .arg(
                    Arg::new("directory")
                        .about("Read sites to scan from the securedrop.org directory")
                        .default_value("true")
                        .takes_value(false)
                        .long("directory")
                        .short('d'),
                )
                .arg(
                    Arg::new("format")
                        .about("Specify output format: 'csv', 'json', or 'pp'")
                        .default_value("json")
                        .long("format")
                        .short('f'),
                ),
        )
        .subcommand(
            App::new("l10n")
                .about("Reports localization metrics from scanned metadata")
                .arg(
                    Arg::new("input_file")
                        .about("The JSON output of a previous 'scan'")
                        .required(true),
                ),
        )
        .get_matches();

    // Primary subcommand
    if let Some(ref matches) = matches.subcommand_matches("scan") {
        // Get all SecureDrops from directory
        let instances = get_securedrop_directory().await?;
        let format = matches.value_of("format").unwrap();
        let full_instances = populate_metadata(instances).await?;
        if format == "json" {
            debug!("Will print results in JSON format");
            let j = json!(full_instances);
            println!("{}", serde_json::to_string_pretty(&j).unwrap());
        } else if format == "pp" {
            for i in full_instances {
                println!("{:?}", i);
            }
        } else {
            error!("Output format {} is unimplemented", format);
        }
    } else if let Some(ref matches) = matches.subcommand_matches("l10n") {
        let input_file = matches.value_of("input_file").unwrap();
        info!(
            "Generating localization report from scan results at: {}",
            input_file
        );
        match generate_l10n_report(input_file).await {
            Ok(r) => println!("{}", r),
            Err(e) => {
                error!("Failed to generated report, {}", e);
            }
        }
    }
    Ok(())
}
