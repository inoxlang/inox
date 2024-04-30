#! /bin/bash -eux

if [ -z "${BINARY_ASSET_NAME}" ]; then
  echo "BINARY_ASSET_NAME is empty"
  exit 1
fi

if [ -z "${CHECKSUM_ASSET_NAME}" ]; then
  echo "CHECKSUM_ASSET_NAME is empty"
  exit 1
fi

if [ -z "${GIT_TAG}" ]; then
  echo "GIT_TAG is empty"
  exit 1
fi

# Compute checksum
SHA256_SUM=$(sha256sum ${BINARY_ASSET_NAME} | cut -d ' ' -f 1)
echo ${SHA256_SUM} >${CHECKSUM_ASSET_NAME}

# Create archive
tar cvfz ${BINARY_ASSET_NAME} ./inox

# Upload archive and checksum file
assets_uploader=github.com/wangyoucao577/assets-uploader/cmd/github-assets-uploader@v0.13.0

go run $assets_uploader -logtostderr \
    -f ${BINARY_ASSET_NAME} \
    -mediatype 'application/gzip' \
    -token ${GITHUB_TOKEN} \
    -repo ${GITHUB_REPOSITORY} \
    -tag=${GIT_TAG} \
    -retry 3

go run $assets_uploader -logtostderr \
    -f ${CHECKSUM_ASSET_NAME} \
    -mediatype 'text/plain' \
    -token ${GITHUB_TOKEN} \
    -repo ${GITHUB_REPOSITORY} \
    -tag=${GIT_TAG} \
    -retry 3