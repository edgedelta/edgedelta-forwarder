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


ARCH_TYPE="AMD64"
if [ "$arch_type" == "arm64" ]; then
    ARCH_TYPE="ARM64"
fi
file_name="forwarder_${arch_type}_${version}.zip"

compatible_arch="x86_64"
if [ "$arch_type" == "arm64" ]; then
    compatible_arch="arm64"
fi

cat template.yaml.tmpl \
| sed "s|{ARCH_TYPE}|$ARCH_TYPE|g" \
| sed "s|{BUCKET}|$bucket_name|g" \
| sed "s|{VERSION}|$version|g" \
| sed "s|{FILE_NAME}|$file_name|g" \
| sed "s|{ARCH_TYPE_LOWER}|$arch_type|g" \
| sed "s|{COMPATIBLE_ARCHITECTURE}|$compatible_arch|g" \
> template.yml

echo "Packaging SAM template"
sam package --output-template-file packaged.yaml --s3-bucket $bucket_name --template-file template.yml

echo "Publishing SAM template"
sam publish --template packaged.yaml --region $AWS_DEFAULT_REGION