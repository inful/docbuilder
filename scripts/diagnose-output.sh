#!/bin/bash
# Diagnose output directory state to identify serving issues

echo "=== Diagnosing DocBuilder Output Directories ==="
echo

# Read config to get output directory
if [ -f "config.yaml" ]; then
    BASE_DIR=$(grep "base_directory:" config.yaml | awk '{print $2}' | tr -d '"')
    OUTPUT_DIR=$(grep "directory:" config.yaml | grep -v "base_directory" | awk '{print $2}' | tr -d '"')
    
    if [ -n "$BASE_DIR" ]; then
        FULL_OUTPUT="${BASE_DIR}/${OUTPUT_DIR}"
    else
        FULL_OUTPUT="${OUTPUT_DIR:-./site}"
    fi
    
    echo "Config: base_directory=$BASE_DIR, directory=$OUTPUT_DIR"
    echo "Full output path: $FULL_OUTPUT"
    echo
fi

# Check all potential output locations
for dir in site /data/site ./site; do
    if [ -d "$dir" ]; then
        echo "=== Checking $dir ==="
        ls -lah "$dir" 2>/dev/null | head -20
        echo
        
        # Check for backup/staging directories
        if [ -d "${dir}.prev" ]; then
            echo "⚠️  Found backup directory: ${dir}.prev"
            ls -lah "${dir}.prev/public" 2>/dev/null | head -5
            echo
        fi
        
        if [ -d "${dir}_stage" ]; then
            echo "⚠️  Found staging directory: ${dir}_stage"
            ls -lah "${dir}_stage" 2>/dev/null | head -5
            echo
        fi
        
        # Check public directory
        if [ -d "${dir}/public" ]; then
            echo "✓ Public directory exists: ${dir}/public"
            echo "Recent files in public:"
            find "${dir}/public" -type f -mtime -1 -ls 2>/dev/null | head -10
            echo
        else
            echo "✗ No public directory at ${dir}/public"
            echo
        fi
        
        # Check build report
        if [ -f "${dir}/build-report.json" ]; then
            echo "Build report:"
            cat "${dir}/build-report.json" | head -20
            echo
        fi
    fi
done

# Check for staging in base_directory
if [ -n "$BASE_DIR" ] && [ -d "$BASE_DIR" ]; then
    echo "=== Checking base_directory: $BASE_DIR ==="
    ls -lah "$BASE_DIR" 2>/dev/null
    echo
    
    if [ -d "${BASE_DIR}/staging" ]; then
        echo "⚠️  Found staging directory in base: ${BASE_DIR}/staging"
        ls -lah "${BASE_DIR}/staging" 2>/dev/null | head -10
        echo
    fi
fi

echo "=== Recommendation ==="
echo "If you see .prev or _stage directories, these may contain old content."
echo "The HTTP server will serve from .prev if public doesn't exist."
echo
echo "To force a clean rebuild:"
echo "  rm -rf ${FULL_OUTPUT:-site}.prev ${FULL_OUTPUT:-site}_stage"
echo "  systemctl restart docbuilder  # or however you run the daemon"
