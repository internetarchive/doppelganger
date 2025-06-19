#!/bin/bash

# Set Pyroscope address and build
export PYROSCOPE_ADDRESS=http://pyroscope.crawl1.archive.org
go build

# Check if build was successful
if [ $? -eq 0 ]; then
    # Find all .gz files in the directory and run doppelganger ingest for each
    # Create a list of all .gz files sorted by name
    find /1/crawling/cdx/deltacdx/ -name "*.gz" -type f | sort > /tmp/file_list.txt
    
    # Check if resume file exists
    if [ -f "/tmp/last_processed.txt" ]; then
        last_processed=$(cat /tmp/last_processed.txt)
        echo "Resuming from: $last_processed"
        # Skip files until we reach the last processed one
        awk -v start="$last_processed" 'found || $0 > start {found=1; print}' /tmp/file_list.txt
    else
        cat /tmp/file_list.txt
    fi | while read -r file; do
        echo "Processing: $file"
        ./doppelganger ingest -c 50 "$file"
        # Save current file as last processed
        echo "$file" > /tmp/last_processed.txt
    done
else
    echo "Build failed"
    exit 1
fi