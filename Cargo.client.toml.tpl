[package]
name = "dkg"
version = "0.1.0"
edition = "2021"

[features]
dkg_v1 = []

[dependencies]
prost = "0.10.3"
prost-types = "0.10.1"
tonic = { version = "0.7.2", features = ["compression"] }
tokio = { version = "1.18.2", features = ["full"] }