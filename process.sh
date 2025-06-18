#!/bin/bash

# Set Pyroscope address and build
export PYROSCOPE_ADDRESS=http://pyroscope.crawl1.archive.org
go build

# Check if build was successful
if [ $? -eq 0 ]; then
    # Find all .gz files in the directory and run doppelganger ingest for each
    find /1/crawling/cdx/deltacdx/ -name "*.gz" -type f | while read -r file; do
        echo "Processing: $file"
        ./doppelganger ingest -c 50 "$file"
    done
else
    echo "Build failed"
    exit 1
fi