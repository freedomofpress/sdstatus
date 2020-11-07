use serde::Deserialize;
use reqwest::Error;


// SDMetadata stores the information obtained from a given SecureDrop
// instance's /metadata endpoint, a JSON API with platform info.
#[derive(Deserialize, Debug)]
struct SDMetadata {
    version: String,
    platform: String,
    fingerprint: String,
    v2_source_url: String,
    v3_source_url: String,
}

#[derive(Deserialize, Debug)]
struct SDDirectoryInstance {
    // TODO: figure out how to add metadata, maybe impl?
    // metadata: SDMetadata,
    title: String,
    landing_page_url: String,
    onion_address: String,
}


async fn get_metadata(onion_url: String) -> Result<SDMetadata, Error> {
    let metadata_url = format!("{}/metadata", onion_url);
    let response = reqwest::get(&metadata_url).await?;
    let metadata: SDMetadata = response.json().await?;
    return metadata;
}

#[derive(Debug)]
impl SDDirectoryInstance {
    fn metadata(&self) -> SDMetadata {
        let m = get_metadata(self.onion_address).await?;
        return m;
    }
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    let request_url = "https://securedrop.org/api/v1/directory/";
    println!("Fetching directory API at {}", request_url);
    let response = reqwest::get(request_url).await?;

    let metadata: Vec<SDDirectoryInstance> = response.json().await?;
    for m in metadata {
        println!("{:?}", m);
    }
    Ok(())
}
