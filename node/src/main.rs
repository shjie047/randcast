// use std::time;
use dkg::dkg::v1::dkg_service_client::DkgServiceClient;
use dkg::dkg::v1::StartDkgRequest;

/*
struct Node {
    addr: String,
}

impl Node {
    fn new() -> Self {
        Node {
            addr: ":7070".to_owned(),
        }
    }
}
*/

async fn start_dkg(req: StartDkgRequest, port: String) -> Result<(), Box<dyn std::error::Error>> {
    let mut client = DkgServiceClient::connect(format!("http://[::]:{}", port)).await?;
    let request = tonic::Request::new(req);
    let response = client.start_dkg(request).await?;

    println!("RESPONSE = {:?}", response.get_ref().state());

    Ok(())
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let ps = [1u32, 2, 3, 4];
    let id = std::env::args().nth(1).unwrap().parse::<u32>().unwrap();
    let port = format!("808{}", id);

    start_dkg(StartDkgRequest {
        threshold: 3,
        id,
        others: ps.into_iter().filter(|&x| x != id).collect(),
        msg: "hi".to_string(),
    },port).await?;

    // let ps = [1u32, 2, 3, 4];
    // for id in ps.into_iter() {
    //     std::thread::sleep(time::Duration::from_secs(1));
    //     start_dkg(StartDkgRequest {
    //         threshold: 3,
    //         id,
    //         others: ps.into_iter().filter(|&x| x != id).collect(),
    //         msg: "hi".to_string(),
    //     })
    //     .await?;
    // }

    Ok(())
}
