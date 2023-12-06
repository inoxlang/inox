## Planned Features

- Encryption of secrets and database data
- Storage of secrets in key management services (e.g. GCP KMS, AWS KMS)
- Version Control System for Inox projects:
    - Git subset using https://github.com/go-git/go-git with storage in S3 Buckets and Git-compatible services (Github, Gitlab).
    - Simplified VCS for non-professional developers.
- Teams
    - creation & management of members
    - access control
- Improved Database Engine
    - smart pre-fetching and caching
- Improved Local Database
    - (short term) ability to store hundreds of gigabytes of data
    - (long term)  ability to store terabytes of data
- Database with persistence in S3 and on-disk cache
- Monitoring with persistence in S3.
- Log persistence in S3.
- Limited IaaC (infrastructure as code) capabilities:
    - VM provisioning
    - S3 Bucket creation
    - CDN configuration
- Cluster management using only the **inox** binary (small scale only)
- WebAssembly support using https://github.com/tetratelabs/wazero.
- Execution of modules when certain events occur (e.g. new user in database)
- Progressive web app support
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

- Zero boilerplate
- Secure by default
- Low maintenance
- Dead simple configuration
___

[README.md](./README.md)