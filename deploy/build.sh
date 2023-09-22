#!/usr/bin/env bash
set -e
set -o nounset

environment=$1
arch_type=$2
version=$3

bucket_name=$ED_DEV_SERVERLESS_REPOSITORY_BUCKET
# if environment is prod then use prod bucket
if [ "$environment" == "prod" ]; then
    bucket_name=$ED_SERVERLESS_REPOSITORY_BUCKET
fi

if [ -z "$arch_type" ]; then
    echo "Arch type is empty"
    exit 1
fi

if [ -z "$version" ]; then
    echo "Version is empty"
    exit 1
fi

project_root=$(git rev-parse --show-toplevel)
# Executable’s name must be “bootstrap”
name="bootstrap"
exe_path="bin/$name"
zip_name="forwarder_${arch_type}_${version}.zip"

rm -rf "${project_root}/bin"
mkdir -p "${project_root}/bin"

cd "${project_root}"
GOOS=linux GOARCH=$arch_type go build -tags lambda.norpc -o "$exe_path" main.go
chmod +x "$exe_path"

cd "${project_root}/bin"
zip -r "$zip_name" "$name"

echo "Uploading $zip_name to s3://$bucket_name/$zip_name"
aws s3 cp "$zip_name" "s3://$bucket_name/$zip_name"
