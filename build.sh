#!/usr/bin/env bash

# cd client
# npm run build
# cd ..
# rm -rf ./server-go/html/*
# cp -r client/dist/* server-go/html/

cd server-go
platforms=("windows/amd64" "linux/amd64" "darwin/amd64")

for platform in "${platforms[@]}"
do
	platform_split=(${platform//\// })
	GOOS=${platform_split[0]}
	GOARCH=${platform_split[1]}
	output_name='../release/'$GOOS'-'$GOARCH'/oraprofiler'
	if [ $GOOS = "windows" ]; then
		output_name+='.exe'
	fi	

	env GOOS=$GOOS GOARCH=$GOARCH GO111MODULE=on go build -o $output_name . -ldflags "-s -w"
	if [ $? -ne 0 ]; then
   		echo 'An error has occurred! Aborting the script execution...'
		exit 1
	fi
done