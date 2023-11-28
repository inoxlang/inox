## Planned Features

- Version Control System for Inox projects:
    - Git subset with storage in S3 Buckets and Git-compatible services (Github, Gitlab).
    - Simplified VCS for non-professional developers.
- Teams
    - creation & management of members
    - access control
- Improved Database
    - smart pre-fetching and caching
    - (short term) ability to store hundreds of gigabytes of data
    - (long term)  ability to store terabytes of data 
- Monitoring with persistence in S3.
- Log persistence in S3.
- Limited IaaC (infrastructure as code) capabilities:
    - VM provisioning
    - S3 Bucket creation
    - CDN configuration
- Cluster management using only the **inox** binary (small scale only)
- WebAssembly support using https://github.com/tetratelabs/wazero.
- Internal plugin system or hooks (Inox | WASM)
- Support other init systems in addition to Systemd

## Planned Improvements

- Reduction of memory usage
- Faster runtime
    - (short term) improve the bytecode interpreter
    - (medium term) compile to WASM functions or modules written in a **subset** of Inoxlang
- Security improvements
- Stabilization of the default namespace APIs
- Better code quality
- (short term) 90-95% unit test coverage
- (long term) 99-100% unit test coverage

## Won't Have Or Provide 

- Interactivity with native code (FFIs ...)
- Windows support
- Integration with Docker or Kubernetes
- Integration with Terraform or Pulumi

## Goals

- Zero config (or dead simple)
- Zero boilerplate
- Low maintenance
- Secure by default
- Minimal number of dependencies

___

[README.md](./README.md)